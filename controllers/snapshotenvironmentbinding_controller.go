package controllers

import (
	"context"
	stderrors "errors"
	"fmt"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
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
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
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

	remoteSecretsList := api.RemoteSecretList{}
	if err := r.k8sClient.List(ctx, &remoteSecretsList, client.InNamespace(appNamespace), client.MatchingLabels{ApplicationLabelName: applicationName, EnvironmentLabelName: environmentName}); err != nil {
		spiFileContentRequestLog.Error(err, "Unable to fetch remote secrets list", "namespace", appNamespace)
		return ctrl.Result{}, unableToFetchRemoteSecretsError
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		addTargetIfNotExists(&remoteSecret, target)
		if err = r.k8sClient.Update(ctx, &remoteSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update secret: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func addTargetIfNotExists(secret *api.RemoteSecret, target api.RemoteSecretTarget) {
	for idx := range secret.Spec.Targets {
		existingTarget := secret.Spec.Targets[idx]
		// Add the rest of the fields there
		if existingTarget.Namespace == target.Namespace {
			return
		}
	}
	secret.Spec.Targets = append(secret.Spec.Targets, target)
}

func removeTarget(secret *api.RemoteSecret, target api.RemoteSecretTarget) {
	for idx := range secret.Spec.Targets {
		existingTarget := secret.Spec.Targets[idx]
		// Add the rest of the fields there
		if existingTarget.Namespace == target.Namespace {
			secret.Spec.Targets = append(secret.Spec.Targets[:idx], secret.Spec.Targets[idx+1:]...)
		}
	}
}

func detectTargetFromEnvironment(ctx context.Context, environment appstudiov1alpha1.Environment) (api.RemoteSecretTarget, error) {
	// Dummy load. Real impl should reverse pass Env -> DTC -> DT -> SpaceRequest to find out full target details
	return api.RemoteSecretTarget{Namespace: environment.Namespace}, nil
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
		return res, unableToFetchRemoteSecretsError
	}

	target, err := detectTargetFromEnvironment(ctx, environment)
	if err != nil {
		return res, fmt.Errorf("error resolving targer for environment %s: %w", snapshotEnvBinding.Spec.Environment, err)
	}

	remoteSecretsList := api.RemoteSecretList{}
	if err := f.client.List(ctx, &remoteSecretsList, client.InNamespace(snapshotEnvBinding.Namespace), client.MatchingLabels{ApplicationLabelName: snapshotEnvBinding.Spec.Application, EnvironmentLabelName: snapshotEnvBinding.Spec.Environment}); err != nil {
		return res, unableToFetchRemoteSecretsError
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		removeTarget(&remoteSecret, target)
		if err = f.client.Update(ctx, &remoteSecret); err != nil {
			return res, unableToUpdateRemoteSecret
		}
	}
	return finalizer.Result{}, nil
}