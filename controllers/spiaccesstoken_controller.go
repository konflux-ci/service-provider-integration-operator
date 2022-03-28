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
const tokenStorageFinalizerName = "spi.appstudio.redhat.com/token-storage"

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
	if err := r.finalizers.Register(tokenStorageFinalizerName, &tokenStorageFinalizer{storage: r.TokenStorage}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessToken{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestsForTokenInObjectNamespace(object, func() string {
				return object.GetLabels()[opconfig.SPIAccessTokenLinkLabel]
			})
		})).
		Watches(&source.Kind{Type: &api.SPIAccessTokenDataUpdate{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestsForTokenInObjectNamespace(object, func() string {
				update, ok := object.(*api.SPIAccessTokenDataUpdate)
				if !ok {
					return ""
				}

				return update.Spec.TokenName
			})
		})).
		Complete(r)
}

func requestsForTokenInObjectNamespace(object client.Object, tokenNameExtractor func() string) []reconcile.Request {
	tokenName := tokenNameExtractor()
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
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling")

	at := api.SPIAccessToken{}

	if err := r.Get(ctx, req.NamespacedName, &at); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("token already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, NewReconcileError(err, "failed to load the token from the cluster")
	}

	lg = lg.WithValues("phase_at_reconcile_start", at.Status.Phase)

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

	// persist the SP-specific state so that it is available as soon as the token flips to the ready state.
	sp, err := r.ServiceProviderFactory.FromRepoUrl(at.Spec.ServiceProviderUrl)
	if err != nil {
		return ctrl.Result{}, NewReconcileError(err, "failed to determine the service provider")
	}

	if err := sp.PersistMetadata(ctx, r.Client, &at); err != nil {
		return ctrl.Result{}, NewReconcileError(err, "failed to persist token metadata")
	}

	if at.EnsureLabels(sp.GetType()) {
		if err := r.Update(ctx, &at); err != nil {
			lg.Error(err, "failed to update the object with the changes")
			return ctrl.Result{}, NewReconcileError(err, "failed to update the object with the changes")
		}
	}

	if err := r.fillInStatus(ctx, &at); err != nil {
		return ctrl.Result{}, NewReconcileError(err, "failed to update the status")
	}

	lg.WithValues("phase_at_reconcile_end", at.Status.Phase).
		Info("reconciliation finished successfully")

	return ctrl.Result{}, nil
}

// fillInStatus examines the provided token object and updates its status to match the state of the object.
func (r *SPIAccessTokenReconciler) fillInStatus(ctx context.Context, at *api.SPIAccessToken) error {
	changed := false

	if at.Status.TokenMetadata == nil || at.Status.TokenMetadata.Username == "" {
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
		if changed {
			lg := log.FromContext(ctx)
			lg.Info("Flipping token to ready state because of metadata presence", "metadata", at.Status.TokenMetadata)
		}
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

	codec, err := oauthstate.NewCodec(r.Configuration.SharedSecret)
	if err != nil {
		return "", NewReconcileError(err, "failed to instantiate OAuth state codec")
	}

	state, err := codec.EncodeAnonymous(&oauthstate.AnonymousOAuthState{
		TokenName:           at.Name,
		TokenNamespace:      at.Namespace,
		IssuedAt:            time.Now().Unix(),
		Scopes:              serviceprovider.GetAllScopes(sp.TranslateToScopes, &at.Spec.Permissions),
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

type tokenStorageFinalizer struct {
	storage tokenstorage.TokenStorage
}

var _ finalizer.Finalizer = (*linkedBindingsFinalizer)(nil)
var _ finalizer.Finalizer = (*tokenStorageFinalizer)(nil)

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

func (f *tokenStorageFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	return finalizer.Result{}, f.storage.Delete(ctx, obj.(*api.SPIAccessToken))
}
