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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/storage"

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
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

var spiAccessTokenBindingLog = log.Log.WithName("spiaccesstokenbinding-controller")

var (
	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}
)

// SPIAccessTokenBindingReconciler reconciles a SPIAccessTokenBinding object
type SPIAccessTokenBindingReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	Storage                *storage.Storage
	syncer                 sync.Syncer
	ServiceProviderFactory serviceprovider.Factory
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;watch;create;list

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.syncer = sync.New(mgr.GetClient(), mgr.GetScheme())
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessTokenBinding{}).
		Owns(&corev1.Secret{}).
		Watches(&source.Kind{Type: &api.SPIAccessToken{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			bindings := &api.SPIAccessTokenList{}
			if err := r.Client.List(context.TODO(), bindings, client.InNamespace(o.GetNamespace()), client.MatchingLabels{
				config.SPIAccessTokenLinkLabel: o.GetName(),
			}); err != nil {
				spiAccessTokenBindingLog.Error(err, "failed to list SPIAccessTokenBindings while determining the ones linked to SPIAccessToken",
					"SPIAccessTokenName", o.GetName(), "SPIAccessTokenNamespace", o.GetNamespace())
				return []reconcile.Request{}
			}
			ret := make([]reconcile.Request, len(bindings.Items))
			for _, b := range bindings.Items {
				ret = append(ret, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      b.Name,
						Namespace: b.Namespace,
					},
				})
			}
			return ret
		})).
		Complete(r)
}

func (r *SPIAccessTokenBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx, "SPIAccessTokenBinding", req.NamespacedName)
	ctx = log.IntoContext(ctx, lg)

	binding := api.SPIAccessTokenBinding{}

	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, NewReconcileError(err, "failed to read the object")
	}

	if binding.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	sp, rerr := r.getServiceProvider(ctx, &binding)
	if rerr != nil {
		return ctrl.Result{}, rerr
	}

	token, err := r.linkToken(ctx, sp, &binding)
	if err != nil {
		return ctrl.Result{}, NewReconcileError(err, "failed to link the token")
	}

	if token.Status.Phase == api.SPIAccessTokenPhaseReady {
		ref, err := r.syncSecret(ctx, sp, &binding, token)
		if err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to sync the secret")
		}
		binding.Status.SyncedObjectRef = ref
		if err := r.updateStatusSuccess(ctx, &binding); err != nil {
			return ctrl.Result{}, NewReconcileError(err, "failed to update the status")
		}
	}

	return ctrl.Result{}, nil
}

func (r *SPIAccessTokenBindingReconciler) getServiceProvider(ctx context.Context, binding *api.SPIAccessTokenBinding) (serviceprovider.ServiceProvider, *ReconcileError) {
	serviceProvider, err := r.ServiceProviderFactory.FromRepoUrl(binding.Spec.RepoUrl)
	if err != nil {
		r.updateStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
		return nil, NewReconcileError(err, "failed to find the service provider")
	}

	return serviceProvider, nil
}

func (r *SPIAccessTokenBindingReconciler) linkToken(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	token, err := sp.LookupToken(ctx, r.Client, binding)
	if err != nil {
		r.updateStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonTokenLookup, err)
		return nil, NewReconcileError(err, "failed to lookup the token in the service provider")
	}

	if token == nil {
		log.FromContext(ctx).Info("creating a new token because none found for binding")

		serviceProviderUrl := sp.GetBaseUrl()
		if err != nil {
			r.updateStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType, err)
			return nil, NewReconcileError(err, "failed to determine the service provider URL from the repo")
		}

		// create the token (and let its webhook and controller finish the setup)
		token = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "generated-spi-access-token-",
				Namespace:    binding.Namespace,
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: sp.GetType(),
				Permissions:         binding.Spec.Permissions,
				ServiceProviderUrl:  serviceProviderUrl,
			},
		}

		if err := r.Client.Create(ctx, token); err != nil {
			r.updateStatusError(ctx, binding, api.SPIAccessTokenBindingErrorReasonLinkedToken, err)
			return nil, NewReconcileError(err, "failed to create the token")
		}
	}

	// we need to have this label so that updates to the linked SPIAccessToken are reflected here, too... We're setting
	// up the watch to use the label to limit the scope...
	if binding.Labels[config.SPIAccessTokenLinkLabel] != token.Name {
		if binding.Labels == nil {
			binding.Labels = map[string]string{}
		}
		binding.Labels[config.SPIAccessTokenLinkLabel] = token.Name

		if err := r.Client.Update(ctx, binding); err != nil {
			return token, NewReconcileError(err, "failed to update the binding with the token link")
		}
	}

	if binding.Status.LinkedAccessTokenName != token.Name {
		binding.Status.LinkedAccessTokenName = token.Name
		if err := r.updateStatusSuccess(ctx, binding); err != nil {
			return token, err
		}
	}

	return token, nil
}

func (r *SPIAccessTokenBindingReconciler) updateStatusError(ctx context.Context, binding *api.SPIAccessTokenBinding, reason api.SPIAccessTokenBindingErrorReason, err error) {
	binding.Status.ErrorMessage = err.Error()
	binding.Status.ErrorReason = reason
	if err := r.Client.Status().Update(ctx, binding); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status with error", "reason", reason, "error", err)
	}
}

func (r *SPIAccessTokenBindingReconciler) updateStatusSuccess(ctx context.Context, binding *api.SPIAccessTokenBinding) error {
	binding.Status.ErrorMessage = ""
	binding.Status.ErrorReason = ""
	if err := r.Client.Status().Update(ctx, binding); err != nil {
		log.FromContext(ctx).Error(err, "failed to update status")
		return err
	}
	return nil
}

func (r *SPIAccessTokenBindingReconciler) syncSecret(ctx context.Context, sp serviceprovider.ServiceProvider, binding *api.SPIAccessTokenBinding, tokenObject *api.SPIAccessToken) (api.TargetObjectRef, error) {
	token, err := r.Storage.Get(tokenObject)
	if err != nil {
		return api.TargetObjectRef{}, err
	}

	if token == nil {
		return api.TargetObjectRef{}, fmt.Errorf("access token data not found")
	}

	at := AccessTokenMapper{
		Name:                    tokenObject.Name,
		Token:                   token.AccessToken,
		ServiceProviderUrl:      tokenObject.Spec.ServiceProviderUrl,
		ServiceProviderUserName: tokenObject.Spec.TokenMetadata.UserName,
		ServiceProviderUserId:   tokenObject.Spec.TokenMetadata.UserId,
		UserId:                  "",
		ExpiredAfter:            &token.Expiry,
		Scopes:                  serviceprovider.GetAllScopes(sp, &binding.Spec.Permissions),
	}

	stringData := at.toSecretType(binding.Spec.Secret.Type)
	at.fillByMapping(&binding.Spec.Secret.Fields, stringData)

	// copy the string data into the byte-array data so that sync works reliably. If we didn't sync, we could have just
	// used the Secret.StringData, but Sync gives us other goodies.
	// So let's bite the bullet and convert manually here.
	data := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		data[k] = []byte(v)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        binding.Spec.Secret.Name,
			Namespace:   binding.GetNamespace(),
			Labels:      binding.Spec.Secret.Labels,
			Annotations: binding.Spec.Secret.Annotations,
		},
		Data: data,
		Type: binding.Spec.Secret.Type,
	}

	_, obj, err := r.syncer.Sync(ctx, binding, secret, secretDiffOpts)
	return toObjectRef(obj), err
}

func toObjectRef(obj client.Object) api.TargetObjectRef {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return api.TargetObjectRef{
		Name:       obj.GetName(),
		Kind:       kind,
		ApiVersion: apiVersion,
	}
}
