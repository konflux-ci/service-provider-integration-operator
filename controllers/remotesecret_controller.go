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
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/remotesecretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type RemoteSecretReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configuration       *opconfig.OperatorConfiguration
	RemoteSecretStorage remotesecretstorage.RemoteSecretStorage
}

var _ reconcile.Reconciler = (*RemoteSecretReconciler)(nil)

func (r *RemoteSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.RemoteSecret{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to configure the reconciler: %w", err)
	}
	return nil
}

// Reconcile implements reconcile.Reconciler
func (r *RemoteSecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile RemoteSecret")

	remoteSecret := &api.RemoteSecret{}

	if err := r.Get(ctx, req.NamespacedName, remoteSecret); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("RemoteSecret already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the RemoteSecret: %w", err)
	}

	if remoteSecret.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("RemoteSecret is being deleted. skipping reconciliation")
		return ctrl.Result{}, nil
	}

	secretData, found, err := r.findSecretData(ctx, remoteSecret)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		return ctrl.Result{}, nil
	}

	if remoteSecret.Spec.Target.Namespace != "" {
		err = r.deployToNamespace(ctx, remoteSecret, secretData)
	} else if remoteSecret.Spec.Target.Environment != "" {
		err = r.deployToEnvironment(ctx, remoteSecret, secretData)
	}

	return ctrl.Result{}, err
}

func (r *RemoteSecretReconciler) saveSuccessStatus(ctx context.Context, remoteSecret *api.RemoteSecret) error {
	meta.RemoveStatusCondition(&remoteSecret.Status.Conditions, string(api.RemoteSecretConditionTypeError))

	if err := r.Client.Status().Update(ctx, remoteSecret); err != nil {
		return fmt.Errorf("failed to update the status: %w", err)
	}
	return nil
}

func (r *RemoteSecretReconciler) saveFailureStatus(ctx context.Context, remoteSecret *api.RemoteSecret, err error) error {
	meta.SetStatusCondition(&remoteSecret.Status.Conditions, metav1.Condition{
		Type:    string(api.RemoteSecretConditionTypeError),
		Status:  metav1.ConditionTrue,
		Reason:  string(api.RemoteSecretReasonError),
		Message: err.Error(),
	})

	if err := r.Client.Status().Update(ctx, remoteSecret); err != nil {
		return fmt.Errorf("failed to update the status: %w", err)
	}
	return nil
}

func (r *RemoteSecretReconciler) findSecretData(ctx context.Context, remoteSecret *api.RemoteSecret) (*remotesecretstorage.SecretData, bool, error) {
	lg := log.FromContext(ctx)
	secretData, err := r.RemoteSecretStorage.Get(ctx, remoteSecret)
	if err != nil {
		if stdErrors.Is(err, secretstorage.NotFoundError) {
			meta.SetStatusCondition(&remoteSecret.Status.Conditions, metav1.Condition{
				Type:    string(api.RemoteSecretConditionTypeDataObtained),
				Status:  metav1.ConditionFalse,
				Reason:  string(api.RemoteSecretReasonAwaitingTokenData),
				Message: "The data of the remote secret not found in storage. Please provide it.",
			})

			if err := r.saveSuccessStatus(ctx, remoteSecret); err != nil {
				return nil, false, err
			}
			return nil, false, nil
		}

		if updateErr := r.saveFailureStatus(ctx, remoteSecret, err); updateErr != nil {
			lg.Error(updateErr, "failed to update the remote secret with the error while getting the data from storage", "underlyingError", err)
		}

		return nil, false, fmt.Errorf("failed to find the secret associated with the remote secret %s: %w", client.ObjectKeyFromObject(remoteSecret), err)
	}

	return secretData, true, nil
}

func (r *RemoteSecretReconciler) deployToNamespace(ctx context.Context, remoteSecret *api.RemoteSecret, data *remotesecretstorage.SecretData) error {

	return nil
}

func (r *RemoteSecretReconciler) deployToEnvironment(ctx context.Context, remoteSecret *api.RemoteSecret, data *remotesecretstorage.SecretData) error {
	meta.SetStatusCondition(&remoteSecret.Status.Conditions, metav1.Condition{
		Type:    string(api.RemoteSecretConditionTypeDeployed),
		Status:  metav1.ConditionFalse,
		Reason:  string(api.RemoteSecretReasonUnsupported),
		Message: "The enviroment remote secret target is not supported yet",
	})
	if err := r.saveSuccessStatus(ctx, remoteSecret); err != nil {
		return err
	}
	return nil
}
