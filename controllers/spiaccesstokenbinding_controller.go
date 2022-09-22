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

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/kcp-dev/logicalcluster/v2"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/infrastructure"

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

const linkedSecretsFinalizerName = "spi.appstudio.redhat.com/linked-secrets" //nolint:gosec

var spiAccessTokenBindingLog = log.Log.WithName("spiaccesstokenbinding-controller")

var (
	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}

	linkedTokenDoesntMatchError     = stderrors.New("linked token doesn't match the criteria")
	accessTokenDataNotFoundError    = stderrors.New("access token data not found")
	invalidServiceProviderHostError = stderrors.New("the host of service provider url, determined from repoUrl, is not a valid DNS1123 subdomain")
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
			kcpWorkspace := logicalcluster.From(o)
			ctx := context.TODO()
			if !kcpWorkspace.Empty() {
				ctx = logicalcluster.WithCluster(ctx, kcpWorkspace)
			}

			requests, err := r.filteredBindingsAsRequests(ctx, kcpWorkspace.String(), o.GetNamespace(), func(_ api.SPIAccessTokenBinding) bool { return true })
			if err != nil {
				spiAccessTokenBindingLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to SPIAccessToken",
					"SPIAccessTokenName", o.GetName(), "SPIAccessTokenNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}

			return requests
		})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			kcpWorkspace := logicalcluster.From(o)
			ctx := context.TODO()
			if !kcpWorkspace.Empty() {
				ctx = logicalcluster.WithCluster(ctx, kcpWorkspace)
			}

			requests, err := r.filteredBindingsAsRequests(ctx, kcpWorkspace.String(), o.GetNamespace(), func(binding api.SPIAccessTokenBinding) bool {
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

//filteredBindingsAsRequests filters all bindings in a given namespace by a BindingMatchingFunc and creates reconcile requests for every one after filtering.
func (r *SPIAccessTokenBindingReconciler) filteredBindingsAsRequests(ctx context.Context, kcpWorkspace string, namespace string, matchingFunc BindingMatchingFunc) ([]reconcile.Request, error) {
	bindings := &api.SPIAccessTokenBindingList{}
	if err := r.Client.List(ctx, bindings, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list bindings in the namespace %s, error: %w", namespace, err)
	}
	ret := make([]reconcile.Request, 0, len(bindings.Items))
	for _, b := range bindings.Items {
		if matchingFunc(b) {
			ret = append(ret, reconcile.Request{
				ClusterName: kcpWorkspace,
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
	ctx = infrastructure.InitKcpControllerContext(ctx, req)

	lg := log.FromContext(ctx)
	defer logs.TimeTrack(lg, time.Now(), "Reconcile SPIAccessTokenBinding")

	binding := api.SPIAccessTokenBinding{}

	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("object not found")
			return ctrl.Result{}, nil
		}

		lg.Error(err, "failed to get the object")
		return ctrl.Result{}, fmt.Errorf("failed to read the object: %w", err)
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

	// cleanup bindings by lifetime
	bindingLifetime := time.Since(binding.CreationTimestamp.Time).Seconds()
	if bindingLifetime > r.Configuration.AccessTokenBindingTtl.Seconds() {
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

	sp, rerr := r.getServiceProvider(ctx, &binding)
	if rerr != nil {
		lg.Error(rerr, "unable to get the service provider")
		// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
		// user fixes that URL. So no need to repeat the reconciliation and therefore no error returned here.
		return ctrl.Result{}, nil
	}

	validation, err := sp.Validate(ctx, &binding)
	if err != nil {
		lg.Error(err, "failed to validate the object")
		return ctrl.Result{}, fmt.Errorf("failed to validate the object: %w", err)
	}
	if len(validation.ScopeValidation) > 0 {
		r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonUnsupportedPermissions, NewAggregatedError(validation.ScopeValidation...))
		return ctrl.Result{}, nil
	}

	var token *api.SPIAccessToken

	if binding.Status.LinkedAccessTokenName == "" {
		var err error
		token, err = r.linkToken(ctx, sp, &binding)
		if err != nil {
			lg.Error(err, "unable to link the token")
			return ctrl.Result{}, fmt.Errorf("failed to link the token: %w", err)
		}

		lg = lg.WithValues("linked_to", binding.Status.LinkedAccessTokenName, "token_phase", token.Status.Phase)
	} else {
		token = &api.SPIAccessToken{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: binding.Status.LinkedAccessTokenName, Namespace: binding.Namespace}, token); err != nil {
			if errors.IsNotFound(err) {
				binding.Status.LinkedAccessTokenName = ""
				r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, err)
			}
			lg.Error(err, "failed to fetch the linked token")
			return ctrl.Result{}, fmt.Errorf("failed to fetch the linked token: %w", err)
		}
		lg = lg.WithValues("token_phase", token.Status.Phase)

		if token.Status.Phase == api.SPIAccessTokenPhaseReady && binding.Status.SyncedObjectRef.Name == "" {
			// we've not yet synced the token... let's check that it fulfills the reqs
			newToken, err := sp.LookupToken(ctx, r.Client, &binding)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to lookup token before definitely assigning it to the binding: %w", err)
			}
			if newToken == nil {
				// the token that we are linked to is ready but doesn't match the criteria of the binding.
				// We can't do much here - the user granted the token the access we requested, but we still don't match
				binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
				binding.Status.OAuthUrl = ""
				r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, linkedTokenDoesntMatchError)
				return ctrl.Result{}, nil
			}

			if newToken.UID != token.UID {
				if err = r.persistWithMatchingLabels(ctx, &binding, newToken); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to persist the newly matching token: %w", err)
				}
				token = newToken
				lg = lg.WithValues("new_token_phase", token.Status.Phase, "new_token", newToken.Name)
			}
		} else if token.Status.Phase != api.SPIAccessTokenPhaseReady {
			// let's try to do a lookup in case another token started matching our reqs
			// this time, only do the lookup in SP and don't create a new token if no match found
			//
			// yes, this can create garbage - abandoned tokens, see https://issues.redhat.com/browse/SVPI-65
			newToken, err := sp.LookupToken(ctx, r.Client, &binding)
			if err != nil {
				lg.Error(err, "failed lookup when trying to reassign linked token")
				// we're not returning the error or writing the status here, because the binding already has a valid
				// linked token.
			} else if newToken != nil {
				// yay, we found another match! Let's persist that change otherwise we could enter a weird state below,
				// where we would be syncing a secret that comes from a token that is not linked
				if err = r.persistWithMatchingLabels(ctx, &binding, newToken); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to persist the newly matching token: %w", err)
				}
				token = newToken
				lg = lg.WithValues("new_token_phase", token.Status.Phase, "new_token", newToken.Name)
			}
		}
	}

	binding.Status.OAuthUrl = token.Status.OAuthUrl
	binding.Status.UploadUrl = token.Status.UploadUrl

	existingSyncedSecretName := ""
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
		existingSyncedSecretName = binding.Status.SyncedObjectRef.Name
		binding.Status.SyncedObjectRef = api.TargetObjectRef{}
	}

	if err := r.updateBindingStatusSuccess(ctx, &binding); err != nil {
		lg.Error(err, "unable to update the status")
		return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
	}

	// now that we set up the binding correctly, we need to clean up the potentially dangling secret (that might contain
	// stale data if the data of the token disappeared from the token)
	if binding.Status.Phase == api.SPIAccessTokenBindingPhaseAwaitingTokenData {
		if err := deleteSyncedSecret(ctx, r.Client, existingSyncedSecretName, binding.Namespace); err != nil {
			lg.Error(err, "failed to delete the stale synced object")
			r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
		}
	}
	return ctrl.Result{RequeueAfter: r.durationUntilNextReconcile(&binding)}, nil
}

func (r *SPIAccessTokenBindingReconciler) durationUntilNextReconcile(tb *api.SPIAccessTokenBinding) time.Duration {
	return time.Until(tb.CreationTimestamp.Add(r.Configuration.AccessTokenBindingTtl).Add(r.Configuration.DeletionGracePeriod * time.Second))
}

// getServiceProvider obtains the service provider instance according to the repository URL from the binding's spec.
// The status of the binding is immediately persisted with an error if the service provider cannot be determined.
func (r *SPIAccessTokenBindingReconciler) getServiceProvider(ctx context.Context, binding *api.SPIAccessTokenBinding) (serviceprovider.ServiceProvider, error) {
	serviceProvider, err := r.ServiceProviderFactory.FromRepoUrl(ctx, binding.Spec.RepoUrl)
	if err != nil {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
		return nil, fmt.Errorf("failed to find the service provider: %w", err)
	}

	return serviceProvider, nil
}

// linkToken updates the binding with a link to an SPIAccessToken object that should hold the token data. If no
// suitable SPIAccessToken object exists, it is created (in an awaiting state) and linked.
func (r *SPIAccessTokenBindingReconciler) linkToken(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	lg := log.FromContext(ctx)
	token, err := sp.LookupToken(ctx, r.Client, binding)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenLookup, err)
		return nil, fmt.Errorf("failed to lookup the token in the service provider: %w", err)
	}

	newTokenCreated := false
	if token == nil {
		lg.V(logs.DebugLevel).Info("creating a new token because none found for binding")

		serviceProviderUrl := sp.GetBaseUrl()
		if err := validateServiceProviderUrl(serviceProviderUrl); err != nil {
			binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
			r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
			return nil, fmt.Errorf("failed to determine the service provider URL from the repo: %w", err)
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
			return nil, fmt.Errorf("failed to create the token: %w", err)
		}
		newTokenCreated = true
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
		return nil, err
	}

	return token, nil
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
		log.FromContext(ctx).Error(err, "failed to update the status with error", "reason", reason, "error", err)
	}
}

// updateBindingStatusSuccess updates the status of the binding as successful, clearing any previous error state.
func (r *SPIAccessTokenBindingReconciler) updateBindingStatusSuccess(ctx context.Context, binding *api.SPIAccessTokenBinding) error {
	binding.Status.ErrorMessage = ""
	binding.Status.ErrorReason = ""
	if err := r.Client.Status().Update(ctx, binding); err != nil {
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
