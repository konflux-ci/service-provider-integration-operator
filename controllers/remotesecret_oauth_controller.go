//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/go-playground/validator/v10"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OAuthConditionReason string

const (
	OAuthUrlAnnotation = "appstudio.redhat.com/sp.oauthurl"

	OAuthConditionType = "OAuthURLGenerated"

	OAuthConditionReasonNotSupported OAuthConditionReason = "NotSupported"
	OAuthConditionReasonSuccess      OAuthConditionReason = "Success"
	OAuthConditionReasonError        OAuthConditionReason = "Error"
)

type RemoteSecretOAuthReconciler struct {
	client.Client
	ServiceProviderFactory serviceprovider.Factory
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/status,verbs=get;update;patch

var _ reconcile.Reconciler = (*RemoteSecretOAuthReconciler)(nil)

func (r *RemoteSecretOAuthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// for newly created objects we are interested in those that have the host label and in update we check
	// if host label or oauth url annotation changed
	pred := predicate.Funcs{
		CreateFunc: func(de event.CreateEvent) bool {
			return de.Object.GetLabels()[api.RSServiceProviderHostLabel] != ""
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			if ue.ObjectOld.GetLabels()[api.RSServiceProviderHostLabel] != ue.ObjectNew.GetLabels()[api.RSServiceProviderHostLabel] {
				return true
			}
			return ue.ObjectOld.GetAnnotations()[OAuthUrlAnnotation] != ue.ObjectNew.GetAnnotations()[OAuthUrlAnnotation]
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}

	err := ctrl.NewControllerManagedBy(mgr).
		Named("remotesecretoauth").
		Watches(
			&v1beta1.RemoteSecret{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(pred)).Complete(r)
	if err != nil {
		return fmt.Errorf("failed to configure the reconciler: %w", err)
	}
	return nil
}

func (r *RemoteSecretOAuthReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile RemoteSecret")

	remoteSecret := &v1beta1.RemoteSecret{}

	if err := r.Get(ctx, req.NamespacedName, remoteSecret); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("RemoteSecret already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the RemoteSecret: %w", err)
	}

	if remoteSecret.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("RemoteSecret is being deleted. skipping reconciliation")
		return ctrl.Result{}, nil
	}
	patch := client.MergeFrom(remoteSecret.DeepCopy())

	repoHost, labelPresent := remoteSecret.Labels[api.RSServiceProviderHostLabel]
	if !labelPresent {
		if _, annoPresent := remoteSecret.Annotations[OAuthUrlAnnotation]; annoPresent {
			lg.V(logs.DebugLevel).Info("RemoteSecret doesn't have host label but has oauth condition left. cleaning it up")
			delete(remoteSecret.Annotations, OAuthUrlAnnotation)
			if err := r.Patch(ctx, remoteSecret, patch); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove oauth annotation by patching remoteSecret: %w", err)
			}
		}
		if meta.FindStatusCondition(remoteSecret.Status.Conditions, OAuthConditionType) != nil {
			lg.V(logs.DebugLevel).Info("RemoteSecret doesn't have host label but has oauth condition left. cleaning up")
			meta.RemoveStatusCondition(&remoteSecret.Status.Conditions, OAuthConditionType)
			if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove oauth condition by patching remoteSecret: %w", err)
			}
		}
		lg.Info("RemoteSecret doesn't have host label. nothing left to be done")
		return ctrl.Result{}, nil
	}

	// We assume https schema for every service provider.
	serviceProviderUrl := "https://" + repoHost
	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, serviceProviderUrl, remoteSecret.Namespace)
	if err != nil {
		var validationErr validator.ValidationErrors
		if stderrors.As(err, &validationErr) {
			// Reconciling here would not resolve the problem because either repoHost is malformed and must be changed
			// or service provider configuration has to be changed to fix the error.
			// So just log it and update condition without reconciling (note that reconcile will happen IF patching errors).
			lg.Error(validationErr, "failed to validate the service provider", "serviceProviderUrl", serviceProviderUrl)
			meta.SetStatusCondition(&remoteSecret.Status.Conditions, conditionFromError(validationErr))
			if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to add oauth error condition by patching remoteSecret: %w", err)
			}
			return ctrl.Result{}, nil
		}

		// In this case, reconciling again might fix the problem.
		meta.SetStatusCondition(&remoteSecret.Status.Conditions, conditionFromError(err))
		if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add oauth error condition by patching remoteSecret: %w", err)
		}
		return ctrl.Result{}, fmt.Errorf("failed to determine the service provider from URL %s: %w", serviceProviderUrl, err)
	}
	oauthCapability := sp.GetOAuthCapability()
	if oauthCapability == nil {
		condition := metav1.Condition{
			Type:    OAuthConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  string(OAuthConditionReasonNotSupported),
			Message: fmt.Sprintf("Service provider with host '%s' does not support OAuth flow.", repoHost),
		}

		lg.Info("service provider does not support oauth capability. nothing left to be done",
			"serviceprovider", sp.GetType(),
			"host", repoHost)
		meta.SetStatusCondition(&remoteSecret.Status.Conditions, condition)
		if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add oauth not supported condition by patching remoteSecret: %w", err)
		}
		return ctrl.Result{}, nil
	}

	permissions := getFixedPermissionsForProvider(sp.GetType())
	state, err := oauthstate.Encode(&oauthstate.OAuthInfo{
		ObjectName:          remoteSecret.Name,
		ObjectNamespace:     remoteSecret.Namespace,
		ObjectKind:          remoteSecret.Kind,
		Scopes:              oauthCapability.OAuthScopesFor(&permissions),
		ServiceProviderName: sp.GetType().Name,
		ServiceProviderUrl:  sp.GetBaseUrl(),
	})
	if err != nil {
		meta.SetStatusCondition(&remoteSecret.Status.Conditions, conditionFromError(err))
		if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add oauth error condition by patching remoteSecret: %w", err)
		}
		return ctrl.Result{}, fmt.Errorf("failed to encode the OAuth state, this might indicate that some part of OAuth state is malformed: %w", err)
	}

	if remoteSecret.Annotations == nil {
		remoteSecret.Annotations = make(map[string]string)
	}
	remoteSecret.Annotations[OAuthUrlAnnotation] = oauthCapability.GetOAuthEndpoint() + "?state=" + state
	if err := r.Patch(ctx, remoteSecret, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to add oauth annotation by patching remoteSecret: %w", err)
	}

	meta.SetStatusCondition(&remoteSecret.Status.Conditions, metav1.Condition{
		Type:    OAuthConditionType,
		Status:  metav1.ConditionTrue,
		Reason:  string(OAuthConditionReasonSuccess),
		Message: "OAuth URL successfully generated.",
	})
	if err := r.Status().Patch(ctx, remoteSecret, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to add oauth success condition by patching remoteSecret: %w", err)
	}
	return ctrl.Result{}, nil
}

// getFixedPermissionsForProvider returns fixed Permissions for a ServiceProvider. These permissions can be then
// used to obtain a fixed set of scopes to request in OAuth flow. In the future this function can/should be replaced
// with a functionality that will enable users to pick whatever scopes they require.
func getFixedPermissionsForProvider(spType config.ServiceProviderType) api.Permissions {
	switch spType.Name {
	case config.ServiceProviderTypeQuay.Name:
		return api.Permissions{
			Required: []api.Permission{
				{
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaRegistry,
				},
				{
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaUser,
				}},
		}
	default:
		return api.Permissions{
			Required: []api.Permission{
				{
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaRepository,
				}, {
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaUser,
				}},
		}
	}
}

func conditionFromError(err error) metav1.Condition {
	return metav1.Condition{
		Type:    OAuthConditionType,
		Status:  metav1.ConditionFalse,
		Reason:  string(OAuthConditionReasonError),
		Message: err.Error(),
	}
}
