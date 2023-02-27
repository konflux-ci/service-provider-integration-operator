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

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
	linkedBindingErrorStateError    = stderrors.New("linked binding is in error state")
	multipleLinkedBindingsError     = stderrors.New("multiple bindings found for the same file content request")
	noSuitableServiceProviderFound  = stderrors.New("unable to find a matching service provider for the given URL")
	unableToFetchBindingsError      = stderrors.New("unable to fetch the SPI Access token bindings list")
	unableToFetchTokenError         = stderrors.New("unable to fetch the SPI Access token")
	unableToValidateServiceProvider = stderrors.New("unable to validate service provider for the given URL")
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

var spiFileContentRequestLog = log.Log.WithName("spifilecontentrequest-controller")

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

	if request.Status.Phase == api.SPIFileContentRequestPhaseDelivered {
		// already injected, nothing to do
		return ctrl.Result{}, nil
	}
	// find associated binding
	var associatedBinding *api.SPIAccessTokenBinding
	bindingList := &api.SPIAccessTokenBindingList{}
	if err := r.K8sClient.List(ctx, bindingList, client.InNamespace(request.GetNamespace()), client.MatchingLabels{LinkedFileRequestLabel: request.Name}); err != nil {
		spiFileContentRequestLog.Error(err, "Unable to fetch bindings list", "namespace", request.GetNamespace())
		return ctrl.Result{}, unableToFetchBindingsError
	}
	if len(bindingList.Items) == 1 {
		associatedBinding = &bindingList.Items[0]
	} else if len(bindingList.Items) > 1 {
		lg.Error(multipleLinkedBindingsError, "found multiple bindings for the same SPIFileContentRequest", "request", request.Name)
		r.updateFileRequestStatusError(ctx, &request, multipleLinkedBindingsError)
		return ctrl.Result{}, nil
	}

	if associatedBinding == nil {
		// check if the URL is processable and create binding, otherwise fail fast
		sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, request.Spec.RepoUrl, request.Namespace)
		if err != nil {
			var validationErr validator.ValidationErrors
			if stderrors.As(err, &validationErr) {
				lg.Error(err, "unable to validate the service provider for the SPIFileContentRequest")
				r.updateFileRequestStatusError(ctx, &request, unableToValidateServiceProvider)
			} else {
				lg.Error(err, "unable to get the service provider for the SPIFileContentRequest")
				// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
				// user fixes that URL. So no need to repeat the reconciliation and therefore no error returned here.
				r.updateFileRequestStatusError(ctx, &request, noSuitableServiceProviderFound)
			}
			return ctrl.Result{}, nil
		}
		if sp.GetDownloadFileCapability() == nil {
			r.updateFileRequestStatusError(ctx, &request, serviceprovider.FileDownloadNotSupportedError{})
			return ctrl.Result{}, serviceprovider.FileDownloadNotSupportedError{}
		}
		bindingName, err := r.createAndLinkBinding(ctx, &request)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create the object: %w", err)
		}
		request.Status.Phase = api.SPIFileContentRequestPhaseAwaitingBinding
		lg = lg.WithValues("linked_binding", bindingName)
		if err := r.K8sClient.Status().Update(ctx, &request); err != nil {
			log.FromContext(ctx).Error(err, "failed to update the file binding link", "error", err)
			return reconcile.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}
	//binding not yet fully ready to work with, let's try next time
	if associatedBinding.Status.Phase == "" || associatedBinding.Status.UploadUrl == "" {
		return reconcile.Result{}, nil
	}

	lg = lg.WithValues("linked_binding", associatedBinding.Name)

	//binding not injected yet, let's just synchronize URL's and that's it
	if associatedBinding.Status.Phase == api.SPIAccessTokenBindingPhaseAwaitingTokenData {
		request.Status.Phase = api.SPIFileContentRequestPhaseAwaitingTokenData
		request.Status.OAuthUrl = associatedBinding.Status.OAuthUrl
		request.Status.TokenUploadUrl = associatedBinding.Status.UploadUrl
	} else if associatedBinding.Status.Phase == api.SPIAccessTokenBindingPhaseInjected {
		sp, err := r.ServiceProviderFactory.FromRepoUrl(ctx, request.Spec.RepoUrl, request.Namespace)
		if err != nil {
			var validationErr validator.ValidationErrors
			if stderrors.As(err, &validationErr) {
				lg.Error(err, "unable to validate the service provider for the SPIFileContentRequest")
				r.updateFileRequestStatusError(ctx, &request, unableToValidateServiceProvider)
			} else {
				lg.Error(err, "unable to get the service provider for the SPIFileContentRequest")
				// we determine the service provider from the URL in the spec. If we can't do that, nothing works until the
				// user fixes that URL. So no need to repeat the reconciliation and therefore no error returned here.
				r.updateFileRequestStatusError(ctx, &request, noSuitableServiceProviderFound)
			}
			return ctrl.Result{}, nil
		}
		if sp.GetDownloadFileCapability() == nil {
			r.updateFileRequestStatusError(ctx, &request, serviceprovider.FileDownloadNotSupportedError{})
			return ctrl.Result{}, serviceprovider.FileDownloadNotSupportedError{}
		}

		token := &api.SPIAccessToken{}
		err = r.K8sClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: associatedBinding.Status.LinkedAccessTokenName}, token)
		if err != nil {
			lg.Error(err, "unable to fetch the token")
			r.updateFileRequestStatusError(ctx, &request, unableToFetchTokenError)
			return ctrl.Result{}, fmt.Errorf("unable to fetch the SPI Access token: %w", err)
		}
		contents, err := sp.GetDownloadFileCapability().DownloadFile(ctx, request.Spec.RepoUrl, request.Spec.FilePath, request.Spec.Ref, token, r.Configuration.MaxFileDownloadSize)
		if err != nil {
			r.updateFileRequestStatusError(ctx, &request, err)
			return reconcile.Result{}, fmt.Errorf("error fetching file content: %w", err)
		}
		request.Status.OAuthUrl = ""
		request.Status.ErrorMessage = ""
		request.Status.ContentEncoding = "base64"
		request.Status.Content = base64.StdEncoding.EncodeToString([]byte(contents))
		request.Status.Phase = api.SPIFileContentRequestPhaseDelivered
		if err = deleteSyncedBinding(ctx, r.K8sClient, &request); err != nil {
			log.FromContext(ctx).Error(err, "failed to cleanup the binding, re-queueing it", "error", err)
			return reconcile.Result{Requeue: true}, err
		}
	} else {
		err := fmt.Errorf("%w: %s", linkedBindingErrorStateError, associatedBinding.Status.ErrorMessage)
		r.updateFileRequestStatusError(ctx, &request, err)
	}
	if err := r.K8sClient.Status().Update(ctx, &request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the file request status", "error", err)
		return reconcile.Result{Requeue: true}, nil
	}

	return ctrl.Result{RequeueAfter: r.durationUntilNextReconcile(&request)}, nil
}

func (r *SPIFileContentRequestReconciler) durationUntilNextReconcile(cr *api.SPIFileContentRequest) time.Duration {
	return time.Until(cr.CreationTimestamp.Add(r.Configuration.FileContentRequestTtl).Add(r.Configuration.DeletionGracePeriod))
}

func (r *SPIFileContentRequestReconciler) createAndLinkBinding(ctx context.Context, request *api.SPIFileContentRequest) (string, error) {
	newBinding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "file-retriever-binding-", Namespace: request.GetNamespace(), Labels: map[string]string{LinkedFileRequestLabel: request.Name}},
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: request.RepoUrl(),
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeRead,
						Area: api.PermissionAreaRepository,
					},
				},
			},
			Secret: api.SecretSpec{
				Type: corev1.SecretTypeBasicAuth,
			},
		},
	}
	if err := r.K8sClient.Create(ctx, newBinding); err != nil {
		return "", fmt.Errorf("failed to create token binding: %w", err)
	}
	return newBinding.Name, nil
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

func (r *SPIFileContentRequestReconciler) updateFileRequestStatusError(ctx context.Context, request *api.SPIFileContentRequest, err error) {
	request.Status.ErrorMessage = err.Error()
	request.Status.Phase = api.SPIFileContentRequestPhaseError
	if err := r.K8sClient.Status().Update(ctx, request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status with error", "error", err)
	}

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
