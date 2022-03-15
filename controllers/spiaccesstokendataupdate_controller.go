package controllers

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokendataupdates,verbs=get;list;watch;delete

// SPIAccessTokenDataUpdateReconciler reconciles a SPIAccessTokenDataUpdate object
type SPIAccessTokenDataUpdateReconciler struct {
	client.Client
}

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenDataUpdateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessTokenDataUpdate{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenDataUpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling")

	update := api.SPIAccessTokenDataUpdate{}

	if err := r.Get(ctx, req.NamespacedName, &update); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("token data update already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, NewReconcileError(err, "failed to load the token data update from the cluster")
	}

	if update.DeletionTimestamp != nil {
		lg.Info("token data update being deleted, no other changes required after completed finalization")
		return ctrl.Result{}, nil
	}

	// Here, we just directly delete the object, because it serves only as a trigger for reconciling the token
	// The SPIAccessTokenReconciler is set up to watch the update objects and translate those to reconciliation requests
	// of the tokens themselves.
	return ctrl.Result{}, r.Delete(ctx, &update)
}
