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
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/namespacetarget"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/remotesecrets"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/remotesecretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type RemoteSecretReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configuration       *opconfig.OperatorConfiguration
	RemoteSecretStorage remotesecretstorage.RemoteSecretStorage
	finalizers          finalizer.Finalizers
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/finalizers,verbs=update

var _ reconcile.Reconciler = (*RemoteSecretReconciler)(nil)

const storageFinalizerName = "spi.appstudio.redhat.com/secret-storage" //#nosec G101 -- false positive, we're not storing any sensitive data using this

func (r *RemoteSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(storageFinalizerName, &remoteSecretStorageFinalizer{storage: r.RemoteSecretStorage}); err != nil {
		return fmt.Errorf("failed to register the remote secret storage finalizer: %w", err)
	}
	if err := r.finalizers.Register(linkedObjectsFinalizerName, &remoteSecretLinksFinalizer{client: r.Client, storage: r.RemoteSecretStorage}); err != nil {
		return fmt.Errorf("failed to register the remote secret links finalizer: %w", err)
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.RemoteSecret{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenDataUpdate{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return requestForDataUpdateOwner(object, "RemoteSecret", true)
		})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			return linksToReconcileRequests(mgr.GetLogger(), mgr.GetScheme(), o)
		})).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			return linksToReconcileRequests(mgr.GetLogger(), mgr.GetScheme(), o)
		})).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to configure the reconciler: %w", err)
	}
	return nil
}

func linksToReconcileRequests(lg logr.Logger, scheme *runtime.Scheme, o client.Object) []reconcile.Request {
	// XXX: here, we need to use the object markers of ALL SUPPORTED TARGETS to check if the secret doesn't contain a reference in their
	// format.

	nsMarker := namespacetarget.NamespaceObjectMarker{}

	refs, err := nsMarker.GetReferencingTargets(context.Background(), o)
	if err != nil {
		var gvk schema.GroupVersionKind
		gvks, _, _ := scheme.ObjectKinds(o)
		if len(gvks) > 0 {
			gvk = gvks[0]
		}
		lg.Error(err, "failed to list the referencing targets of the object", "objectKey", client.ObjectKeyFromObject(o), "gvk", gvk)
	}

	reqs := make([]reconcile.Request, len(refs))
	for i, r := range refs {
		reqs[i].NamespacedName = r
	}

	return reqs

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

	finalizationResult, err := r.finalizers.Finalize(ctx, remoteSecret)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, remoteSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.Client.Status().Update(ctx, remoteSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status based on finalization result: %w", err)
		}
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
	debugLog := log.FromContext(ctx).V(logs.DebugLevel)

	depHandler := bindings.DependentsHandler[*api.RemoteSecret]{
		Target: &namespacetarget.NamespaceTarget{
			Client:       r.Client,
			RemoteSecret: remoteSecret,
		},
		SecretBuilder: &remotesecrets.SecretBuilder{
			RemoteSecret: remoteSecret,
			Storage:      r.RemoteSecretStorage,
		},
		ObjectMarker: &namespacetarget.NamespaceObjectMarker{},
	}

	checkPoint, err := depHandler.CheckPoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to construct a checkpoint before dependent objects deployment: %w", err)
	}

	deps, _, err := depHandler.Sync(ctx, remoteSecret)
	if err != nil {
		if rerr := depHandler.RevertTo(ctx, checkPoint); rerr != nil {
			debugLog.Error(rerr, "failed to revert the sync of the dependent objects of the remote secret after failure", "syncError", err)
		}
		if serr := r.saveFailureStatus(ctx, remoteSecret, err); serr != nil {
			debugLog.Error(serr, "failed to save the failure status after failing to revert the failed sync", "syncError", err)
		}
		return fmt.Errorf("failed to sync the dependent objects of the remote secret: %w", err)
	}

	remoteSecret.Status.Target.Namespace.Namespace = deps.Secret.Namespace
	remoteSecret.Status.Target.Namespace.SecretName = deps.Secret.Name

	remoteSecret.Status.Target.Namespace.ServiceAccountNames = make([]string, 0, len(deps.ServiceAccounts))
	for i, sa := range deps.ServiceAccounts {
		remoteSecret.Status.Target.Namespace.ServiceAccountNames[i] = sa.Name
	}

	if err := r.Client.Status().Update(ctx, remoteSecret); err != nil {
		if rerr := depHandler.RevertTo(ctx, checkPoint); rerr != nil {
			debugLog.Error(rerr, "failed to revert the sync of the dependent objects of the remote secret after failure to record the dependent objects in the remote secret status", "updateError", err)
		}
		if serr := r.saveFailureStatus(ctx, remoteSecret, err); serr != nil {
			debugLog.Error(serr, "failed to save the failure status failing to update status (somewhat expected?)", "updateError", err)
		}
		return fmt.Errorf("failed to update the status with the info about dependent objects: %w", err)
	}

	if debugLog.Enabled() {
		saks := make([]client.ObjectKey, len(deps.ServiceAccounts))
		for i, sa := range deps.ServiceAccounts {
			saks[i] = client.ObjectKeyFromObject(sa)
		}
		debugLog.Info("successfully synced dependent objects of remote secret", "remoteSecret", client.ObjectKeyFromObject(remoteSecret), "syncedSecret", client.ObjectKeyFromObject(deps.Secret))
	}

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

type remoteSecretStorageFinalizer struct {
	storage remotesecretstorage.RemoteSecretStorage
}

var _ finalizer.Finalizer = (*remoteSecretStorageFinalizer)(nil)

func (f *remoteSecretStorageFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	err := f.storage.Delete(ctx, obj.(*api.RemoteSecret))
	if err != nil {
		err = fmt.Errorf("failed to delete the linked token during finalization of %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return finalizer.Result{}, err
}

type remoteSecretLinksFinalizer struct {
	client  client.Client
	storage remotesecretstorage.RemoteSecretStorage
}

var _ finalizer.Finalizer = (*linkedObjectsFinalizer)(nil)

// Finalize removes the secret and possibly also service account synced to the actual binging being deleted
func (f *remoteSecretLinksFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	remoteSecret, ok := obj.(*api.RemoteSecret)
	if !ok {
		return res, unexpectedObjectTypeError
	}

	lg := log.FromContext(ctx).V(logs.DebugLevel)

	key := client.ObjectKeyFromObject(remoteSecret)

	lg.Info("linked objects finalizer starting to clean up dependent objects", "remoteSecret", key)

	// XXX: all support targets need to do this cleanup... For now we only support the namespace target.

	dep := bindings.DependentsHandler[*api.RemoteSecret]{
		Target: &namespacetarget.NamespaceTarget{
			Client:       f.client,
			RemoteSecret: remoteSecret,
		},
		SecretBuilder: &remotesecrets.SecretBuilder{
			RemoteSecret: remoteSecret,
			Storage:      f.storage,
		},
		ObjectMarker: &namespacetarget.NamespaceObjectMarker{},
	}

	if err := dep.Cleanup(ctx); err != nil {
		lg.Error(err, "failed to clean up the dependent objects in the finalizer", "binding", client.ObjectKeyFromObject(remoteSecret))
		return res, fmt.Errorf("failed to clean up dependent objects in the finalizer: %w", err)
	}

	lg.Info("linked objects finalizer completed without failure", "binding", key)

	return res, nil
}
