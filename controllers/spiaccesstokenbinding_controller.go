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

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const deprecatedLinkedSecretsFinalizerName = "spi.appstudio.redhat.com/linked-secrets" //#nosec G101 -- false positive, this is not a private data
const linkedObjectsFinalizerName = "spi.appstudio.redhat.com/linked-objects"

var (
	// pre-allocated empty map so that we don't have to allocate new empty instances in the serviceAccountSecretDiffOpts
	emptySecretData = map[string][]byte{}

	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}

	// the service account secrets are treated specially by Kubernetes that automatically adds "ca.crt", "namespace" and
	// "token" entries into the secret's data.
	serviceAccountSecretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
		cmp.FilterPath(func(p cmp.Path) bool {
			return p.Last().String() == ".Data"
		}, cmp.Comparer(func(a map[string][]byte, b map[string][]byte) bool {
			// cmp.Equal short-circuits if it sees nil maps - but we don't want that...
			if a == nil {
				a = emptySecretData
			}
			if b == nil {
				b = emptySecretData
			}

			return cmp.Equal(a, b, cmpopts.IgnoreMapEntries(func(key string, _ []byte) bool {
				switch key {
				case "ca.crt", "namespace", "token":
					return true
				default:
					return false
				}
			}))
		}),
		),
	}
	linkedTokenDoesntMatchError     = stderrors.New("linked token doesn't match the criteria")
	accessTokenDataNotFoundError    = stderrors.New("access token data not found")
	invalidServiceProviderHostError = stderrors.New("the host of service provider url, determined from repoUrl, is not a valid DNS1123 subdomain")
	minimalBindingLifetimeError     = stderrors.New("a specified binding lifetime is less than 60s, which cannot be accepted")
	bindingConsistencyError         = stderrors.New("binding consistency error")
	serviceAccountLabelPredicate    predicate.Predicate
)

func init() {
	var err error
	serviceAccountLabelPredicate, err = predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Operator: metav1.LabelSelectorOpExists,
				Key:      SPIAccessTokenBindingLinkLabel,
			},
		},
	})
	utilruntime.Must(err)
}

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
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;delete

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.syncer = sync.New(mgr.GetClient())
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedObjectsFinalizerName, &linkedObjectsFinalizer{client: r.Client}); err != nil {
		return fmt.Errorf("failed to register the linked secrets finalizer: %w", err)
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessTokenBinding{}).
		Watches(&source.Kind{Type: &api.SPIAccessToken{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			requests, err := r.filteredBindingsAsRequests(context.Background(), o.GetNamespace(), func(_ api.SPIAccessTokenBinding) bool { return true })
			if err != nil {
				enqueueLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to SPIAccessToken",
					"SPIAccessTokenName", o.GetName(), "SPIAccessTokenNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}

			logReconciliationRequests(requests, "SPIAccessTokenBinding", o, "SPIAccessToken")

			return requests
		})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			requests, err := r.filteredBindingsAsRequests(context.Background(), o.GetNamespace(), func(binding api.SPIAccessTokenBinding) bool {
				return binding.Status.SyncedObjectRef.Kind == "Secret" && binding.Status.SyncedObjectRef.Name == o.GetName()

			})
			if err != nil {
				enqueueLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to Secret",
					"SecretName", o.GetName(), "SecretNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}

			logReconciliationRequests(requests, "SPIAccessTokenBinding", o, "Secret")

			return requests
		})).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			requests, err := r.filteredBindingsAsRequests(context.Background(), o.GetNamespace(), func(binding api.SPIAccessTokenBinding) bool {
				return binding.Name == o.GetLabels()[SPIAccessTokenBindingLinkLabel]
			})
			if err != nil {
				enqueueLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to a ServiceAccount",
					"ServiceAccountName", o.GetName(), "ServiceAccountNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}

			logReconciliationRequests(requests, "SPIAccessTokenBinding", o, "ServiceAccount")

			return requests
		}), builder.WithPredicates(serviceAccountLabelPredicate)).
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
			lg.Info("object not found")
			return ctrl.Result{}, nil
		}

		lg.Error(err, "failed to get the object")
		return ctrl.Result{}, fmt.Errorf("failed to read the object: %w", err)
	}

	if err := r.migrateFinalizers(ctx, &binding); err != nil {
		return ctrl.Result{}, err
	}

	sp, rerr := r.getServiceProvider(ctx, &binding)
	if rerr != nil {
		lg.Error(rerr, "unable to get the service provider")
		// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
		// user fixes that URL. So no need to repeat the reconciliation and therefore no error returned here.
		return ctrl.Result{}, nil
	}

	if checkQuayPermissionAreasMigration(&binding, sp.GetType().Name) {
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

	val := binding.Validate()
	if len(val.Consistency) > 0 {
		validationErrors := NewAggregatedError()
		for _, e := range val.Consistency {
			validationErrors.Add(fmt.Errorf("%w: %s", bindingConsistencyError, e))
		}
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseError
		r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonInconsistentSpec, validationErrors)
		return ctrl.Result{}, nil
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

	if token.Status.Phase == api.SPIAccessTokenPhaseReady {
		// linking the service account with a secret is a 3-step process.
		// First, an empty service account needs to be created.
		// Second, a secret linking to the service account needs to be created.
		// Third, the service account needs to be updated with the link to the secret.

		// TODO this can leave the SA behind if we subsequently fail to update the status of the binding
		serviceAccount, err := r.ensureServiceAccount(ctx, &binding)
		if err != nil {
			lg.Error(err, "unable to sync the service account")
			return ctrl.Result{}, fmt.Errorf("failed to sync the service account: %w", err)
		}
		if serviceAccount != nil {
			binding.Status.ServiceAccountName = serviceAccount.Name
			lg.Info("service account linked", "saName", serviceAccount.Name, "managed", binding.Spec.ServiceAccount.Managed)
		}

		// TODO this can leave the secret behind if we subsequently fail to update the status of the binding
		secret, err := r.syncSecret(ctx, sp, &binding, serviceAccount, token)
		if err != nil {
			lg.Error(err, "unable to sync the secret")
			return ctrl.Result{}, fmt.Errorf("failed to sync the secret: %w", err)
		}
		binding.Status.SyncedObjectRef = toObjectRef(secret)

		lg.Info("secret synced", "secretName", secret.Name)

		if serviceAccount != nil {
			if err = r.linkSecretToServiceAccount(ctx, binding.Spec.ServiceAccount.EffectiveSecretLinkType(), secret, serviceAccount); err != nil {
				lg.Error(err, "failed to link the secret with the service account")
				return ctrl.Result{}, fmt.Errorf("failed to link the secret with the service account: %w", err)
			}
		}

		binding.Status.Phase = api.SPIAccessTokenBindingPhaseInjected
	} else {
		binding.Status.Phase = api.SPIAccessTokenBindingPhaseAwaitingTokenData
		binding.Status.SyncedObjectRef = api.TargetObjectRef{}
		binding.Status.ServiceAccountName = ""
	}

	if err := r.updateBindingStatusSuccess(ctx, &binding); err != nil {
		lg.Error(err, "unable to update the status")
		return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
	}

	// now that we set up the binding correctly, we need to clean up the potentially dangling secret (that might contain
	// stale data if the data of the token disappeared from the token)
	if binding.Status.Phase != api.SPIAccessTokenBindingPhaseInjected {
		if binding.Spec.ServiceAccount.Managed {
			if err := cleanupDependentObjectsManaged(ctx, r.Client, &binding); err != nil {
				lg.Error(err, "failed to delete the stale synced secret(s) and service account(s)")
				r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
			}
		} else {
			if err := cleanupDependentObjectsUnmanaged(ctx, r.Client, &binding); err != nil {
				lg.Error(err, "failed to delete the stale synced secret(s) and service account(s)")
				r.updateBindingStatusError(ctx, &binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
			}
		}
	}

	if expectedLifetimeDuration == nil {
		lg.V(logs.DebugLevel).Info("binding with unlimited lifetime", "requeueIn", "never")
		// no need to re-schedule by any timeout
		return ctrl.Result{}, nil
	}

	delay := time.Until(binding.CreationTimestamp.Add(*expectedLifetimeDuration).Add(r.Configuration.DeletionGracePeriod))
	lg.V(logs.DebugLevel).Info("binding with limited lifetime", "requeueIn", delay)

	return ctrl.Result{RequeueAfter: delay}, nil
}

// migrateFinalizers checks if the finalizer list containst the obsolete "linked-secrets" finalizer and replaces it with the new "linked-objects".
// It performs the update straight away if there is an obsolete finalizer.
func (r *SPIAccessTokenBindingReconciler) migrateFinalizers(ctx context.Context, binding *api.SPIAccessTokenBinding) error {
	updateNeeded := false
	for i, f := range binding.Finalizers {
		if f == deprecatedLinkedSecretsFinalizerName {
			binding.Finalizers[i] = linkedObjectsFinalizerName
			updateNeeded = true
		}
	}

	if updateNeeded && binding.DeletionTimestamp == nil {
		// only update in cluster if the object is not being deleted. Adding a finalizer while an object is being
		// deleted is disallowed. But that is not a problem because we've already changed that in memory and the Finalizer
		// doesn't reload the object from the cluster.
		if err := r.Client.Update(ctx, binding); err != nil {
			return fmt.Errorf("failed to update the deprecated finalizer names: %w", err)
		}
	}

	return nil
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

func checkQuayPermissionAreasMigration(binding *api.SPIAccessTokenBinding, spName config.ServiceProviderName) bool {
	permissionChange := false
	if spName == config.ServiceProviderTypeQuay.Name {
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
func (r *SPIAccessTokenBindingReconciler) syncSecret(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding, serviceAccount *corev1.ServiceAccount, tokenObject *api.SPIAccessToken) (*corev1.Secret, error) {
	token, err := r.TokenStorage.Get(ctx, tokenObject)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, err)
		return nil, fmt.Errorf("failed to get the token data from token storage: %w", err)
	}

	if token == nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, accessTokenDataNotFoundError)
		return nil, accessTokenDataNotFoundError
	}

	at, err := sp.MapToken(ctx, binding, tokenObject, token)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenAnalysis, err)
		return nil, fmt.Errorf("failed to analyze the token to produce the mapping to the secret: %w", err)
	}

	stringData, err := at.ToSecretType(binding.Spec.Secret.Type, &binding.Spec.Secret.Fields)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenAnalysis, err)
		return nil, fmt.Errorf("failed to create data to be injected into the secret: %w", err)
	}

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

	annos := binding.Spec.Secret.Annotations

	diffOpts := secretDiffOpts

	if binding.Spec.Secret.Type == corev1.SecretTypeServiceAccountToken && serviceAccount != nil {
		diffOpts = serviceAccountSecretDiffOpts

		// we assume the binding has already been validated using its Validate() method so that we don't have
		// to check for corner cases here anymore.
		if annos == nil {
			annos = map[string]string{}
		}
		annos[corev1.ServiceAccountNameKey] = serviceAccount.Name
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
			Annotations: annos,
		},
		Data: data,
		Type: binding.Spec.Secret.Type,
	}

	if secret.Name == "" {
		secret.GenerateName = binding.Name + "-secret-"
	}

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}

	secret.Labels[SPIAccessTokenBindingLinkLabel] = binding.Name

	_, obj, err := r.syncer.Sync(ctx, nil, secret, diffOpts)
	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
		return nil, fmt.Errorf("failed to sync the secret with the token data: %w", err)
	}
	return obj.(*corev1.Secret), nil
}

// ensureServiceAccount loads the service account configured in the binding from the cluster or creates a new one if needed.
// It also makes sure that the service account is correctly labeled.
func (r *SPIAccessTokenBindingReconciler) ensureServiceAccount(ctx context.Context, binding *api.SPIAccessTokenBinding) (*corev1.ServiceAccount, error) {
	spec := binding.Spec.ServiceAccount

	if spec.Name == "" && spec.GenerateName == "" {
		return nil, nil
	}

	name := binding.Status.ServiceAccountName
	if name == "" {
		name = spec.Name
	}

	requestedLabels := spec.Labels
	if requestedLabels == nil {
		requestedLabels = map[string]string{}
	}
	requestedLabels[SPIAccessTokenBindingLinkLabel] = binding.Name

	var err error
	sa := &corev1.ServiceAccount{}
	needsUpdate := false

	if err = r.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: binding.Namespace}, sa); err != nil {
		if errors.IsNotFound(err) {
			sa = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:         name,
					Namespace:    binding.Namespace,
					GenerateName: binding.Spec.ServiceAccount.GenerateName,
					Annotations:  binding.Spec.ServiceAccount.Annotations,
					Labels:       requestedLabels,
				},
			}
			err = r.Client.Create(ctx, sa)
		}
	}

	// make sure all our labels have the values we want
	if err == nil {
		if sa.Labels == nil {
			sa.Labels = map[string]string{}
		}

		for k, rv := range requestedLabels {
			v, ok := sa.Labels[k]
			if !ok || v != rv {
				needsUpdate = true
			}
			sa.Labels[k] = rv
		}

		if len(spec.Annotations) > 0 {
			for k, rv := range spec.Annotations {
				v, ok := sa.Annotations[k]
				if !ok || v != rv {
					needsUpdate = true
				}
				sa.Annotations[k] = rv
			}
		}

		if needsUpdate {
			err = r.Client.Update(ctx, sa)
		}
	}

	if err != nil {
		r.updateBindingStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenSync, err)
		return nil, fmt.Errorf("failed to sync the configured service account of the binding: %w", err)
	}

	return sa, nil
}

// linkSecretToServiceAccount links the secret with the service account according to the type of the secret.
func (r *SPIAccessTokenBindingReconciler) linkSecretToServiceAccount(ctx context.Context, linkType api.SecretLinkType, secret *corev1.Secret, sa *corev1.ServiceAccount) error {
	updated := false

	if linkType == api.SecretLinkTypeSecret {
		hasLink := false

		for _, r := range sa.Secrets {
			if r.Name == secret.Name {
				hasLink = true
				break
			}
		}

		if !hasLink {
			sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: secret.Name})
			updated = true
		}
	} else if linkType == api.SecretLinkTypeImagePullSecret {
		hasLink := false

		for _, r := range sa.ImagePullSecrets {
			if r.Name == secret.Name {
				hasLink = true
			}
		}
		if !hasLink {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: secret.Name})
			updated = true
		}
	}

	if updated {
		if err := r.Client.Update(ctx, sa); err != nil {
			return fmt.Errorf("failed to update the service account with the link to the secret: %w", err)
		}
	}

	return nil
}

// unlinkSecretFromServiceAccount removes the provided secret from any links in the service account (either secrets or image pull secrets fields).
// Returns `true` if the service account object was changed, `false` otherwise. This does not update the object in the cluster!
func unlinkSecretFromServiceAccount(secret *corev1.Secret, serviceAccount *corev1.ServiceAccount) bool {
	updated := false

	if len(serviceAccount.Secrets) > 0 {
		saSecrets := make([]corev1.ObjectReference, 0, len(serviceAccount.Secrets))
		for i := range serviceAccount.Secrets {
			r := serviceAccount.Secrets[i]
			if r.Name == secret.Name {
				updated = true
			} else {
				saSecrets = append(saSecrets, r)
			}
		}
		serviceAccount.Secrets = saSecrets
	}

	if len(serviceAccount.ImagePullSecrets) > 0 {
		saIPSecrets := make([]corev1.LocalObjectReference, 0, len(serviceAccount.ImagePullSecrets))
		for i := range serviceAccount.ImagePullSecrets {
			r := serviceAccount.ImagePullSecrets[i]
			if r.Name == secret.Name {
				updated = true
			} else {
				saIPSecrets = append(saIPSecrets, r)
			}
		}
		serviceAccount.ImagePullSecrets = saIPSecrets
	}

	return updated
}

func deleteDependentSecrets(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) error {
	sl := &corev1.SecretList{}
	if err := cl.List(ctx, sl, client.MatchingLabels{SPIAccessTokenBindingLinkLabel: binding.Name}, client.InNamespace(binding.Namespace)); err != nil {
		return fmt.Errorf("failed to list the secrets(s) associated with the binding %+v while trying to delete them: %w", client.ObjectKeyFromObject(binding), err)
	}

	for i := range sl.Items {
		s := sl.Items[i]
		if err := cl.Delete(ctx, &s); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete the secret %+v while trying to clean up dependent object of binding %+v: %w", client.ObjectKeyFromObject(&s), client.ObjectKeyFromObject(binding), err)
			}
		}
	}

	log.FromContext(ctx).V(logs.DebugLevel).Info("dependent secret(s) deleted", "binding", client.ObjectKeyFromObject(binding))

	return nil
}

func deleteDependentServiceAccounts(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) error {
	sal := &corev1.ServiceAccountList{}
	if err := cl.List(ctx, sal, client.MatchingLabels{SPIAccessTokenBindingLinkLabel: binding.Name}, client.InNamespace(binding.Namespace)); err != nil {
		return fmt.Errorf("failed to list the service account(s) associated with the binding %+v while trying to delete them: %w", client.ObjectKeyFromObject(binding), err)
	}

	for i := range sal.Items {
		sa := sal.Items[i]
		if err := cl.Delete(ctx, &sa); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete the service account %+v while trying to clean up dependent object of binding %+v: %w", client.ObjectKeyFromObject(&sa), client.ObjectKeyFromObject(binding), err)
			}
		}
	}

	log.FromContext(ctx).V(logs.DebugLevel).Info("dependent service account(s) deleted", "binding", client.ObjectKeyFromObject(binding))

	return nil
}

func cleanupDependentObjectsManaged(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) error {
	if err := deleteDependentSecrets(ctx, cl, binding); err != nil {
		return fmt.Errorf("failed to delete the secret(s) associated with the binding %+v: %w", client.ObjectKeyFromObject(binding), err)
	}

	if err := deleteDependentServiceAccounts(ctx, cl, binding); err != nil {
		return fmt.Errorf("failed to delete the service account(s) associated with the binding %+v: %w", client.ObjectKeyFromObject(binding), err)
	}

	return nil
}

func cleanupDependentObjectsUnmanaged(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) error {
	// we need to first unlink the secrets from the service account(s) and then delete the secrets

	sal := &corev1.ServiceAccountList{}
	if err := cl.List(ctx, sal, client.MatchingLabels{SPIAccessTokenBindingLinkLabel: binding.Name}, client.InNamespace(binding.Namespace)); err != nil {
		return fmt.Errorf("failed to list the service accounts associated with the binding %+v: %w", client.ObjectKeyFromObject(binding), err)
	}

	sl := &corev1.SecretList{}
	if err := cl.List(ctx, sl, client.MatchingLabels{SPIAccessTokenBindingLinkLabel: binding.Name}, client.InNamespace(binding.Namespace)); err != nil {
		return fmt.Errorf("failed to list the secrets associated with the binding %+v: %w", client.ObjectKeyFromObject(binding), err)
	}

	for i := range sal.Items {
		sa := sal.Items[i]
		persist := false
		for j := range sl.Items {
			s := sl.Items[j]
			persist = persist || unlinkSecretFromServiceAccount(&s, &sa)
		}
		if persist {
			if err := cl.Update(ctx, &sa); err != nil {
				return fmt.Errorf("failed to remove the linked secrets from the service account %+v while cleaning up dependent objects of binding %+v: %w", client.ObjectKeyFromObject(&sa), client.ObjectKeyFromObject(binding), err)
			}
		}
	}

	if err := deleteDependentSecrets(ctx, cl, binding); err != nil {
		return fmt.Errorf("failed to delete the secret(s) associated with the binding %+v: %w", client.ObjectKeyFromObject(binding), err)
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

type linkedObjectsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedObjectsFinalizer)(nil)

// Finalize removes the secret and possibly also service account synced to the actual binging being deleted
func (f *linkedObjectsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	binding, ok := obj.(*api.SPIAccessTokenBinding)
	if !ok {
		return res, unexpectedObjectTypeError
	}

	lg := log.FromContext(ctx).V(logs.DebugLevel)

	key := client.ObjectKeyFromObject(binding)

	lg.Info("linked objects finalizer starting to clean up dependent objects", "binding", key)
	if binding.Spec.ServiceAccount.Managed {
		if err := cleanupDependentObjectsManaged(ctx, f.client, binding); err != nil {
			lg.Error(err, "failed to clean up managed dependent objects in the finalizer", "binding", key)
			return res, err
		}
	} else {
		if err := cleanupDependentObjectsUnmanaged(ctx, f.client, binding); err != nil {
			lg.Error(err, "failed to clean up unmanaged dependent objects in the finalizer", "binding", key)
			return res, err
		}
	}

	lg.Info("linked objects finalizer completed without failure", "binding", key)

	return res, nil
}
