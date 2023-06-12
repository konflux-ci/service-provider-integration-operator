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
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ApplicationLabelName                   = "appstudio.application"
	EnvironmentLabelName                   = "appstudio.environment"
	ComponentLabelName                     = "appstudio.component"
	linkedRemoteSecretsTargetFinalizerName = "spi.appstudio.redhat.com/remote-secrets" //#nosec G101 -- false positive, just label name
)

var (
	unableToFetchRemoteSecretsError = stderrors.New("unable to fetch the Remote Secrets list")
	unableToFetchEnvironmentError   = stderrors.New("unable to fetch environment")
	unableToUpdateRemoteSecret      = stderrors.New("unable to update the remote secret")
)

// SnapshotEnvironmentBindingReconciler reconciles a SnapshotEnvironmentBinding object
type SnapshotEnvironmentBindingReconciler struct {
	k8sClient     client.Client
	Scheme        *runtime.Scheme
	Configuration *opconfig.OperatorConfiguration
	finalizers    finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments,verbs=get;list;watch

func (r *SnapshotEnvironmentBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedRemoteSecretsTargetFinalizerName, &linkedRemoteSecretsFinalizer{client: r.k8sClient}); err != nil {
		return fmt.Errorf("failed to register the linked remote secret finalizer: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.SnapshotEnvironmentBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)

	if err != nil {
		return fmt.Errorf("failed to configure the reconciler: %w", err)
	}
	return nil
}

func (r *SnapshotEnvironmentBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")

	var snapshotEnvBinding appstudiov1alpha1.SnapshotEnvironmentBinding
	if err := r.k8sClient.Get(ctx, req.NamespacedName, &snapshotEnvBinding); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("SnapshotEnvironmentBinding already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the SnapshotEnvironmentBinding: %w", err)
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &snapshotEnvBinding)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.k8sClient.Update(ctx, &snapshotEnvBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update snapshot env binding based on finalization result: %w", err)
		}
	}

	if snapshotEnvBinding.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("SnapshotEnvironmentBinding is being deleted. skipping reconciliation")
		return ctrl.Result{}, nil
	}

	appNamespace := snapshotEnvBinding.Namespace
	environmentName := snapshotEnvBinding.Spec.Environment
	applicationName := snapshotEnvBinding.Spec.Application

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err = r.k8sClient.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: snapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		lg.Error(err, fmt.Sprintf("unable to get the Environment %s %v", environmentName, req.NamespacedName))
		return ctrl.Result{}, fmt.Errorf("failed to load environment from cluster: %w", err)
	}

	target, err := detectTargetFromEnvironment(ctx, environment)
	if err != nil {
		lg.Error(err, fmt.Sprintf("error while resolving target for environment %s", environmentName))
		return ctrl.Result{}, fmt.Errorf("error resolving targer for environment %s: %w", environmentName, err)
	}

	remoteSecretsList := rapi.RemoteSecretList{}
	if err := r.k8sClient.List(ctx, &remoteSecretsList, client.InNamespace(appNamespace), client.MatchingLabels{ApplicationLabelName: applicationName}); err != nil {
		lg.Error(err, "Unable to fetch remote secrets list", "namespace", appNamespace)
		return ctrl.Result{}, unableToFetchRemoteSecretsError
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		environmentInSecret, set := remoteSecret.Labels[EnvironmentLabelName]
		if set && environmentInSecret != "" && environmentInSecret != environmentName {
			// this secret is intended for another environment, bypassing it
			continue
		}
		addTargetIfNotExists(&remoteSecret, target)
		if err = r.k8sClient.Update(ctx, &remoteSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update secret: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func addTargetIfNotExists(secret *rapi.RemoteSecret, target rapi.RemoteSecretTarget) {
	for idx := range secret.Spec.Targets {
		existingTarget := secret.Spec.Targets[idx]
		if targetsMatch(existingTarget, target) {
			return
		}
	}
	secret.Spec.Targets = append(secret.Spec.Targets, target)
}

func removeTarget(secret *rapi.RemoteSecret, target rapi.RemoteSecretTarget) {
	for idx := range secret.Spec.Targets {
		existingTarget := secret.Spec.Targets[idx]
		if targetsMatch(existingTarget, target) {
			secret.Spec.Targets = append(secret.Spec.Targets[:idx], secret.Spec.Targets[idx+1:]...)
		}
	}
}

func targetsMatch(target1, target2 rapi.RemoteSecretTarget) bool {
	return target1.Namespace == target2.Namespace && target1.ApiUrl == target2.ApiUrl && target1.ClusterCredentialsSecret == target2.ClusterCredentialsSecret
}

func detectTargetFromEnvironment(ctx context.Context, environment appstudiov1alpha1.Environment) (rapi.RemoteSecretTarget, error) {
	// Dummy load. Real impl should reverse pass Env -> DTC -> DT -> SpaceRequest to find out full target details
	return rapi.RemoteSecretTarget{Namespace: environment.Namespace}, nil
}

type linkedRemoteSecretsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedRemoteSecretsFinalizer)(nil)

// Finalize removes the remote secret targets synced to the actual snapshot environment binding which is being deleted
func (f *linkedRemoteSecretsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	snapshotEnvBinding, ok := obj.(*appstudiov1alpha1.SnapshotEnvironmentBinding)
	if !ok {
		return res, unexpectedObjectTypeError
	}

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err := f.client.Get(ctx, types.NamespacedName{Name: snapshotEnvBinding.Spec.Environment, Namespace: snapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		return res, unableToFetchEnvironmentError
	}

	target, err := detectTargetFromEnvironment(ctx, environment)
	if err != nil {
		return res, fmt.Errorf("error resolving targer for environment %s: %w", snapshotEnvBinding.Spec.Environment, err)
	}

	remoteSecretsList := rapi.RemoteSecretList{}
	if err := f.client.List(ctx, &remoteSecretsList, client.InNamespace(snapshotEnvBinding.Namespace), client.MatchingLabels{ApplicationLabelName: snapshotEnvBinding.Spec.Application}); err != nil {
		return res, unableToFetchRemoteSecretsError
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		environmentInSecret, set := remoteSecret.Labels[EnvironmentLabelName]
		if set && environmentInSecret != "" && environmentInSecret != snapshotEnvBinding.Spec.Environment {
			// this secret is intended for another environment, bypassing it
			continue
		}
		removeTarget(&remoteSecret, target)
		if err = f.client.Update(ctx, &remoteSecret); err != nil {
			return res, unableToUpdateRemoteSecret
		}
	}
	return finalizer.Result{}, nil
}
