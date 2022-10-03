package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/gitfile"
	"io"
	"math/rand"
	"time"

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

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
)

type SPIFileContentRequestReconciler struct {
	client.Client
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
			ctx := context.TODO()
			if !kcpWorkspace.Empty() {
				ctx = logicalcluster.WithCluster(ctx, kcpWorkspace)
			}

			fileRequests := &api.SPIFileContentRequestList{}
			if err := r.Client.List(ctx, fileRequests, client.InNamespace(o.GetNamespace())); err != nil {
				spiFileContentRequestLog.Error(err, "Unable to fetch file content requests list", "namespace", o.GetNamespace())
				return []reconcile.Request{}
			}
			ret := make([]reconcile.Request, 0, len(fileRequests.Items))
			for _, r := range fileRequests.Items {
				if r.Status.LinkedBindingName == o.GetName() {
					ret = append(ret, reconcile.Request{
						ClusterName: kcpWorkspace.String(),
						NamespacedName: types.NamespacedName{
							Name:      r.Name,
							Namespace: r.Namespace,
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

func (r *SPIFileContentRequestReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = infrastructure.InitKcpControllerContext(ctx, req)
	lg := log.FromContext(ctx)

	request := api.SPIFileContentRequest{}

	if err := r.Get(ctx, req.NamespacedName, &request); err != nil {
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

	//TODO: lifetime cleanup

	if request.Status.Phase == "" {
		request.Status.Phase = api.SPIFileContentRequestPhaseAwaitingTokenData
	}

	if request.Status.LinkedBindingName == "" {
		if err := r.createAndLinkBinding(ctx, request); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create the object: %w", err)
		}
	} else {
		binding := &api.SPIAccessTokenBinding{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: request.Status.LinkedBindingName, Namespace: request.Namespace}, binding); err != nil {
			if errors.IsNotFound(err) {
				request.Status.LinkedBindingName = ""
				r.updateFileRequestStatusError(ctx, &request, err)
			}
		}
		//binding not injected yet, let's just synchronize URL's and that's it
		if binding.Status.Phase == api.SPIAccessTokenBindingPhaseAwaitingTokenData {
			request.Status.OAuthUrl = binding.Status.OAuthUrl
			request.Status.UploadUrl = binding.Status.UploadUrl
			if err := r.Client.Status().Update(ctx, &request); err != nil {
				r.updateFileRequestStatusError(ctx, &request, err)
				return reconcile.Result{}, err
			}
			return ctrl.Result{}, nil
		} else if binding.Status.Phase == api.SPIAccessTokenBindingPhaseError {
			err := fmt.Errorf("linked binding is in error state: %s", binding.Status.ErrorMessage)
			r.updateFileRequestStatusError(ctx, &request, err)
			return reconcile.Result{}, err
		} else {
			contents, err := gitfile.GetFileContents(ctx, r.Client, request.Namespace, binding.Status.SyncedObjectRef.Name, request.Spec.RepoUrl, request.Spec.FilePath, request.Spec.Ref)
			if err != nil {
				r.updateFileRequestStatusError(ctx, &request, err)
				return reconcile.Result{}, err
			}
			fileBytes, err := io.ReadAll(contents)
			if err != nil {
				r.updateFileRequestStatusError(ctx, &request, err)
				return reconcile.Result{}, err
			}
			request.Status.ContentEncoding = "base64"
			request.Status.Content = base64.StdEncoding.EncodeToString(fileBytes)
			request.Status.Phase = api.SPIFileContentRequestPhaseDelivered
		}
	}
	if err := r.Client.Status().Update(ctx, &request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the file request status", "error", err)
	}

	return reconcile.Result{}, nil
}

func (r *SPIFileContentRequestReconciler) createAndLinkBinding(ctx context.Context, request api.SPIFileContentRequest) error {
	lg := log.FromContext(ctx)
	newBinding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "file-retriever-binding-" + randStringBytes(6), Namespace: request.GetNamespace()},
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: request.RepoUrl(),
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeReadWrite,
						Area: api.PermissionAreaRepository,
					},
				},
			},
			Secret: api.SecretSpec{
				Type: corev1.SecretTypeBasicAuth,
			},
		},
	}
	err := r.Client.Create(ctx, newBinding)
	if err != nil {
		lg.Error(err, "Error creating Token Binding item")
		return fmt.Errorf("failed to create token binding: %w", err)
	}
	request.Status.LinkedBindingName = newBinding.GetName()
	return nil
}

func (r *SPIFileContentRequestReconciler) updateFileRequestStatusError(ctx context.Context, request *api.SPIFileContentRequest, err error) {
	request.Status.ErrorMessage = err.Error()
	request.Status.Phase = api.SPIFileContentRequestPhaseError
	if err := r.Client.Status().Update(ctx, request); err != nil {
		log.FromContext(ctx).Error(err, "failed to update the status with error", "error", err)
	}

}

func randStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec // we're using this to produce a random name so
		// the weakness of the generator is not a big deal here
	}
	return string(b)
}
