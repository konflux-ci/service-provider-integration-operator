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
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const linkedFileRequestBindingsFinalizerName = "spi.appstudio.redhat.com/file-linked-bindings"
const LinkedFileRequestLabel = "spi.appstudio.redhat.com/file-content-request-name"

var (
	noSuitableServiceProviderFound  = stderrors.New("unable to find a matching service provider for the given URL")
	unableToValidateServiceProvider = stderrors.New("unable to validate service provider for the given URL")
	noCredentialsFoundError         = stderrors.New("no suitable credentials found, please create SPIAccessToken or RemoteSecret")
	labelSelectorPredicate          predicate.Predicate
)

type SPIFileContentRequestReconciler struct {
	Configuration          *opconfig.OperatorConfiguration
	K8sClient              client.Client
	Scheme                 *runtime.Scheme
	HttpClient             *http.Client
	ServiceProviderFactory serviceprovider.Factory
	finalizers             finalizer.Finalizers
}

func init() {
	var err error
	labelSelectorPredicate, err = predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Operator: metav1.LabelSelectorOpExists,
				Key:      LinkedFileRequestLabel,
			},
		},
	})
	utilruntime.Must(err)
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *SPIFileContentRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(linkedFileRequestBindingsFinalizerName, &linkedFileRequestBindingsFinalizer{client: r.K8sClient}); err != nil {
		return fmt.Errorf("failed to register the linked bindings finalizer: %w", err)
	}
	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIFileContentRequest{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      o.GetLabels()[LinkedFileRequestLabel],
						Namespace: o.GetNamespace(),
					},
				},
			}
		}), builder.WithPredicates(labelSelectorPredicate)).
		Complete(r)
	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}
	return err

}

func (r *SPIFileContentRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile SPIFileContentRequest")

	request := api.SPIFileContentRequest{}

	if err := r.K8sClient.Get(ctx, req.NamespacedName, &request); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("object not found")
			return ctrl.Result{}, nil
		}

		lg.Error(err, "failed to get the object")
		return ctrl.Result{}, fmt.Errorf("failed to read the object: %w", err)
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &request)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	}
	if finalizationResult.Updated {
		if err = r.K8sClient.Update(ctx, &request); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update file request based on finalization result: %w", err)
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.K8sClient.Status().Update(ctx, &request); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the file request status based on finalization result: %w", err)
		}
	}

	if request.DeletionTimestamp != nil {
		lg.Info("file request object is being deleted")
		return ctrl.Result{}, nil
	}

	// cleanup file content requests by lifetime
	if time.Now().After(request.CreationTimestamp.Add(r.Configuration.FileContentRequestTtl)) {
		err := r.K8sClient.Delete(ctx, &request)
		if err != nil {
			lg.Error(err, "failed to cleanup file content request on reaching the max lifetime", "error", err)
			return ctrl.Result{}, fmt.Errorf("failed to cleanup file content request on reaching the max lifetime: %w", err)
		}
		lg.V(logs.DebugLevel).Info("file content request being cleaned up on reaching the max lifetime", "binding", request.ObjectMeta.Name, "requestLifetime", request.CreationTimestamp.String(), "requestTTL", r.Configuration.FileContentRequestTtl.String())
		return ctrl.Result{}, nil
	}

	// Controller logic usually should not be dependent on the object's status, but this helps us save some
	// service provider API calls which lessens the chance we hit a rate limit on the API.
	if request.Status.Phase == api.SPIFileContentRequestPhaseDelivered {
		return ctrl.Result{}, nil
	}

	sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, request.Spec.RepoUrl, request.Namespace)
	if err != nil {
		var validationErr validator.ValidationErrors
		if stderrors.As(err, &validationErr) {
			lg.Error(err, "unable to validate the service provider for the SPIFileContentRequest")
			return r.updateFileRequestStatusError(ctx, &request, unableToValidateServiceProvider)
		}
		lg.Error(err, "unable to get the service provider for the SPIFileContentRequest")
		// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
		// user fixes that URL. So no need to repeat the reconciliation.
		return r.updateFileRequestStatusError(ctx, &request, noSuitableServiceProviderFound)
	}

	if sp.GetDownloadFileCapability() == nil {
		return r.updateFileRequestStatusError(ctx, &request, serviceprovider.FileDownloadNotSupportedError{})
	}
	credentials, err := sp.LookupCredentials(ctx, r.K8sClient, &request)
	if err != nil {
		// We might fix the lookup error by retrying the reconciliation
		return r.requeueErrorWithStatusUpdate(ctx, &request, fmt.Errorf("error in credentials lookup for SPIFileContentRequest: %w", err))
	}
	if credentials == nil {
		return r.updateFileRequestStatusError(ctx, &request, noCredentialsFoundError)
	}

	contents, err := sp.GetDownloadFileCapability().DownloadFile(ctx, request.Spec, *credentials, r.Configuration.MaxFileDownloadSize)
	if err != nil {
		// We might fix the download error by retrying the reconciliation
		return r.requeueErrorWithStatusUpdate(ctx, &request, fmt.Errorf("error fetching file content: %w", err))
	}

	request.Status.ErrorMessage = ""
	request.Status.ContentEncoding = "base64"
	request.Status.Content = base64.StdEncoding.EncodeToString([]byte(contents))
	request.Status.Phase = api.SPIFileContentRequestPhaseDelivered

	if err := r.K8sClient.Status().Update(ctx, &request); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update the file request status: %w", err)
	}
	return ctrl.Result{RequeueAfter: r.durationUntilNextReconcile(&request)}, nil
}

func (r *SPIFileContentRequestReconciler) durationUntilNextReconcile(req *api.SPIFileContentRequest) time.Duration {
	return time.Until(req.CreationTimestamp.Add(r.Configuration.FileContentRequestTtl))
}

func deleteSyncedBinding(ctx context.Context, k8sClient client.Client, request *api.SPIFileContentRequest) error {
	bindings := &api.SPIAccessTokenBindingList{}
	if err := k8sClient.List(ctx, bindings, client.MatchingLabels{LinkedFileRequestLabel: request.Name}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting Token Binding items during cleanup: %w", err)
		} else {
			// already deleted, nothing to do
			return nil
		}
	}

	for i := range bindings.Items {
		if err := k8sClient.Delete(ctx, &bindings.Items[i]); err != nil {
			return fmt.Errorf("failed to delete token binding: %w", err)
		}
	}
	return nil
}

// requeueErrorWithStatusUpdate tries to update SPIFileContent's status with err, but prioritizes returning the updateErr
// if the status update fails. It returns Result for convenient use from reconcile.
func (r *SPIFileContentRequestReconciler) requeueErrorWithStatusUpdate(ctx context.Context, request *api.SPIFileContentRequest, err error) (ctrl.Result, error) {
	_, updateErr := r.updateFileRequestStatusError(ctx, request, err)
	if updateErr != nil {
		return ctrl.Result{}, updateErr
	}
	return ctrl.Result{}, err
}

// updateFileRequestStatusError puts the SPIFileContentRequest into tan error phase. It returns Result for convenient use from reconcile.
func (r *SPIFileContentRequestReconciler) updateFileRequestStatusError(ctx context.Context, request *api.SPIFileContentRequest, err error) (ctrl.Result, error) {
	request.Status.Content = ""
	request.Status.ContentEncoding = ""
	request.Status.ErrorMessage = err.Error()
	request.Status.Phase = api.SPIFileContentRequestPhaseError
	if err := r.K8sClient.Status().Update(ctx, request); err != nil {
		// we might consider using `errors.IsConflict(err)` here
		return ctrl.Result{}, fmt.Errorf("failed to update SPIFileContentRequest status with error: %w", err)
	}
	// We have successfully updated the status with error. We need to schedule reconciliation so that requests in error phase are cleaned as well.
	return ctrl.Result{RequeueAfter: r.durationUntilNextReconcile(request)}, nil
}

type linkedFileRequestBindingsFinalizer struct {
	client client.Client
}

var _ finalizer.Finalizer = (*linkedFileRequestBindingsFinalizer)(nil)

// Finalize removes the binding synced to the actual file content request which is being deleted
func (f *linkedFileRequestBindingsFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	res := finalizer.Result{}
	contentRequest, ok := obj.(*api.SPIFileContentRequest)
	if !ok {
		return res, unexpectedObjectTypeError
	}

	if err := deleteSyncedBinding(ctx, f.client, contentRequest); err != nil {
		return res, err
	}
	return finalizer.Result{}, nil
}
