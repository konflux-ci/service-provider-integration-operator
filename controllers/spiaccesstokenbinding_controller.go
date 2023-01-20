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

	"github.com/go-logr/logr"

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"

	"sigs.k8s.io/controller-runtime/pkg/finalizer"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

const linkedSecretsFinalizerName = "spi.appstudio.redhat.com/linked-secrets" //#nosec G101 -- false positive, this is not a private data

var spiAccessTokenBindingLog = log.Log.WithName("spiaccesstokenbinding-controller")

var (
	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}

	linkedTokenDoesntMatchError     = stderrors.New("linked token doesn't match the criteria")
	accessTokenDataNotFoundError    = stderrors.New("access token data not found")
	invalidServiceProviderHostError = stderrors.New("the host of service provider url, determined from repoUrl, is not a valid DNS1123 subdomain")
	minimalBindingLifetimeError     = stderrors.New("a specified binding lifetime is less than 60s, which cannot be accepted")
)

// SPIAccessTokenBindingReconciler reconciles a SPIAccessTokenBinding object
type SPIAccessTokenBindingReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	TokenStorage           tokenstorage.TokenStorage
	Configuration          *opconfig.OperatorConfiguration
	syncer                 sync.Syncer
	ServiceProviderFactory serviceprovider.Factory
	finalizers             finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;watch;create;update;list;delete

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.syncer = sync.New(mgr.GetClient())
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedSecretsFinalizerName, &linkedSecretsFinalizer{client: r.Client}); err != nil {
		return fmt.Errorf("failed to register the linked secrets finalizer: %w", err)
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessTokenBinding{}).
		Watches(&source.Kind{Type: &api.SPIAccessToken{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			requests, err := r.filteredBindingsAsRequests(context.Background(), o.GetNamespace(), func(_ api.SPIAccessTokenBinding) bool { return true })
			if err != nil {
				spiAccessTokenBindingLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to SPIAccessToken",
					"SPIAccessTokenName", o.GetName(), "SPIAccessTokenNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}

			return requests
		})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			requests, err := r.filteredBindingsAsRequests(context.Background(), o.GetNamespace(), func(binding api.SPIAccessTokenBinding) bool {
				return binding.Status.SyncedObjectRef.Kind == "Secret" && binding.Status.SyncedObjectRef.Name == o.GetName()

			})
			if err != nil {
				spiAccessTokenBindingLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to Secret",
					"SecretName", o.GetName(), "SecretNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}
			return requests
		})).
		Complete(r)
	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}
	return err
}

type BindingMatchingFunc func(api.SPIAccessTokenBinding) bool

// filteredBindingsAsRequests filters all bindings in a given namespace by a BindingMatchingFunc and creates reconcile requests for every one after filtering.
func (r *SPIAccessTokenBindingReconciler) filteredBindingsAsRequests(ctx context.Context, namespace string, matchingFunc BindingMatchingFunc) ([]reconcile.Request, error) {
	bindings := &api.SPIAccessTokenBindingList{}
	if err := r.Client.List(ctx, bindings, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list bindings in the namespace %s, error: %w", namespace, err)
	}
	ret := make([]reconcile.Request, 0, len(bindings.Items))
	for _, b := range bindings.Items {
		if matchingFunc(b) {
			ret = append(ret, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      b.Name,
					Namespace: b.Namespace,
				},
			})
		}
	}
	return ret, nil
}

func (r *SPIAccessTokenBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile SPIAccessTokenBinding")

	binding := api.SPIAccessTokenBinding{}

	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("binding object not found in cluster, skipping reconciliation")
			return ctrl.Result{}, nil
		}

		lg.Error(err, "failed to get the object")
		return ctrl.Result{}, fmt.Errorf("failed to read the object: %w", err)
	}

	sp, rerr := r.getServiceProvider(ctx, &binding)
	if rerr != nil {
		lg.Error(rerr, "unable to get the service provider")
		// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
		// user fixes that URL. So no need to repeat the reconciliation and therefore no error returned here.
		return ctrl.Result{}, nil
	}

	if checkQuayPermissionAreasMigration(&binding, sp.GetType()) {
		lg.Info("migrating old permission areas for quay", "binding", binding)
		if err := r.Client.Update(ctx, &binding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update binding with migrated permission areas: %w", err)
		}
		return ctrl.Result{}, nil
	}

	lg = lg.WithValues("linked_to", binding.Status.LinkedAccessTokenName,
		"phase_at_reconcile_start", binding.Status.Phase)

	finalizationResult, err := r.finalizers.Finalize(ctx, &binding)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, &binding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.Client.Status().Update(ctx, &binding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status based on finalization result: %w", err)
		}
	}

	if binding.DeletionTimestamp != nil {
		lg.Info("object is being deleted")
		return ctrl.Result{}, nil
	}

	expectedLifetimeDuration, err := bindingLifetime(r, binding)
	if err != nil {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonInvalidLifetime, err)
		return ctrl.Result{}, fmt.Errorf("binding lifetime processing failed: %w", err)
	}

	// cleanup bindings by lifetime
	bindingLifetime := time.Since(binding.CreationTimestamp.Time).Seconds()
	if expectedLifetimeDuration != nil && bindingLifetime > expectedLifetimeDuration.Seconds() {
		err := r.Client.Delete(ctx, &binding)
		if err != nil {
			lg.Error(err, "failed to cleanup binding on reaching the max lifetime", "error", err)
			return ctrl.Result{}, fmt.Errorf("failed to cleanup binding on reaching the max lifetime: %w", err)
		}
		lg.V(logs.DebugLevel).Info("binding being cleaned up on reaching the max lifetime", "binding", binding.ObjectMeta.Name, "bindingLifetime", bindingLifetime, "bindingttl", r.Configuration.AccessTokenBindingTtl.Seconds())
		return ctrl.Result{}, nil
	}

	if binding.Status.Phase == "" {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseAwaitingTokenData
	}

	validation, err := sp.Validate(ctx, &binding)
	if err != nil {
		lg.Error(err, "failed to validate the object")
		return ctrl.Result{}, fmt.Errorf("failed to validate the object: %w", err)
	}
	if len(validation.ScopeValidation) > 0 {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonUnsupportedPermissions, NewAggregatedError(validation.ScopeValidation...))
		return ctrl.Result{}, nil
	}

	ctx = log.IntoContext(ctx, lg)

	var token *api.SPIAccessToken
	var matching bool
	token, matching, err = r.linkToken(ctx, sp, &binding)
	if err != nil {
		lg.Error(err, "unable to link the token")
		return ctrl.Result{}, fmt.Errorf("failed to link the token: %w", err)
	}
	lg = lg.WithValues("linked_to", token.Name)
	if !matching && token.Status.Phase == api.SPIAccessTokenPhaseReady {
		// the token that we are linked to is ready but doesn't match the criteria of the binding.
		// We can't do much here - the user granted the token the access we requested, but we still don't match
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		binding.Status.OAuthUrl = ""
		r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, linkedTokenDoesntMatchError)
		return ctrl.Result{}, nil
	}

	origSyncedObject := binding.Status.SyncedObjectRef

	if token != nil {
		binding.Status.OAuthUrl = token.Status.OAuthUrl
		binding.Status.UploadUrl = token.Status.UploadUrl

		if token.Status.Phase == api.SPIAccessTokenPhaseReady {
			ref, err := r.syncSecret(ctx, sp, &binding, token)
			if err != nil {
				lg.Error(err, "unable to sync the secret")
				return ctrl.Result{}, fmt.Errorf("failed to sync the secret: %w", err)
			}
			binding.Status.SyncedObjectRef = ref
			binding.Status.Phase = api.SPIAccessTokenBindingPhaseInjected
		} else {
			binding.Status.Phase = api.SPIAccessTokenBindingPhaseAwaitingTokenData
			binding.Status.SyncedObjectRef = api.TargetObjectRef{}
		}

		if err := r.updateBindingStatusSuccess(ctx, &binding); err != nil {
			lg.Error(err, "unable to update the status")
			return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
		}
	}

	if binding.Status.Phase != api.SPIAccessTokenBindingPhaseInjected && origSyncedObject.Name != "" {
		if err = deleteSyncedSecret(ctx, r.Client, origSyncedObject.Name, binding.Namespace); err != nil {
			lg.Error(err, "failed to delete the potential stale synced secret")
			r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
			return ctrl.Result{}, err
		}
	}

	// this will be used in the deferred time tracker log message
	lg = lg.WithValues("phase_at_reconcile_end", binding.Status.Phase)

	if expectedLifetimeDuration == nil {
		// no need to re-schedule by any timeout
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: time.Until(binding.CreationTimestamp.Add(*expectedLifetimeDuration).Add(r.Configuration.DeletionGracePeriod))}, nil
}

// returns user specified binding lifetime or default binding lifetime or nil if set to infinite
func bindingLifetime(r *SPIAccessTokenBindingReconciler, binding api.SPIAccessTokenBinding) (*time.Duration, error) {
	switch binding.Spec.Lifetime {
	case "":
		return &r.Configuration.AccessTokenBindingTtl, nil
	case "-1":
		return nil, nil
	default:
		expectedLifetimeDuration, err := time.ParseDuration(binding.Spec.Lifetime)
		if err != nil {
			return nil, fmt.Errorf("invalid binding lifetime format specified: %w", err)
		}
		if expectedLifetimeDuration.Seconds() < 60 {
			return nil, minimalBindingLifetimeError
		}
		return &expectedLifetimeDuration, nil
	}
}

func checkQuayPermissionAreasMigration(binding *api.SPIAccessTokenBinding, spType api.ServiceProviderType) bool {
	permissionChange := false
	if spType == api.ServiceProviderTypeQuay {
		for i, permission := range binding.Spec.Permissions.Required {
			if permission.Area == api.PermissionAreaRepository {
				binding.Spec.Permissions.Required[i].Area = api.PermissionAreaRegistry
				permissionChange = true
			}
			if permission.Area == api.PermissionAreaRepositoryMetadata {
				binding.Spec.Permissions.Required[i].Area = api.PermissionAreaRegistryMetadata
				permissionChange = true
			}
		}
	}
	return permissionChange
}

func getLinkedTokenFromList(s *api.SPIAccessTokenBinding, tokens []api.SPIAccessToken) *api.SPIAccessToken {
	for _, t := range tokens {
		if t.Name == s.Status.LinkedAccessTokenName {
			return &t
		}
	}

	return nil
}

// getServiceProvider obtains the service provider instance according to the repository URL from the binding's spec.
// The status of the binding is immediately persisted with an error if the service provider cannot be determined.
func (r *SPIAccessTokenBindingReconciler) getServiceProvider(ctx context.Context, binding *api.SPIAccessTokenBinding) (serviceprovider.ServiceProvider, error) {
	serviceProvider, err := r.ServiceProviderFactory.FromRepoUrl(ctx, binding.Spec.RepoUrl, binding.Namespace)
	if err != nil {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
		return nil, fmt.Errorf("failed to find the service provider: %w", err)
	}

	return serviceProvider, nil
}

// linkToken updates the binding with a link to an SPIAccessToken object that should hold the token data. If no
// suitable SPIAccessToken object exists, it is created (in an awaiting state) and linked.
func (r *SPIAccessTokenBindingReconciler) linkToken(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding) (token *api.SPIAccessToken, matching bool, err error) {
	lg := log.FromContext(ctx)
	tokens, err := sp.LookupTokens(ctx, r.Client, binding)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenLookup, err)
		return nil, false, fmt.Errorf("failed to lookup the matching tokens in the service provider: %w", err)
	}

	newTokenCreated := false
	if len(tokens) == 0 {
		matching = false
		if binding.Status.LinkedAccessTokenName != "" {
			// ok, there are no matching tokens, but we're already linked to one. So let's just load that token here...
			token = &api.SPIAccessToken{}
			if err = r.Client.Get(ctx, client.ObjectKey{Name: binding.Status.LinkedAccessTokenName, Namespace: binding.Namespace}, token); err != nil {
				return nil, false, fmt.Errorf("failed to get the linked token: %w", err)
			}
		} else {
			lg.V(logs.DebugLevel).Info("creating a new token because none found for binding")

			serviceProviderUrl := sp.GetBaseUrl()
			if err := validateServiceProviderUrl(serviceProviderUrl); err != nil {
				binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
				r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
				return nil, false, fmt.Errorf("failed to determine the service provider URL from the repo: %w", err)
			}

			// create the token (and let its webhook and controller finish the setup)
			token = &api.SPIAccessToken{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "generated-spi-access-token-",
					Namespace:    binding.Namespace,
				},
				Spec: api.SPIAccessTokenSpec{
					Permissions:        binding.Spec.Permissions,
					ServiceProviderUrl: serviceProviderUrl,
				},
			}

			if err := r.Client.Create(ctx, token); err != nil {
				r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, err)
				return nil, false, fmt.Errorf("failed to create the token: %w", err)
			}
			newTokenCreated = true
			// we've just created a new token. It technically doesn't match, but we want to give it a chance at least
			// until the next reconciliation.
			matching = true
		}
	} else {
		// if the binding is already linked to a matching token, no change needed. Otherwise, we need to link it to one
		// of the results (we randomly choose the first one ;) ).
		token = getLinkedTokenFromList(binding, tokens)
		if token == nil {
			token = &tokens[0]
		}
		matching = true
	}

	// we need to have this label so that updates to the linked SPIAccessToken are reflected here, too... We're setting
	// up the watch to use the label to limit the scope...
	if err := r.persistWithMatchingLabels(ctx, binding, token); err != nil {
		// linking newly created token failed, lets cleanup it
		if newTokenCreated {
			lg.Error(err, "linking of the created token failed, cleaning up token.", "namespace", token.GetNamespace(), "token", token.GetName())
			err := r.Client.Delete(ctx, token)
			if err != nil {
				lg.Error(err, "failed to delete token after the an unsuccessful linking attempt", "namespace", token.GetNamespace(), "token", token.GetName())
			}
		}
		return nil, false, err
	}

	return
}

func validateServiceProviderUrl(serviceProviderUrl string) error {
	parse, err := url.Parse(serviceProviderUrl)
	if err != nil {
		return fmt.Errorf("the service provider url, determined from repoUrl, is not parsable: %w", err)
	}
	if errs := kubevalidation.IsDNS1123Subdomain(parse.Host); len(errs) > 0 {
		return invalidServiceProviderHostError
	}
	return nil
}

func (r *SPIAccessTokenBindingReconciler) persistWithMatchingLabels(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) error {
	if binding.Labels[SPIAccessTokenLinkLabel] != token.Name {
		if binding.Labels == nil {
			binding.Labels = map[string]string{}
		}
		binding.Labels[SPIAccessTokenLinkLabel] = token.Name

		if err := r.Client.Update(ctx, binding); err != nil {
			r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, err)
			return fmt.Errorf("failed to update the binding with the token link: %w", err)
		}
	}

	if binding.Status.LinkedAccessTokenName != token.Name {
		binding.Status.LinkedAccessTokenName = token.Name
		binding.Status.OAuthUrl = token.Status.OAuthUrl
		binding.Status.UploadUrl = token.Status.UploadUrl
		if err := r.updateBindingStatusSuccess(ctx, binding); err != nil {
			r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, err)
			return fmt.Errorf("failed to update the binding status with the token link: %w", err)
		}
	}

	return nil
}

// updateBindingStatusError updates the status of the binding with the provided error
func (r *SPIAccessTokenBindingReconciler) updateBindingStatusError(ctx context.Context, binding *api.SPIAccessTokenBinding, reason api.SPIAccessTokenBindingErrorReason, err error) {
	binding.Status.ErrorMessage = err.Error()
	binding.Status.ErrorReason = reason
	if err := r.Client.Status().Update(ctx, binding); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status with error", "status_to_update", binding.Status, "reason", reason, "error", err)
	}
}

// updateBindingStatusSuccess updates the status of the binding as successful, clearing any previous error state.
func (r *SPIAccessTokenBindingReconciler) updateBindingStatusSuccess(ctx context.Context, binding *api.SPIAccessTokenBinding) error {
	binding.Status.ErrorMessage = ""
	binding.Status.ErrorReason = ""
	if err := r.Client.Status().Update(ctx, binding); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status", "status_to_update", binding.Status, "error", err)
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}

// syncSecret creates/updates/deletes the secret specified in the binding with the token data and returns a reference
// to the secret.
func (r *SPIAccessTokenBindingReconciler) syncSecret(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding, tokenObject *api.SPIAccessToken) (api.TargetObjectRef, error) {
	token, err := r.TokenStorage.Get(ctx, tokenObject)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, err)
		return api.TargetObjectRef{}, fmt.Errorf("failed to get the token data from token storage: %w", err)
	}

	if token == nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, accessTokenDataNotFoundError)
		return api.TargetObjectRef{}, accessTokenDataNotFoundError
	}

	at, err := sp.MapToken(ctx, binding, tokenObject, token)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenAnalysis, err)
		return api.TargetObjectRef{}, fmt.Errorf("failed to analyze the token to produce the mapping to the secret: %w", err)
	}

	stringData := at.ToSecretType(binding.Spec.Secret.Type)
	at.FillByMapping(&binding.Spec.Secret.Fields, stringData)

	// copy the string data into the byte-array data so that sync works reliably. If we didn't sync, we could have just
	// used the Secret.StringData, but Sync gives us other goodies.
	// So let's bite the bullet and convert manually here.
	data := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		data[k] = []byte(v)
	}

	secretName := binding.Status.SyncedObjectRef.Name
	if secretName == "" {
		secretName = binding.Spec.Secret.Name
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   binding.GetNamespace(),
			Labels:      binding.Spec.Secret.Labels,
			Annotations: binding.Spec.Secret.Annotations,
		},
		Data: data,
		Type: binding.Spec.Secret.Type,
	}

	if secret.Name == "" {
		secret.GenerateName = binding.Name + "-secret-"
	}

	_, obj, err := r.syncer.Sync(ctx, nil, secret, secretDiffOpts)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
		return api.TargetObjectRef{}, fmt.Errorf("failed to sync the secret with the token data: %w", err)
	}
	return toObjectRef(obj), nil
}

func deleteSyncedSecret(ctx context.Context, cl client.Client, secretName string, secretNamespace string) error {
	if secretName == "" {
		return nil
	}

	secret := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get the secret to delete: %w", err)
	}

	if err := cl.Delete(ctx, secret); err != nil {
		return fmt.Errorf("failed to delete the injected secret: %w", err)
	}

	return nil
}

// toObjectRef creates a reference to a kubernetes object within the same namespace (i.e, a struct containing the name,
// kind and API version of the target object).
func toObjectRef(obj client.Object) api.TargetObjectRef {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return api.TargetObjectRef{
		Name:       obj.GetName(),
		Kind:       kind,
		ApiVersion: apiVersion,
	}
}

type linkedSecretsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedSecretsFinalizer)(nil)

// Finalize removes the secret synced to the actual binging being deleted
func (f *linkedSecretsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	binding, ok := obj.(*api.SPIAccessTokenBinding)
	if !ok {
		return res, unexpectedObjectTypeError
	}

	if err := deleteSyncedSecret(ctx, f.client, binding.Status.SyncedObjectRef.Name, binding.Namespace); err != nil {
		return res, err
	}
	return finalizer.Result{}, nil
}
