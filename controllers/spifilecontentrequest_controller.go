package controllers

import (
	"context"
	"fmt"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SPIFileContentRequestReconciler struct {
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokenbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;watch;create;update;list;delete

// SetupWithManager sets up the controller with the Manager.
func (r *SPIFileContentRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {

	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIFileContentRequest{}).
		Complete(r)
	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}
	return err

}

func (r *SPIFileContentRequestReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	//TODO implement me
	panic("implement me")
}
