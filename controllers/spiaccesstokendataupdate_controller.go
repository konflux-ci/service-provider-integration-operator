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
	"time"

	"github.com/go-logr/logr"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var objectLifetimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: config.MetricsNamespace,
	Subsystem: config.MetricsSubsystem,
	Name:      "spiaccesstokendataupdate_lifetime_seconds",
	Help:      "The lifetime in seconds of the SPIAccessTokenDataUpdate objects. This measures the time between storing the new token data in Vault and the operator reacting on its presence.",
})

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokendataupdates,verbs=get;list;watch;delete

// SPIAccessTokenDataUpdateReconciler reconciles a SPIAccessTokenDataUpdate object
type SPIAccessTokenDataUpdateReconciler struct {
	client.Client
}

// SetupWithManager sets up the controller with the Manager.
func (r *SPIAccessTokenDataUpdateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIAccessTokenDataUpdate{}).
		Complete(r); err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
		return err
	}

	if err := metrics.Registry.Register(objectLifetimeMetric); err != nil {
		return fmt.Errorf("failed to register the SPIAccessTokenDataUpdate metrics: %w", err)
	}

	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SPIAccessTokenDataUpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile SPIAccessTokenDataUpdate")

	update := api.SPIAccessTokenDataUpdate{}

	if err := r.Get(ctx, req.NamespacedName, &update); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("token data update already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to load the token data update from the cluster: %w", err)
	}

	if update.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("token data update being deleted, no other changes required after completed finalization")
		return ctrl.Result{}, nil
	}

	lg = lg.WithValues("token_name", update.Spec.TokenName)

	// The token data changed in the token storage. We need to delete the token metadata so that all the bindings are
	// updated with the latest data...
	token := &api.SPIAccessToken{}
	if err := r.Get(ctx, client.ObjectKey{Name: update.Spec.TokenName, Namespace: update.Namespace}, token); err != nil {
		if !errors.IsNotFound(err) {
			lg.Error(err, "failed to obtain the updated token")
			return ctrl.Result{}, fmt.Errorf("failed to obtain the updated token: %w", err)
		}
	}

	creationTime := update.CreationTimestamp

	// Here, we just directly delete the object, because it serves only as a trigger for reconciling the token
	// The SPIAccessTokenReconciler is set up to watch the update objects and translate those to reconciliation requests
	// of the tokens themselves.
	if err := r.Delete(ctx, &update); err != nil {
		lg.Error(err, "failed to delete the processed data token update")
		return ctrl.Result{}, fmt.Errorf("failed to delete the processed data token update: %w", err)
	}

	lg.V(logs.DebugLevel).Info("token data update deleted: %s", "SPIAccessTokenDataUpdate.name", req.Name)

	objectLifetimeMetric.Observe(time.Since(creationTime.Time).Seconds())

	return ctrl.Result{}, nil
}
