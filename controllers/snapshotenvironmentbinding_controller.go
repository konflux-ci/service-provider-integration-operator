package controllers

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ApplicationLabelName = "appstudio.application"
	EnvironmentLabelName = "appstudio.environment"
	ComponentLabelName   = "appstudio.component"
)

// SnapshotEnvironmentBindingReconciler reconciles a SnapshotEnvironmentBinding object
type SnapshotEnvironmentBindingReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Configuration *opconfig.OperatorConfiguration
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/finalizers,verbs=update

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
	if err := r.Get(ctx, req.NamespacedName, &snapshotEnvBinding); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("SnapshotEnvironmentBinding already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the SnapshotEnvironmentBinding: %w", err)
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
	err := r.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: snapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		lg.Error(err, fmt.Sprintf("unable to get the Environment %s %v", environmentName, req.NamespacedName))
		return ctrl.Result{}, err
	}

	target, err := detectTargetFromEnvironment(ctx, environment)

	remoteSecretsList := api.RemoteSecretList{}
	if err := r.List(ctx, &remoteSecretsList, client.InNamespace(appNamespace), client.MatchingLabels{ApplicationLabelName: applicationName, EnvironmentLabelName: environmentName}); err != nil {
		spiFileContentRequestLog.Error(err, "Unable to fetch remote secrets list", "namespace", appNamespace)
		return ctrl.Result{}, unableToFetchBindingsError
	}

	for rs := range remoteSecretsList.Items {
		remoteSecret := remoteSecretsList.Items[rs]
		addTargetIfNotExists(&remoteSecret, target)
		if err = r.Client.Update(ctx, &remoteSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update secret spec: %w", err)
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

func detectTargetFromEnvironment(ctx context.Context, environment appstudiov1alpha1.Environment) (api.RemoteSecretTarget, error) {
	// Dummy load. Real impl should reverse pass Env -> DTC -> DT -> SpaceRequest to find out full target details
	return api.RemoteSecretTarget{Namespace: environment.Namespace}, nil
}
