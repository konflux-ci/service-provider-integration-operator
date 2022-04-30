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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// SPIAccessCheckReconciler reconciles a SPIAccessCheck object
type SPIAccessCheckReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	ServiceProviderFactory serviceprovider.Factory
}

const ttlMin = 1

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks/finalizers,verbs=update

func (r *SPIAccessCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	at := api.SPIAccessCheck{}
	if err := r.Get(ctx, req.NamespacedName, &at); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("SPIAccessCheck not found on cluster",
				"namespace:name", fmt.Sprintf("%s:%s", req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, NewReconcileError(err, "failed to load the SPIAccessCheck from the cluster")
	}

	if at.Status.Ttl != 0 {
		if time.Now().After(time.Unix(at.Status.Ttl, 0)) {
			lg.Info("SPIAccessCheck is after ttl, deleting ...", "check", at.Name)
			if deleteError := r.Delete(ctx, &at); deleteError != nil {
				return ctrl.Result{Requeue: true}, deleteError
			} else {
				lg.Info("SPIAccessCheck deleted")
				return ctrl.Result{}, nil
			}
		} else if at.Spec.RepoUrl == at.Status.RepoURL {
			lg.Info("already analyzed, nothing to do", "check", at)
			return ctrl.Result{}, nil
		}
	}

	sp, spErr := r.ServiceProviderFactory.FromRepoUrl(at.Spec.RepoUrl)
	if spErr != nil {
		return ctrl.Result{}, spErr
	}

	lg.Info(fmt.Sprintf("'%s' is '%s'", at.Spec.RepoUrl, sp.GetType()))

	status := sp.CheckRepositoryAccess(ctx, r.Client, &at)
	at.Status = *status
	at.Status.Ttl = time.Now().Add(ttlMin * time.Minute).Unix()

	if updateErr := r.Client.Status().Update(ctx, &at); updateErr != nil {
		lg.Error(updateErr, "Failed to update status")
		return ctrl.Result{}, updateErr
	} else {
		return ctrl.Result{RequeueAfter: (ttlMin + 1) * time.Minute}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessCheck{}).
		Complete(r)
}
