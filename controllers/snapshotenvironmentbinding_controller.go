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
	"github.com/redhat-appstudio/remote-secret/pkg/commaseparated"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ApplicationLabelName              = "appstudio.redhat.com/application"
	EnvironmentLabelAndAnnotationName = "appstudio.redhat.com/environment"
	ComponentLabelName                = "appstudio.redhat.com/component"
)

var (
	unableToFetchRemoteSecretsError = stderrors.New("unable to fetch the Remote Secrets list")
	unableToUpdateRemoteSecret      = stderrors.New("unable to update the remote secret")
)

// SnapshotEnvironmentBindingReconciler reconciles a SnapshotEnvironmentBinding object
type SnapshotEnvironmentBindingReconciler struct {
	k8sClient     client.Client
	Scheme        *runtime.Scheme
	Configuration *opconfig.OperatorConfiguration
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/status,verbs=get
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=list;update;watch

func (r *SnapshotEnvironmentBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	// remove old finalizers
	if slices.Contains(snapshotEnvBinding.Finalizers, linkedRemoteSecretsTargetFinalizerName) {
		controllerutil.RemoveFinalizer(&snapshotEnvBinding, linkedRemoteSecretsTargetFinalizerName)
		if err := r.k8sClient.Update(ctx, &snapshotEnvBinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to cleanup old finalizer on snapshot env binding: %w", err)
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
	err := r.k8sClient.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: snapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("Environment CR not found. skipping reconciliation")
			return ctrl.Result{}, nil
		}
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

	for idx := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[idx]
		if !remoteSecretApplicableForEnvironment(remoteSecret, environmentName) {
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

func targetsMatch(target1, target2 rapi.RemoteSecretTarget) bool {
	return target1.Namespace == target2.Namespace && target1.ApiUrl == target2.ApiUrl && target1.ClusterCredentialsSecret == target2.ClusterCredentialsSecret
}

func detectTargetFromEnvironment(_ context.Context, environment appstudiov1alpha1.Environment) (rapi.RemoteSecretTarget, error) {
	if slices.Contains(environment.Spec.Tags, "managed") && environment.Spec.UnstableConfigurationFields != nil {
		return rapi.RemoteSecretTarget{
			Namespace:                environment.Spec.UnstableConfigurationFields.TargetNamespace,
			ApiUrl:                   environment.Spec.UnstableConfigurationFields.APIURL,
			ClusterCredentialsSecret: environment.Spec.UnstableConfigurationFields.ClusterCredentialsSecret,
		}, nil
	} else {
		// local environment, just return the namespace
		return rapi.RemoteSecretTarget{Namespace: environment.Namespace}, nil
	}
}

func remoteSecretApplicableForEnvironment(remoteSecret rapi.RemoteSecret, environmentName string) bool {
	if annotationValue, annSet := remoteSecret.Annotations[EnvironmentLabelAndAnnotationName]; annSet {
		return slices.Contains(commaseparated.Value(annotationValue).Values(), environmentName)
	}
	if labelValue, labSet := remoteSecret.Labels[EnvironmentLabelAndAnnotationName]; labSet {
		return labelValue == environmentName
	}
	return true
}
