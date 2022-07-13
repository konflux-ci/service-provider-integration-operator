package controllers

import (
	"context"
	"fmt"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type SPIAccessTokenBindingCleanupReconciler struct {
	client.Client
}

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenBindingCleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessToken{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestsForTokenInObjectNamespace(object, func() string {
				return object.GetLabels()[opconfig.SPIAccessTokenLinkLabel]
			})
		})).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
		}).
		Complete(r)
	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}

	return err
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenBindingCleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling")

	token := api.SPIAccessToken{}

	if err := r.Get(ctx, req.NamespacedName, &token); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("token gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to load the token from the cluster: %w", err)
	}

	if token.DeletionTimestamp != nil {
		lg.Info("token being deleted, no other changes required after completed finalization")
		return ctrl.Result{}, nil
	}

	if token.Status.Phase != api.SPIAccessTokenPhaseAwaitingTokenData {
		//nothing to do
		return ctrl.Result{}, nil
	}

	if err := r.Delete(ctx, &token); err != nil {
		lg.Error(err, "failed to delete the processed token")
		return ctrl.Result{}, fmt.Errorf("failed to delete the processed token: %w", err)
	}
	return ctrl.Result{}, nil
}
