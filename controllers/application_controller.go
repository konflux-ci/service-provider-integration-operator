//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	stderrors "errors"
	"fmt"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	rapi "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ApplicationReconciler struct {
	k8sClient  client.Client
	finalizers finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=list;update;watch;delete

var unableToDeleteRemoteSecret = stderrors.New("unable to delete the remote secret with application removal")

func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedRemoteSecretsTargetFinalizerName, &linkedRemoteSecretTargetsFinalizer{client: r.k8sClient}); err != nil {
		return fmt.Errorf("failed to register the linked remote secret finalizer: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Application{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to configure the application reconciler: %w", err)
	}
	return nil
}

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")

	var application appstudiov1alpha1.Application
	if err := r.k8sClient.Get(ctx, req.NamespacedName, &application); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("Application already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get the Application: %w", err)
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &application)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.k8sClient.Update(ctx, &application); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update application based on finalization result: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

type linkedRemoteSecretTargetsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedRemoteSecretTargetsFinalizer)(nil)

// Finalize removes the remote secret targets synced to the actual application which is being deleted
func (f *linkedRemoteSecretTargetsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	application, ok := obj.(*appstudiov1alpha1.Application)
	if !ok {
		return finalizer.Result{}, unexpectedObjectTypeError
	}

	remoteSecretsList := rapi.RemoteSecretList{}
	if err := f.client.List(ctx, &remoteSecretsList, client.InNamespace(application.Namespace)); err != nil {
		return finalizer.Result{}, unableToFetchRemoteSecretsError
	}

	if len(remoteSecretsList.Items) == 0 {
		return finalizer.Result{}, nil
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		applicationInSecret, _ := remoteSecret.Labels[ApplicationLabelName]
		if applicationInSecret != application.Name {
			// this secret is intended for another application, bypassing it
			continue
		}
		// remove the secret
		if err := f.client.Delete(ctx, &remoteSecret); err != nil {
			return finalizer.Result{}, unableToDeleteRemoteSecret
		}
	}
	return finalizer.Result{}, nil
}
