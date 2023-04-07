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
	stderrors "errors"
	"fmt"
	"net/url"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

const linkedBindingsFinalizerName = "spi.appstudio.redhat.com/linked-bindings"
const tokenStorageFinalizerName = "spi.appstudio.redhat.com/token-storage" //#nosec G101 -- false positive, we're not storing any sensitive data using this
const tokenRefreshLabelName = "spi.appstudio.redhat.com/refresh-token"     //#nosec G101 -- false positive, just label name, no sensitive data

var (
	unexpectedObjectTypeError = stderrors.New("unexpected object type")
	linkedBindingPresentError = stderrors.New("linked bindings present")
	noCredentialsFoundError   = stderrors.New("no oauth configuration found matching service provider URL of the token")
)

// SPIAccessTokenReconciler reconciles a SPIAccessToken object
type SPIAccessTokenReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	TokenStorage           tokenstorage.TokenStorage
	Configuration          *opconfig.OperatorConfiguration
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
		return fmt.Errorf("failed to register the linked bindings finalizer: %w", err)
	}
	if err := r.finalizers.Register(tokenStorageFinalizerName, &tokenStorageFinalizer{storage: r.TokenStorage}); err != nil {
		return fmt.Errorf("failed to register the token storage finalizer: %w", err)
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessToken{}).
		// We're watching the bindings so that we can remove abandoned tokens without data
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestsForTokenInObjectNamespace(object, "SPIAccessTokenBinding", func() string {
				return object.GetLabels()[SPIAccessTokenLinkLabel]
			})
		})).
		Watches(&source.Kind{Type: &api.SPIAccessTokenDataUpdate{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestsForTokenInObjectNamespace(object, "SPIAccessTokenDataUpdate", func() string {
				update, ok := object.(*api.SPIAccessTokenDataUpdate)
				if !ok {
					return ""
				}

				return update.Spec.TokenName
			})
		})).
		Complete(r)

	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}

	return err
}

func requestsForTokenInObjectNamespace(object client.Object, objectKind string, tokenNameExtractor func() string) []reconcile.Request {
	tokenName := tokenNameExtractor()
	if tokenName == "" {
		return []reconcile.Request{}
	}

	reqs := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: object.GetNamespace(),
				Name:      tokenName,
			},
		},
	}

	logReconciliationRequests(reqs, "SPIAccessToken", object, objectKind)

	return reqs
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile SPIAccessToken")

	at := api.SPIAccessToken{}

	if err := r.Get(ctx, req.NamespacedName, &at); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("token already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to load the token from the cluster: %w", err)
	}

	lg = lg.WithValues("phase_at_reconcile_start", at.Status.Phase)
	log.IntoContext(ctx, lg)

	finalizationResult, err := r.finalizers.Finalize(ctx, &at)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, &at); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.Client.Status().Update(ctx, &at); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status based on finalization result: %w", err)
		}
	}

	if at.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("token being deleted, no other changes required after completed finalization")
		return ctrl.Result{}, nil
	}

	tokenLifetime := time.Since(at.CreationTimestamp.Time).Seconds()

	// cleanup tokens by lifetime or on being unreferenced by any binding in the AwaitingToken state
	if (tokenLifetime > r.Configuration.AccessTokenTtl.Seconds()) || (at.Status.Phase == api.SPIAccessTokenPhaseAwaitingTokenData && tokenLifetime > r.Configuration.DeletionGracePeriod.Seconds()) {
		hasLinkedBindings, err := hasLinkedBindings(ctx, &at, r.Client)
		if err != nil {
			lg.Error(err, "failed to check linked bindings for token", "error", err)
			return ctrl.Result{}, fmt.Errorf("failed to check linked bindings for token: %w", err)
		}
		if !hasLinkedBindings {
			err = r.Delete(ctx, &at)
			if err != nil {
				lg.Error(err, "failed to cleanup obsolete token", "error", err)
				return ctrl.Result{}, fmt.Errorf("failed to cleanup token on reaching the lifetime or being unreferenced: %w", err)
			}
			lg.V(logs.DebugLevel).Info("token being deleted on reaching іts lifetime or being unreferenced with awaiting state", "token", at.ObjectMeta.Name, "tokenLifetime", tokenLifetime, "accesstokenttl", r.Configuration.AccessTokenTtl.Seconds())
			return ctrl.Result{}, nil
		}
	}

	if errCheckData := r.reconcileTokenData(ctx, &at); errCheckData != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check the token's data in token storage: %w", errCheckData)
	}

	// persist the SP-specific state so that it is available as soon as the token flips to the ready state.
	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, at.Spec.ServiceProviderUrl, req.Namespace)
	if err != nil {
		var reason api.SPIAccessTokenErrorReason
		var validationErr validator.ValidationErrors
		if stderrors.As(err, &validationErr) {
			reason = api.SPIAccessTokenErrorUnsupportedServiceProviderConfiguration
		} else {
			reason = api.SPIAccessTokenErrorReasonUnknownServiceProvider
		}
		if uerr := r.flipToExceptionalPhase(ctx, &at, api.SPIAccessTokenPhaseError, reason, err); uerr != nil {
			return ctrl.Result{}, fmt.Errorf("failed update the status: %w", uerr)
		}
		// we flipped the token to the invalid phase, which is valid phase to be in. All we can do is to wait for the
		// next update of the token, so no need to repeat the reconciliation
		return ctrl.Result{}, nil
	}

	validation, err := sp.Validate(ctx, &at)
	if err != nil {
		lg.Error(err, "failed to validate the object")
		return ctrl.Result{}, fmt.Errorf("failed to validate the object: %w", err)
	}
	if len(validation.ScopeValidation) > 0 {
		if uerr := r.flipToExceptionalPhase(ctx, &at, api.SPIAccessTokenPhaseInvalid, api.SPIAccessTokenErrorReasonUnsupportedPermissions, NewAggregatedError(validation.ScopeValidation...)); uerr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", uerr)
		}
		return ctrl.Result{}, nil
	}

	if err := sp.PersistMetadata(ctx, r.Client, &at); err != nil {
		if sperrors.IsServiceProviderHttpInvalidAccessToken(err) {
			if uerr := r.flipToExceptionalPhase(ctx, &at, api.SPIAccessTokenPhaseInvalid, api.SPIAccessTokenErrorReasonMetadataFailure, err); uerr != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", uerr)
			}
			// the token is invalid, there's no point in repeated reconciliation
			lg.Info("access token determined invalid when trying to persist the metadata")
			return ctrl.Result{}, nil
		} else {
			if uerr := r.flipToExceptionalPhase(ctx, &at, api.SPIAccessTokenPhaseError, api.SPIAccessTokenErrorReasonMetadataFailure, err); uerr != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", uerr)
			}
			// there is some other kind of error in the service provider or environment. Let's retry...
			lg.Error(err, "failed to persist metadata")
			return ctrl.Result{}, fmt.Errorf("failed to persist the metadata: %w", err)
		}
	}

	if ensureLabels(&at, sp.GetType().Name) {
		if err := r.Update(ctx, &at); err != nil {
			lg.Error(err, "failed to update the object with the changes")
			return ctrl.Result{}, fmt.Errorf("failed to update the object with the changes: %w", err)
		}
	}

	if err := r.updateTokenStatusSuccess(ctx, &at); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
	}

	lg.V(logs.DebugLevel).Info("looking for label to initiate token refresh...")
	if val, ok := at.ObjectMeta.Labels[tokenRefreshLabelName]; ok && val == "true" { // intentional string
		if err := r.refreshToken(ctx, &at, sp); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to refresh the token: %w", err)
		}
		delete(at.ObjectMeta.Labels, tokenRefreshLabelName)
		if err := r.Update(ctx, &at); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove token refresh request label: %w", err)
		}
	}

	// this will get picked up by the time tracker
	lg = lg.WithValues("phase_at_reconcile_end", at.Status.Phase)

	return ctrl.Result{RequeueAfter: r.durationUntilNextReconcile(&at)}, nil
}

// reconcileTokenData verifies that token data are really in the storage. If data is missing, it cleans up the token's metadata
func (r *SPIAccessTokenReconciler) reconcileTokenData(ctx context.Context, at *api.SPIAccessToken) error {
	// as an optimization, we check status here first to not touch tokenstorage in cases when we don't expect data to be there
	if at.Status.TokenMetadata != nil && at.Status.Phase == api.SPIAccessTokenPhaseReady {
		if token, errGetToken := r.TokenStorage.Get(ctx, at); token == nil && errGetToken == nil {
			log.FromContext(ctx).Info("no token data found in tokenstorage for the SPIAccessToken. Clearing metadata.")
			at.Status.TokenMetadata = nil
		} else if errGetToken != nil {
			return fmt.Errorf("failed to get the token: %w", errGetToken)
		}
	}
	return nil
}

func (r *SPIAccessTokenReconciler) durationUntilNextReconcile(at *api.SPIAccessToken) time.Duration {
	return time.Until(at.CreationTimestamp.Add(r.Configuration.AccessTokenTtl).Add(r.Configuration.DeletionGracePeriod))
}

func (r *SPIAccessTokenReconciler) flipToExceptionalPhase(ctx context.Context, at *api.SPIAccessToken, phase api.SPIAccessTokenPhase, reason api.SPIAccessTokenErrorReason, err error) error {
	at.Status.Phase = phase
	at.Status.ErrorMessage = err.Error()
	at.Status.ErrorReason = reason
	if uerr := r.Client.Status().Update(ctx, at); uerr != nil {
		log.FromContext(ctx).Error(uerr, "failed to update the status with error", "reason", reason, "token_error", err)
		return fmt.Errorf("failed to update the status with error: %w", uerr)
	}

	return nil
}

func (r *SPIAccessTokenReconciler) updateTokenStatusSuccess(ctx context.Context, at *api.SPIAccessToken) error {
	if err := r.fillInStatus(ctx, at); err != nil {
		return err
	}
	at.Status.ErrorMessage = ""
	at.Status.ErrorReason = ""
	if err := r.Client.Status().Update(ctx, at); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}

// fillInStatus examines the provided token object and updates its status to match the state of the object.
func (r *SPIAccessTokenReconciler) fillInStatus(ctx context.Context, at *api.SPIAccessToken) error {
	if at.Status.TokenMetadata == nil || at.Status.TokenMetadata.Username == "" {
		oauthUrl, err := r.oAuthUrlFor(ctx, at)
		if err != nil {
			return err
		}

		at.Status.OAuthUrl = oauthUrl
		at.Status.Phase = api.SPIAccessTokenPhaseAwaitingTokenData
	} else {
		changed := at.Status.Phase != api.SPIAccessTokenPhaseReady || at.Status.OAuthUrl != ""
		at.Status.Phase = api.SPIAccessTokenPhaseReady
		at.Status.OAuthUrl = ""
		if changed {
			log.FromContext(ctx).V(logs.DebugLevel).Info("Flipping token to ready state because of metadata presence", "metadata", at.Status.TokenMetadata)
		}
	}

	at.Status.UploadUrl = fmt.Sprintf("%s/token/%s/%s", r.Configuration.BaseUrl, at.Namespace, at.Name)

	return nil
}

// oAuthUrlFor determines the OAuth flow initiation URL for given token.
func (r *SPIAccessTokenReconciler) oAuthUrlFor(ctx context.Context, at *api.SPIAccessToken) (string, error) {
	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, at.Spec.ServiceProviderUrl, at.Namespace)
	if err != nil {
		var validationErr validator.ValidationErrors
		if stderrors.As(err, &validationErr) {
			return "", fmt.Errorf("failed to validate the service provider for URL %s: %w", at.Spec.ServiceProviderUrl, validationErr)
		} else {
			return "", fmt.Errorf("failed to determine the service provider from URL %s: %w", at.Spec.ServiceProviderUrl, err)
		}
	}
	oauthCapability := sp.GetOAuthCapability()
	if oauthCapability == nil {
		log.FromContext(ctx).V(logs.DebugLevel).Info("service provider does not support oauth capability", "serviceprovider", sp.GetType())
		return "", nil
	}

	oauthBaseUrl := oauthCapability.GetOAuthEndpoint()
	if len(oauthBaseUrl) == 0 {
		return "", nil
	}

	state, err := oauthstate.Encode(&oauthstate.OAuthInfo{
		TokenName:           at.Name,
		TokenNamespace:      at.Namespace,
		Scopes:              oauthCapability.OAuthScopesFor(&at.Spec.Permissions),
		ServiceProviderName: sp.GetType().Name,
		ServiceProviderUrl:  sp.GetBaseUrl(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode the OAuth state: %w", err)
	}

	return oauthBaseUrl + "?state=" + state, nil
}

func (r *SPIAccessTokenReconciler) refreshToken(ctx context.Context, at *api.SPIAccessToken, sp serviceprovider.ServiceProvider) error {
	lg := logs.AuditLog(ctx)
	lg.Info("initiated token refresh", "action", "UPDATE")
	token, err := r.TokenStorage.Get(ctx, at)
	if err != nil {
		return fmt.Errorf("unable to get refresh token from storage: %w", err)
	}

	parsedUrl, err := url.Parse(at.Spec.ServiceProviderUrl)
	if err != nil {
		return fmt.Errorf("failed to parse service provider url from token: %w", err)
	}

	spConfig, err := config.SpConfigFromUserSecret(ctx, r.Client, at.Namespace, sp.GetType(), parsedUrl)
	if err != nil {
		return fmt.Errorf("failed to find service provider configuration in user secrets: %w", err)
	}
	if spConfig == nil {
		spConfig = config.SpConfigFromGlobalConfig(&r.Configuration.SharedConfiguration, sp.GetType(), at.Spec.ServiceProviderUrl)
	}
	if spConfig == nil || spConfig.OAuth2Config == nil {
		return noCredentialsFoundError
	}

	refreshCapability := sp.GetRefreshTokenCapability()
	if refreshCapability == nil {
		return fmt.Errorf("%s service provider type: %w", sp.GetType().Name, serviceprovider.RefreshTokenNotSupportedError{})
	}

	refreshedToken, err := refreshCapability.RefreshToken(ctx, token, spConfig.OAuth2Config)
	if err != nil {
		return fmt.Errorf("unable to refresh token: %w", err)
	}

	if err := r.TokenStorage.Store(ctx, at, refreshedToken); err != nil {
		return fmt.Errorf("unable to store refresh token: %w", err)
	}

	lg.Info("token refreshed successfully")
	return nil
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
		return res, unexpectedObjectTypeError
	}

	hasBindings, err := hasLinkedBindings(ctx, token, f.client)
	if err != nil {
		return res, err
	}

	if hasBindings {
		return res, linkedBindingPresentError
	} else {
		return res, nil
	}
}

func (f *tokenStorageFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	err := f.storage.Delete(ctx, obj.(*api.SPIAccessToken))
	if err != nil {
		err = fmt.Errorf("failed to delete the linked token during finalization of %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return finalizer.Result{}, err
}

func hasLinkedBindings(ctx context.Context, token *api.SPIAccessToken, k8sClient client.Client) (bool, error) {
	list := &api.SPIAccessTokenBindingList{}
	if err := k8sClient.List(ctx, list, client.InNamespace(token.Namespace), client.Limit(1), client.MatchingLabels{
		SPIAccessTokenLinkLabel: token.Name,
	}); err != nil {
		return false, fmt.Errorf("failed to list the linked bindings for %s/%s: %w", token.Namespace, token.Name, err)
	}

	return len(list.Items) > 0, nil
}

// EnsureLabels makes sure that the object has labels set according to its spec. The labels are used for faster lookup during
// token matching with bindings. Returns `true` if the labels were changed, `false` otherwise.
func ensureLabels(t *api.SPIAccessToken, detectedSp config.ServiceProviderName) (changed bool) {
	if t.Labels == nil {
		t.Labels = map[string]string{}
	}

	if t.Labels[api.ServiceProviderTypeLabel] != string(detectedSp) {
		t.Labels[api.ServiceProviderTypeLabel] = string(detectedSp)
		changed = true
	}

	if len(t.Spec.ServiceProviderUrl) > 0 {
		// we can't use the full service provider URL as a label value, because K8s doesn't allow :// in label values.
		spUrl, err := url.Parse(t.Spec.ServiceProviderUrl)
		if err == nil {
			if t.Labels[api.ServiceProviderHostLabel] != spUrl.Host {
				t.Labels[api.ServiceProviderHostLabel] = spUrl.Host
				changed = true
			}
		}
	}

	return
}
