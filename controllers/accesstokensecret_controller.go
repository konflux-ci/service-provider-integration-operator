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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
)

var (
	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}
)

// AccessTokenSecretReconciler reconciles a AccessTokenSecret object
type AccessTokenSecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	syncer sync.Syncer
}

func NewAccessTokenSecretReconciler(cl client.Client, scheme *runtime.Scheme) *AccessTokenSecretReconciler {
	return &AccessTokenSecretReconciler{
		Client: cl,
		Scheme: scheme,
		syncer: sync.New(cl, scheme),
	}
}

// TODO define this properly once we know more
type accessToken map[string]string

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=accesstokensecrets/finalizers,verbs=update
//+kubebuilder:rbac:groups=,resources=serviceaccounts,verbs=impersonate

func (r *AccessTokenSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx, "AccessTokenSecret", req.NamespacedName)

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

	cl, err := r.getImpersonatedClient(log.IntoContext(ctx, lg), &ats)
	if err != nil {
		// TODO update the status with the failure
		return ctrl.Result{}, err
	}

	token, err := readAccessToken(log.IntoContext(ctx, lg.WithValues("ImpersonatedAs", ats.Spec.ServiceAccount)), &cl, ats.Spec.AccessTokenId)
	if err != nil {
		// TODO update the status with the failure
		return ctrl.Result{}, err
	}

	err = nil
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
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			// TODO look for some label for example
			return []reconcile.Request{}
		})).
		Complete(r)
}

func (r *AccessTokenSecretReconciler) getImpersonatedClient(ctx context.Context, acs *api.AccessTokenSecret) (client.Client, error) {
	lg := log.FromContext(ctx)

	sa := acs.Spec.ServiceAccount
	if sa.Name == "" {
		return r.Client, nil
	}

	if sa.Namespace == "" {
		sa.Namespace = acs.GetNamespace()
	}

	clientConfig, err := ctrl.GetConfig()
	if err != nil {
		lg.Error(err, "Failed to get the configuration for talking to the Kubernetes API server")
	}

	clientConfig.Impersonate = rest.ImpersonationConfig{
		UserName: "system:serviceaccount:" + sa.Namespace + ":" + sa.Name,
	}

	return client.New(clientConfig, client.Options{
		Scheme: r.Scheme,
		Mapper: r.RESTMapper(),
	})
}

func readAccessToken(ctx context.Context, cl *client.Client, tokenId string) (accessToken, error) {
	// TODO implement
	return accessToken{}, nil
}

func (r *AccessTokenSecretReconciler) saveTokenAsConfigMap(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetConfigMap) error {
	// TODO implement
	return stderrors.New("saving accesstokens to config map not implemented")
}

func (r *AccessTokenSecretReconciler) saveTokenAsSecret(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetSecret) error {
	data := map[string][]byte{}
	for k, v := range *token {
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
	}

	_, _, err := r.syncer.Sync(ctx, owner, secret, secretDiffOpts)
	return err
}

func (r *AccessTokenSecretReconciler) injectTokenIntoPods(ctx context.Context, owner *api.AccessTokenSecret, token *accessToken, spec *api.AccessTokenTargetContainers) error {
	// TODO implement
	return stderrors.New("injection into pods not implemented")
}
