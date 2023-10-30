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
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/redhat-appstudio/remote-secret/pkg/commaseparated"
	"k8s.io/utils/strings/slices"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	rapi "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	linkedRemoteSecretsTargetFinalizerName = "spi.appstudio.redhat.com/remote-secrets" //#nosec G101 -- false positive, just label name
)

// completely ignore secrets with this label as they're not meant to be used by the application directly
var (
	ignoredSecretsLabelName   = "ui.appstudio.redhat.com/secret-for" //nolint:gosec // false positive, just label name
	ignoredSecretsLabelValues = []string{"Build"}
)

type EnvironmentReconciler struct {
	k8sClient  client.Client
	finalizers finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=list;update;watch

func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedRemoteSecretsTargetFinalizerName, &linkedEnvRemoteSecretsFinalizer{client: r.k8sClient}); err != nil {
		return fmt.Errorf("failed to register the linked remote secret finalizer: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Environment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to configure the application reconciler: %w", err)
	}
	return nil
}

func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err := r.k8sClient.Get(ctx, req.NamespacedName, &environment)
	if err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("Environment already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}
		lg.Error(err, "unable to get the Environment", "name", req.Name, "namespace", req.NamespacedName)
		return ctrl.Result{}, fmt.Errorf("failed to load environment from cluster: %w", err)
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &environment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.k8sClient.Update(ctx, &environment); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update environment based on finalization result: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

type linkedEnvRemoteSecretsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedEnvRemoteSecretsFinalizer)(nil)

// Finalize removes the remote secret targets synced to the actual environment which is being deleted
func (f *linkedEnvRemoteSecretsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	environment, ok := obj.(*appstudiov1alpha1.Environment)
	if !ok {
		return finalizer.Result{}, unexpectedObjectTypeError
	}

	buildReq, _ := labels.NewRequirement(ignoredSecretsLabelName, selection.NotIn, ignoredSecretsLabelValues)
	selector := labels.NewSelector().Add(*buildReq)

	remoteSecretsList := rapi.RemoteSecretList{}
	if err := f.client.List(ctx, &remoteSecretsList, client.InNamespace(environment.Namespace), &client.ListOptions{LabelSelector: selector}); err != nil {
		return finalizer.Result{}, unableToFetchRemoteSecretsError
	}

	if len(remoteSecretsList.Items) == 0 {
		return finalizer.Result{}, nil
	}

	target, err := detectTargetFromEnvironment(ctx, *environment)
	if err != nil {
		return finalizer.Result{}, fmt.Errorf("failed to detect target from environment: %w", err)
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		if !applicableForEnvironment(remoteSecret, environment.Name) {
			// this secret is intended for another environment, bypassing it
			continue
		}
		logs.AuditLog(ctx).Info("Cleaning up the remote secret targets due to environment deletion", "environment", environment.Name, "remoteSecret", remoteSecret.Name)
		removeTarget(&remoteSecret, target)
		if err := f.client.Update(ctx, &remoteSecret); err != nil {
			return finalizer.Result{}, unableToUpdateRemoteSecret
		}
	}
	return finalizer.Result{}, nil
}

func removeTarget(secret *rapi.RemoteSecret, target rapi.RemoteSecretTarget) {
	tmp := make([]rapi.RemoteSecretTarget, 0, len(secret.Spec.Targets))
	for _, existingTarget := range secret.Spec.Targets {
		if !targetsMatch(existingTarget, target) {
			tmp = append(tmp, existingTarget)
		}
	}
	secret.Spec.Targets = tmp
}

// for cleanup, we check the environment name both in annotations and label
func applicableForEnvironment(remoteSecret rapi.RemoteSecret, environmentName string) bool {
	if annotationValue, annSet := remoteSecret.Annotations[EnvironmentLabelAndAnnotationName]; annSet {
		if slices.Contains(commaseparated.Value(annotationValue).Values(), environmentName) {
			return true
		}
	}
	if labelValue, labSet := remoteSecret.Labels[EnvironmentLabelAndAnnotationName]; labSet {
		return labelValue == environmentName
	}
	return false
}
