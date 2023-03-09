/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	kuberrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tokenSecretLabel  = "spi.appstudio.redhat.com/upload-secret" //#nosec G101 -- false positive, this is not a token
	spiTokenNameField = "spiTokenName"                           //#nosec G101 -- false positive, this is not a token
	providerUrlField  = "providerUrl"
)

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spiaccesstokendataupdates,verbs=create
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;delete

// TokenUploadReconciler reconciles a Secret object
type TokenUploadReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	TokenStorage tokenstorage.NotifyingTokenStorage
}

func (r *TokenUploadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	lg := log.FromContext(ctx)
	uploadSecretList := &corev1.SecretList{}

	// uploadSecretList secrets labeled with "spi.appstudio.redhat.com/upload-secret"
	err := r.List(ctx, uploadSecretList, client.HasLabels{tokenSecretLabel})
	if err != nil {
		lg.Error(err, "can not get uploadSecretList of secrets")
		return ctrl.Result{}, fmt.Errorf("can not get uploadSecretList of secrets: %w", err)
	}

	for i, uploadSecret := range uploadSecretList.Items {

		// we immediately delete the Secret
		err := r.Delete(ctx, &uploadSecretList.Items[i])
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("can not delete the Secret: %w", err), lg)
			continue
		}

		// try to find  SPIAccessToken
		accessToken, err := r.findSpiAccessToken(ctx, uploadSecret, lg)
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("can not find SPI access token: %w ", err), lg)
			continue
		} else if accessToken == nil {
			// SPIAccessToken does not exist, so create it
			accessToken, err = r.createSpiAccessToken(ctx, uploadSecret, lg)
			if err != nil {
				r.logError(ctx, uploadSecret, fmt.Errorf("can not create SPI access token: %w ", err), lg)
				continue
			}
		}

		token := spi.Token{
			Username:    string(uploadSecret.Data["userName"]),
			AccessToken: string(uploadSecret.Data["tokenData"]),
		}

		logs.AuditLog(ctx).Info("manual token upload initiated", uploadSecret.Namespace, accessToken.Name)
		// Upload Token, it will cause update SPIAccessToken State as well
		err = r.TokenStorage.Store(ctx, accessToken, &token)
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("failed to store the token: %w", err), lg)
			logs.AuditLog(ctx).Error(err, "manual token upload failed", uploadSecret.Namespace, accessToken.Name)
			continue
		}
		logs.AuditLog(ctx).Info("manual token upload completed", uploadSecret.Namespace, accessToken.Name)

		r.tryDeleteEvent(ctx, uploadSecret.Name, req.Namespace, lg)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenUploadReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(createTokenPredicate()).
		Complete(r); err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
		return err
	}
	return nil
}

func createTokenPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.GetLabels()[tokenSecretLabel]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, newLabelExists := e.ObjectNew.GetLabels()[tokenSecretLabel]
			_, oldLabelExists := e.ObjectOld.GetLabels()[tokenSecretLabel]
			if newLabelExists && !oldLabelExists {
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

// reports error in both Log and Event in current Namespace
func (r *TokenUploadReconciler) logError(ctx context.Context, secret corev1.Secret, err error, lg logr.Logger) {

	lg.Error(err, "Secret upload failed:")

	r.tryDeleteEvent(ctx, secret.Name, secret.Namespace, lg)

	secretErrEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Message:        err.Error(),
		Reason:         "Can not upload access token",
		InvolvedObject: corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion},
		Type:           "Error",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}

	err = r.Create(ctx, secretErrEvent)
	if err != nil {
		lg.Error(err, "Event creation failed for Secret: ", "secret.name", secret.Name)
	}

}

// Contract: having at only one event if upload failed and no events if uploaded.
// For this need to delete the event every attempt
func (r *TokenUploadReconciler) tryDeleteEvent(ctx context.Context, secretName string, ns string, lg logr.Logger) {
	stored := &corev1.Event{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, stored)

	if err == nil {
		lg.V(logs.DebugLevel).Info("event Found and will be deleted: ", "event.name", stored.Name)
		err = r.Delete(ctx, stored)
		if err != nil {
			lg.Error(err, " can not delete Event ")
		}
	}
}

func (r *TokenUploadReconciler) findSpiAccessToken(ctx context.Context, uploadSecret corev1.Secret, lg logr.Logger) (*spi.SPIAccessToken, error) {
	spiTokenName := string(uploadSecret.Data[spiTokenNameField])
	if spiTokenName == "" {
		lg.V(logs.DebugLevel).Info("No spiTokenName found, will try to create with generated ", "spiTokenName", spiTokenName)
		return nil, nil
	}

	accessToken := spi.SPIAccessToken{}
	err := r.Get(ctx, types.NamespacedName{Name: spiTokenName, Namespace: uploadSecret.Namespace}, &accessToken)

	if err != nil {
		if kuberrors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("SPI Access Token NOT found, will try to create  ", "SPIAccessToken.name", accessToken.Name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("can not find SPI access token %s: %w ", spiTokenName, err)
		}
	} else {
		lg.V(logs.DebugLevel).Info("SPI Access Token found : ", "SPIAccessToken.name", accessToken.Name)
		return &accessToken, nil
	}
}

func (r *TokenUploadReconciler) createSpiAccessToken(ctx context.Context, uploadSecret corev1.Secret, lg logr.Logger) (*spi.SPIAccessToken, error) {
	accessToken := spi.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: uploadSecret.Namespace,
		},
		Spec: spi.SPIAccessTokenSpec{
			ServiceProviderUrl: string(uploadSecret.Data[providerUrlField]),
		},
	}
	spiTokenName := string(uploadSecret.Data[spiTokenNameField])
	if spiTokenName == "" {
		accessToken.GenerateName = "generated-"
	} else {
		accessToken.Name = spiTokenName
	}

	err := r.Create(ctx, &accessToken)
	if err == nil {
		lg.V(logs.DebugLevel).Info("SPI Access Token created : ", "SPIAccessToken.name", accessToken.Name)
		return &accessToken, nil
	} else {
		return nil, fmt.Errorf("can not create SPIAccessToken %s. Reason: %w", accessToken.Name, err)
	}
}
