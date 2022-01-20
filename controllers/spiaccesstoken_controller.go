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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
)

const linkedBindingsFinalizerName = "spi.appstudio.redhat.com/linked-bindings"

// SPIAccessTokenReconciler reconciles a SPIAccessToken object
type SPIAccessTokenReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	TokenStorage           tokenstorage.TokenStorage
	Configuration          config.Configuration
	ServiceProviderFactory serviceprovider.Factory
	finalizers             finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokens/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedBindingsFinalizerName, &linkedBindingsFinalizer{client: r.Client}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessToken{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			tokenName := object.GetLabels()[opconfig.SPIAccessTokenLinkLabel]

			if tokenName == "" {
				return []reconcile.Request{}
			}

			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: object.GetNamespace(),
						Name:      tokenName,
					},
				},
			}
		})).
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
			lg.Info("token already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, NewReconcileError(err, "failed to load the token from the cluster")
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &at)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, NewReconcileError(err, "failed to finalize")
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, &at); err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to update based on finalization result")
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.Client.Status().Update(ctx, &at); err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to update the status based on finalization result")
		}
	}

	if at.DeletionTimestamp != nil {
		lg.Info("token being deleted, no other changes required after completed finalization")
		return ctrl.Result{}, nil
	}

	if at.Spec.DataLocation == "" {
		loc, err := r.TokenStorage.GetDataLocation(ctx, &at)
		if err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to determine data path")
		}

		if loc != "" {
			data, err := r.TokenStorage.Get(ctx, &at)
			if err != nil {
				return ctrl.Result{}, NewReconcileError(err, "failed to read the data from token storage")
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

// fillInStatus examines the provided token object and updates its status to match the state of the object.
func (r *SPIAccessTokenReconciler) fillInStatus(ctx context.Context, at *api.SPIAccessToken) error {
	changed := false

	if at.Spec.DataLocation == "" {
		oauthUrl, err := r.oAuthUrlFor(at)
		if err != nil {
			return err
		}

		changed = at.Status.OAuthUrl != oauthUrl || at.Status.Phase != api.SPIAccessTokenPhaseAwaitingTokenData

		at.Status.OAuthUrl = oauthUrl
		at.Status.Phase = api.SPIAccessTokenPhaseAwaitingTokenData
	} else {
		changed = at.Status.Phase != api.SPIAccessTokenPhaseReady || at.Status.OAuthUrl != ""
		at.Status.Phase = api.SPIAccessTokenPhaseReady
		at.Status.OAuthUrl = ""
	}

	if changed {
		return r.Client.Status().Update(ctx, at)
	} else {
		return nil
	}
}

// oAuthUrlFor determines the OAuth flow initiation URL for given token.
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

type linkedBindingsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedBindingsFinalizer)(nil)

func (f *linkedBindingsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	token, ok := obj.(*api.SPIAccessToken)
	if !ok {
		return res, fmt.Errorf("unexpected object type")
	}

	hasBindings, err := f.hasLinkedBindings(ctx, token)
	if err != nil {
		return res, err
	}

	if hasBindings {
		return res, fmt.Errorf("linked bindings present")
	} else {
		return res, nil
	}
}

func (f *linkedBindingsFinalizer) hasLinkedBindings(ctx context.Context, token *api.SPIAccessToken) (bool, error) {
	list := &api.SPIAccessTokenBindingList{}
	if err := f.client.List(ctx, list, client.InNamespace(token.Namespace), client.Limit(1), client.MatchingLabels{
		opconfig.SPIAccessTokenLinkLabel: token.Name,
	}); err != nil {
		return false, err
	}

	return len(list.Items) > 0, nil
}
