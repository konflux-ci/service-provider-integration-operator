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
	"fmt"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const oauthUrlLabel = "appstudio.redhat.com/sp.oauthurl"

type RemoteSecretOAuthReconciler struct {
	client.Client
	ServiceProviderFactory serviceprovider.Factory
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/status,verbs=get;update;patch

var _ reconcile.Reconciler = (*RemoteSecretOAuthReconciler)(nil)

var oauthRemoteSecretSelector = metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{
		{
			Key:      api.RSServiceProviderHostLabel,
			Values:   []string{""},
			Operator: metav1.LabelSelectorOpExists,
		},
	},
}

func (r *RemoteSecretOAuthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred, err := predicate.LabelSelectorPredicate(oauthRemoteSecretSelector)
	if err != nil {
		return fmt.Errorf("failed to construct the predicate for matching RemoteSecret. This should not happen: %w", err)
	}
	err = ctrl.NewControllerManagedBy(mgr).
		Watches(
			&source.Kind{Type: &v1beta1.RemoteSecret{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(pred, predicate.Funcs{
				CreateFunc:  func(de event.CreateEvent) bool { return true },
				UpdateFunc:  func(de event.UpdateEvent) bool { return true },
				DeleteFunc:  func(de event.DeleteEvent) bool { return false },
				GenericFunc: func(de event.GenericEvent) bool { return false },
			}),
		).
		Complete(r)
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
		lg.V(logs.DebugLevel).Info("RemoteSecret does not have host label. nothing left to be done")
		return ctrl.Result{}, nil
	}

	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, "https://"+repoHost, remoteSecret.Namespace)
	if err != nil {
		// TODO: how to handle errors
		//var validationErr validator.ValidationErrors
		//if stderrors.As(err, &validationErr) {
		//	return "", fmt.Errorf("failed to validate the service provider for URL %s: %w", at.Spec.ServiceProviderUrl, validationErr)
		//} else {
		//	return "", fmt.Errorf("failed to determine the service provider from URL %s: %w", at.Spec.ServiceProviderUrl, err)
		//}
	}
	oauthCapability := sp.GetOAuthCapability()
	if oauthCapability == nil {
		lg.Info("service provider does not support oauth capability. nothing left to be done", "serviceprovider", sp.GetType())
		return ctrl.Result{}, nil
	}

	oauthBaseUrl := oauthCapability.GetOAuthEndpoint()
	permissions := getFixedPermissionsForProvider(sp.GetType())
	scopes := oauthCapability.OAuthScopesFor(&permissions)

	state, err := oauthstate.Encode(&oauthstate.OAuthInfo{
		TokenName:           remoteSecret.Name,
		TokenNamespace:      remoteSecret.Namespace,
		Scopes:              scopes,
		ServiceProviderName: sp.GetType().Name,
		ServiceProviderUrl:  sp.GetBaseUrl(),
		IsRemoteSecret:      true,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to encode the OAuth state: %w", err)
	}

	remoteSecret.Annotations[oauthUrlLabel] = oauthBaseUrl + "?state=" + state

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Update(ctx, remoteSecret)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update RemoteSecret with OAuth URL: %w", err)
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
			Required: []api.Permission{{
				Type: api.PermissionTypeReadWrite,
				Area: api.PermissionAreaRegistry,
			}},
		}
	default:
		return api.Permissions{
			Required: []api.Permission{{
				Type: api.PermissionTypeReadWrite,
				Area: api.PermissionAreaRepository,
			}},
		}
	}
}
