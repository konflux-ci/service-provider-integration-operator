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
	"io/ioutil"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
)

var (
	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}
)

// AccessTokenSecretReconciler reconciles a AccessTokenSecret object
type AccessTokenSecretReconciler struct {
	config *rest.Config
	client.Client
	Scheme *runtime.Scheme
	syncer sync.Syncer
}

func NewAccessTokenSecretReconciler(config *rest.Config, cl client.Client, scheme *runtime.Scheme) *AccessTokenSecretReconciler {
	return &AccessTokenSecretReconciler{
		config: config,
		Client: cl,
		Scheme: scheme,
		syncer: sync.New(cl, scheme),
	}
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=create;delete;get;list;patch;update;watch

func (r *AccessTokenSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx, "AccessTokenSecret", req.NamespacedName)
	ctx = log.IntoContext(ctx, lg)

	ats := api.AccessTokenSecret{}

	if err := r.Get(ctx, req.NamespacedName, &ats); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if ats.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	cl, err := r.getAuthenticatedClient()
	if err != nil {
		r.updateStatus(ctx, &ats, api.AccessTokenSecretStatus{
			Message: err.Error(),
			Phase:   api.AccessTokenSecretPhaseRetrieving,
			Reason:  api.AccessTokenSecretFailureSPIClientSetup,
		})

		return ctrl.Result{}, err
	}

	token, err := readAccessToken(ctx, cl, &ats)
	if err != nil {
		r.updateStatus(ctx, &ats, api.AccessTokenSecretStatus{
			Message: err.Error(),
			Phase:   api.AccessTokenSecretPhaseRetrieving,
			Reason:  api.AccessTokenSecretFailureTokenRetrieval,
		})

		return ctrl.Result{}, err
	}

	var objectRef api.AccessTokenSecretStatusObjectRef
	if ats.Spec.Target.ConfigMap != nil {
		objectRef, err = r.saveTokenAsConfigMap(ctx, &ats, &token, ats.Spec.Target.ConfigMap)
	} else if ats.Spec.Target.Secret != nil {
		objectRef, err = r.saveTokenAsSecret(ctx, &ats, &token, ats.Spec.Target.Secret)
	} else if ats.Spec.Target.Containers != nil {
		objectRef, err = r.injectTokenIntoPods(ctx, &ats, &token, ats.Spec.Target.Containers)
	} else {
		err = stderrors.New("AccessTokenSecret needs to specify a valid target")
	}

	if err != nil {
		r.updateStatus(ctx, &ats, api.AccessTokenSecretStatus{
			Message: err.Error(),
			Phase:   api.AccessTokenSecretPhaseInjecting,
			Reason:  api.AccessTokenSecretFailureInjection,
		})
	} else {
		r.updateStatus(ctx, &ats, api.AccessTokenSecretStatus{
			Phase:     api.AccessTokenSecretPhaseInjected,
			ObjectRef: objectRef,
		})
	}

	return ctrl.Result{}, err
}

func (r *AccessTokenSecretReconciler) updateStatus(ctx context.Context, obj *api.AccessTokenSecret, status api.AccessTokenSecretStatus) {
	obj.Status = status
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		log.FromContext(ctx).Error(err, "failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessTokenSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.AccessTokenSecret{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
			OwnerType: &api.AccessTokenSecret{},
		}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
			OwnerType: &api.AccessTokenSecret{},
		}).
		// TODO wait with these until we need to have them...
		// Watches(&source.Kind{Type: &appsv1.Deployment{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		// 	// Implement this
		// 	return []reconcile.Request{}
		// })).
		// Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		// 	// Implement this
		// 	return []reconcile.Request{}
		// })).
		Complete(r)
}

func (r *AccessTokenSecretReconciler) getAuthenticatedClient() (*http.Client, error) {
	clientConfig := rest.CopyConfig(r.config)

	// make it possible to run outside of the cluster with some auth token
	if clientConfig.BearerTokenFile == "" {
		f := config.BearerTokenFile()
		// only use the token file if it exists
		if _, err := os.Stat(f); err == nil {
			clientConfig.BearerTokenFile = f
		}
	}

	t, err := rest.TransportFor(clientConfig)
	if err != nil {
		return nil, err
	}

	cl := http.Client{Transport: t}
	return &cl, nil
}

func readAccessToken(ctx context.Context, cl *http.Client, ats *api.AccessTokenSecret) (AccessToken, error) {
	token := AccessToken{}

	resp, err := cl.Get(config.SpiTokenEndpoint() + ats.Spec.AccessTokenName)
	if err != nil {
		return token, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return token, err
	}

	if err = json.Unmarshal(data, &token); err != nil {
		return token, err
	}

	return token, nil
}

func (r *AccessTokenSecretReconciler) saveTokenAsConfigMap(ctx context.Context, owner *api.AccessTokenSecret, token *AccessToken, spec *api.AccessTokenTargetConfigMap) (api.AccessTokenSecretStatusObjectRef, error) {
	data := map[string]string{}
	token.fillByMapping(&spec.Fields, data)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        spec.Name,
			Namespace:   owner.GetNamespace(),
			Labels:      spec.Labels,
			Annotations: spec.Annotations,
		},
		Data: data,
	}

	_, obj, err := r.syncer.Sync(ctx, owner, cm, secretDiffOpts)
	return toObjectRef(obj), err
}

func (r *AccessTokenSecretReconciler) saveTokenAsSecret(ctx context.Context, owner *api.AccessTokenSecret, token *AccessToken, spec *api.AccessTokenTargetSecret) (api.AccessTokenSecretStatusObjectRef, error) {
	stringData := token.toSecretType(spec.Type)
	token.fillByMapping(&spec.Fields, stringData)

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
			Name:        spec.Name,
			Namespace:   owner.GetNamespace(),
			Labels:      spec.Labels,
			Annotations: spec.Annotations,
		},
		Data: data,
		Type: spec.Type,
	}

	_, obj, err := r.syncer.Sync(ctx, owner, secret, secretDiffOpts)
	return toObjectRef(obj), err
}

func (r *AccessTokenSecretReconciler) injectTokenIntoPods(ctx context.Context, owner *api.AccessTokenSecret, token *AccessToken, spec *api.AccessTokenTargetContainers) (api.AccessTokenSecretStatusObjectRef, error) {
	// TODO implement
	return api.AccessTokenSecretStatusObjectRef{}, stderrors.New("injection into pods not implemented")
}

func toObjectRef(obj client.Object) api.AccessTokenSecretStatusObjectRef {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return api.AccessTokenSecretStatusObjectRef{
		Name:       obj.GetName(),
		Kind:       kind,
		ApiVersion: apiVersion,
	}
}
