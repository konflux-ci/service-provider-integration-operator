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
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/gitfile"

	"github.com/kcp-dev/logicalcluster/v2"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/infrastructure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var linkedBindingErrorStateError = stderrors.New("linked binding is in error state")

type SPIFileContentRequestReconciler struct {
	K8sClient  client.Client
	Scheme     *runtime.Scheme
	HttpClient *http.Client
}

var spiFileContentRequestLog = log.Log.WithName("spifilecontentrequest-controller")

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *SPIFileContentRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&api.SPIFileContentRequest{}).
		Watches(&source.Kind{Type: &api.SPIAccessTokenBinding{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			kcpWorkspace := logicalcluster.From(o)
			ctx := infrastructure.InitKcpContext(context.Background(), kcpWorkspace.String())

			fileRequests := &api.SPIFileContentRequestList{}
			if err := r.K8sClient.List(ctx, fileRequests, client.InNamespace(o.GetNamespace())); err != nil {
				spiFileContentRequestLog.Error(err, "Unable to fetch file content requests list", "namespace", o.GetNamespace())
				return []reconcile.Request{}
			}
			ret := make([]reconcile.Request, 0, len(fileRequests.Items))
			for _, fr := range fileRequests.Items {
				if fr.Status.LinkedBindingName == o.GetName() {
					ret = append(ret, reconcile.Request{
						ClusterName: kcpWorkspace.String(),
						NamespacedName: types.NamespacedName{
							Name:      fr.Name,
							Namespace: fr.Namespace,
						},
					})
				}
			}
			return ret
		})).
		Complete(r)
	if err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
	}
	return err

}

func (r *SPIFileContentRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = infrastructure.InitKcpContext(ctx, req.ClusterName)
	lg := log.FromContext(ctx)

	request := api.SPIFileContentRequest{}

	if err := r.K8sClient.Get(ctx, req.NamespacedName, &request); err != nil {
		if errors.IsNotFound(err) {
			lg.Info("object not found")
			return ctrl.Result{}, nil
		}

		lg.Error(err, "failed to get the object")
		return ctrl.Result{}, fmt.Errorf("failed to read the object: %w", err)
	}

	if request.DeletionTimestamp != nil {
		lg.Info("file request object is being deleted")
		return ctrl.Result{}, nil
	}

	if request.Status.Phase == "" {
		request.Status.Phase = api.SPIFileContentRequestPhaseAwaitingTokenData
	} else if request.Status.Phase == api.SPIFileContentRequestPhaseDelivered {
		// already injected, nothing to do
		return ctrl.Result{}, nil
	}

	if request.Status.LinkedBindingName == "" {
		if err := r.createAndLinkBinding(ctx, &request); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create the object: %w", err)
		}
	} else {
		binding := &api.SPIAccessTokenBinding{}
		if err := r.K8sClient.Get(ctx, client.ObjectKey{Name: request.Status.LinkedBindingName, Namespace: request.Namespace}, binding); err != nil {
			if errors.IsNotFound(err) {
				request.Status.LinkedBindingName = ""
				r.updateFileRequestStatusError(ctx, &request, err)
			}
		}
		//binding not yet fully ready to work with, let's try next time
		if binding.Status.Phase == "" {
			return reconcile.Result{}, nil
		}

		//binding not injected yet, let's just synchronize URL's and that's it
		if binding.Status.Phase == api.SPIAccessTokenBindingPhaseAwaitingTokenData {
			request.Status.OAuthUrl = binding.Status.OAuthUrl
			request.Status.TokenUploadUrl = binding.Status.UploadUrl
		} else if binding.Status.Phase == api.SPIAccessTokenBindingPhaseInjected {
			contents, err := gitfile.GetFileContents(ctx, r.K8sClient, *r.HttpClient, request.Namespace, binding.Status.SyncedObjectRef.Name, request.Spec.RepoUrl, request.Spec.FilePath, request.Spec.Ref)
			if err != nil {
				r.updateFileRequestStatusError(ctx, &request, err)
				return reconcile.Result{}, fmt.Errorf("error fetching file content: %w", err)
			}
			fileBytes, err := io.ReadAll(contents)
			if err != nil {
				r.updateFileRequestStatusError(ctx, &request, err)
				return reconcile.Result{}, fmt.Errorf("error reading file content: %w", err)
			}
			request.Status.ContentEncoding = "base64"
			request.Status.Content = base64.StdEncoding.EncodeToString(fileBytes)
			request.Status.Phase = api.SPIFileContentRequestPhaseDelivered
			if err = r.cleanupBinding(ctx, &request); err != nil {
				log.FromContext(ctx).Error(err, "failed to cleanup the binding, re-queueing it", "error", err)
				return reconcile.Result{Requeue: true}, err
			}
		} else {
			err := fmt.Errorf("%w: %s", linkedBindingErrorStateError, binding.Status.ErrorMessage)
			r.updateFileRequestStatusError(ctx, &request, err)
		}
	}
	if err := r.K8sClient.Status().Update(ctx, &request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the file request status", "error", err)
		return reconcile.Result{Requeue: true}, nil
	}

	return reconcile.Result{}, nil
}

func (r *SPIFileContentRequestReconciler) createAndLinkBinding(ctx context.Context, request *api.SPIFileContentRequest) error {
	newBinding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "file-retriever-binding-", Namespace: request.GetNamespace()},
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
		return fmt.Errorf("failed to create token binding: %w", err)
	}
	request.Status.LinkedBindingName = newBinding.GetName()
	return nil
}

func (r *SPIFileContentRequestReconciler) cleanupBinding(ctx context.Context, request *api.SPIFileContentRequest) error {
	binding := &api.SPIAccessTokenBinding{}
	if err := r.K8sClient.Get(ctx, client.ObjectKey{Name: request.Status.LinkedBindingName, Namespace: request.Namespace}, binding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting Token Binding item during cleanup: %w", err)
		} else {
			// already deleted, nothing to do
			return nil
		}
	}

	if err := r.K8sClient.Delete(ctx, binding); err != nil {
		return fmt.Errorf("failed to delete token binding: %w", err)
	}
	request.Status.LinkedBindingName = ""
	return nil
}

func (r *SPIFileContentRequestReconciler) updateFileRequestStatusError(ctx context.Context, request *api.SPIFileContentRequest, err error) {
	request.Status.ErrorMessage = err.Error()
	request.Status.Phase = api.SPIFileContentRequestPhaseError
	if err := r.K8sClient.Status().Update(ctx, request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status with error", "error", err)
	}

}
