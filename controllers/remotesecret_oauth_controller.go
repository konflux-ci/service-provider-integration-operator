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
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const OAuthUrlAnnotation = "appstudio.redhat.com/sp.oauthurl"

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
		Named("remoteSecretOauth").
		Watches(
			&source.Kind{Type: &v1beta1.RemoteSecret{}},
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

	repoHost, labelPresent := remoteSecret.Labels[api.RSServiceProviderHostLabel]
	if !labelPresent {
		if _, annoPresent := remoteSecret.Annotations[OAuthUrlAnnotation]; !annoPresent {
			lg.V(logs.DebugLevel).Info("RemoteSecret does not have host label or oauth url annotation. nothing left to be done")
			return ctrl.Result{}, nil
		}
		lg.V(logs.DebugLevel).Info("RemoteSecret does not have host label but has oauth url annotation. removing the annotation")
		delete(remoteSecret.Annotations, OAuthUrlAnnotation)
		return r.retryableUpdate(ctx, remoteSecret, "failed to update RemoteSecret after removing oauth url annotation")
	}

	// We assume https schema for every service provider.
	serviceProviderUrl := "https://" + repoHost
	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, serviceProviderUrl, remoteSecret.Namespace)
	if err != nil {
		// Currently we do not update the RemoteSecret with any error. One possibility of doing so would be through
		// condition in status.
		var validationErr validator.ValidationErrors
		if stderrors.As(err, &validationErr) {
			// Reconciling here would not resolve the problem because either repoHost is malformed and must be changed
			// or service provider configuration has to be changed to fix the error. So just log it without reconciling.
			lg.Error(validationErr, "failed to validate the service provider", "serviceProviderUrl", serviceProviderUrl)
			return ctrl.Result{}, nil
		}
		// In this case reconciling again might fix the problem.
		return ctrl.Result{}, fmt.Errorf("failed to determine the service provider from URL %s: %w", serviceProviderUrl, err)
	}
	oauthCapability := sp.GetOAuthCapability()
	if oauthCapability == nil {
		lg.Info("service provider does not support oauth capability. nothing left to be done", "serviceprovider", sp.GetType())
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
		return ctrl.Result{}, fmt.Errorf("failed to encode the OAuth state: %w", err)
	}

	if remoteSecret.Annotations == nil {
		remoteSecret.Annotations = make(map[string]string)
	}
	remoteSecret.Annotations[OAuthUrlAnnotation] = oauthCapability.GetOAuthEndpoint() + "?state=" + state

	return r.retryableUpdate(ctx, remoteSecret, "failed to update RemoteSecret with OAuth URL")
}

func (r *RemoteSecretOAuthReconciler) retryableUpdate(ctx context.Context, remoteSecret *v1beta1.RemoteSecret, errMsg string) (reconcile.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Update(ctx, remoteSecret); err != nil {
			return fmt.Errorf("%s: %w", errMsg, err)
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update RemoteSecret after multiple retries: %w", err)
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
