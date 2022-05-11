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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

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
	Configuration          config.Configuration
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesschecks/finalizers,verbs=update

func (r *SPIAccessCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	ac := api.SPIAccessCheck{}
	if err := r.Get(ctx, req.NamespacedName, &ac); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("SPIAccessCheck not found on cluster")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, NewReconcileError(err, "failed to load the SPIAccessCheck from the cluster")
	}

	if time.Now().After(ac.ObjectMeta.CreationTimestamp.Add(r.Configuration.AccessCheckTtl)) {
		lg.Info("SPIAccessCheck is after ttl, deleting ...")
		if deleteError := r.Delete(ctx, &ac); deleteError != nil {
			return ctrl.Result{Requeue: true}, deleteError
		} else {
			lg.Info("SPIAccessCheck deleted")
			return ctrl.Result{}, nil
		}
	}

	if sp, spErr := r.ServiceProviderFactory.FromRepoUrl(ac.Spec.RepoUrl); spErr == nil {
		if status, repoCheckErr := sp.CheckRepositoryAccess(ctx, r.Client, &ac); repoCheckErr == nil {
			ac.Status = *status
		} else {
			lg.Error(repoCheckErr, "failed to check repository access")
			return ctrl.Result{}, repoCheckErr
		}
	} else {
		lg.Error(spErr, "failed to determine service provider for SPIAccessCheck")
		ac.Status.ErrorReason = api.SPIAccessCheckErrorUnknownServiceProvider
		ac.Status.ErrorMessage = spErr.Error()
	}

	if updateErr := r.Client.Status().Update(ctx, &ac); updateErr != nil {
		lg.Error(updateErr, "Failed to update status")
		return ctrl.Result{}, updateErr
	} else {
		return ctrl.Result{RequeueAfter: r.Configuration.AccessCheckTtl}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessCheck{}).
		Complete(r)
}
