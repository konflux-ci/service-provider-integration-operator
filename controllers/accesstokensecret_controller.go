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

	tokenEndpoint = config.SpiUrl() + "/api/v1/token/"
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

// This must reflect the data returned from the SPI REST API
type accessToken struct {
	Name  string `json:"name"`
	Token string `json:"token"`
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
		// TODO update the status with the failure
		return ctrl.Result{}, err
	}

	token, err := readAccessToken(ctx, cl, &ats)
	if err != nil {
		// TODO update the status with the failure
		return ctrl.Result{}, err
	}

	if ats.Spec.Target.ConfigMap != nil {
		err = r.saveTokenAsConfigMap(ctx, &ats, &token, ats.Spec.Target.ConfigMap)
	} else if ats.Spec.Target.Secret != nil {
		err = r.saveTokenAsSecret(ctx, &ats, &token, ats.Spec.Target.Secret)
	} else if ats.Spec.Target.Containers != nil {
		err = r.injectTokenIntoPods(ctx, &ats, &token, ats.Spec.Target.Containers)
	} else {
		return ctrl.Result{}, stderrors.New("AccessTokenSecret needs to specify a valid target")
	}

	// TODO update the status with the potential failure

	return ctrl.Result{}, err
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

func readAccessToken(ctx context.Context, cl *http.Client, ats *api.AccessTokenSecret) (accessToken, error) {
	token := accessToken{}

	resp, err := cl.Get(tokenEndpoint + ats.Spec.AccessTokenName)
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

func (r *AccessTokenSecretReconciler) saveTokenAsConfigMap(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetConfigMap) error {
	data := map[string]string{}
	data[spec.AccessTokenKey] = token.Token

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

	_, _, err := r.syncer.Sync(ctx, owner, cm, secretDiffOpts)
	return err
}

func (r *AccessTokenSecretReconciler) saveTokenAsSecret(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetSecret) error {
	data := map[string][]byte{}
	data[spec.AccessTokenKey] = []byte(token.Token)

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
	}

	_, _, err := r.syncer.Sync(ctx, owner, secret, secretDiffOpts)
	return err
}

func (r *AccessTokenSecretReconciler) injectTokenIntoPods(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetContainers) error {
	// TODO implement
	return stderrors.New("injection into pods not implemented")
}
