/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/storage"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// SPIAccessTokenReconciler reconciles a SPIAccessToken object
type SPIAccessTokenReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	Storage                *storage.Storage
	Configuration          config.Configuration
	ServiceProviderFactory serviceprovider.Factory
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessToken{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx, "SPIAccessToken", req.NamespacedName)
	ctx = log.IntoContext(ctx, lg)

	at := api.SPIAccessToken{}

	if err := r.Get(ctx, req.NamespacedName, &at); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if at.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if at.Spec.DataLocation == "" {
		loc, err := r.Storage.GetDataLocation(&at)
		if err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to determine data path")
		}

		if loc != "" {
			data, err := r.Storage.Get(&at)
			if err != nil {
				return ctrl.Result{}, NewReconcileError(err, "failed to read the data from storage")
			}
			if data != nil {
				at.Spec.DataLocation = loc
				if err := r.Update(ctx, &at); err != nil {
					return ctrl.Result{}, NewReconcileError(err, "failed to initialize data location")
				}
			}
		}
	}

	if err := r.fillInStatus(ctx, &at); err != nil {
		return ctrl.Result{}, NewReconcileError(err, "failed to update the status")
	}

	return ctrl.Result{}, nil
}

func (r *SPIAccessTokenReconciler) fillInStatus(ctx context.Context, at *api.SPIAccessToken) error {
	changed := false

	if at.Status.Phase == "" {
		if at.Spec.DataLocation == "" {
			at.Status.Phase = api.SPIAccessTokenPhaseAwaitingTokenData
			oauthUrl, err := r.oAuthUrlFor(at)
			if err != nil {
				return err
			}
			at.Status.OAuthUrl = oauthUrl
		} else {
			at.Status.Phase = api.SPIAccessTokenPhaseReady
			at.Status.OAuthUrl = ""
		}
		changed = true
	}

	if changed {
		return r.Client.Status().Update(ctx, at)
	} else {
		return nil
	}
}

func (r *SPIAccessTokenReconciler) oAuthUrlFor(at *api.SPIAccessToken) (string, error) {
	sp, err := r.ServiceProviderFactory.FromRepoUrl(at.Spec.ServiceProviderUrl)
	if err != nil {
		return "", err
	}

	if sp.GetType() != at.Spec.ServiceProviderType {
		return "", fmt.Errorf("service provider URL not consistent with provider type")
	}

	codec, err := oauthstate.NewCodec(r.Configuration.SharedSecret)
	if err != nil {
		return "", NewReconcileError(err, "failed to instantiate OAuth state codec")
	}

	state, err := codec.EncodeAnonymous(&oauthstate.AnonymousOAuthState{
		TokenName:           at.Name,
		TokenNamespace:      at.Namespace,
		IssuedAt:            time.Now().Unix(),
		Scopes:              serviceprovider.GetAllScopes(sp, &at.Spec.Permissions),
		ServiceProviderType: config.ServiceProviderType(sp.GetType()),
		ServiceProviderUrl:  sp.GetBaseUrl(),
	})
	if err != nil {
		return "", NewReconcileError(err, "failed to encode the OAuth state")
	}

	return sp.GetOAuthEndpoint() + "?state=" + state, nil
}
