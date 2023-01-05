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
	spiTokenNameLabel = "spi.appstudio.redhat.com/token-name"    //#nosec G101 -- false positive, this is not a token
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
		return ctrl.Result{}, fmt.Errorf("can not get uploadSecretList of secrets: %w,", err)
	}

	for i, uploadSecret := range uploadSecretList.Items {

		var accessToken spi.SPIAccessToken

		// we immediatelly delete the Secret
		err := r.Delete(ctx, &uploadSecretList.Items[i])
		if err != nil {
			logError(ctx, uploadSecret, fmt.Errorf("can not delete the Secret: %w", err), r, lg)
			continue
		}

		// if spiTokenName field is not empty - try to find SPIAccessToken by it
		if uploadSecret.Labels[spiTokenNameLabel] != "" {
			spiTokenName := uploadSecret.Labels[spiTokenNameLabel]
			accessToken = spi.SPIAccessToken{}
			err = r.Get(ctx, types.NamespacedName{Name: spiTokenName, Namespace: uploadSecret.Namespace}, &accessToken)

			if err != nil {
				logError(ctx, uploadSecret, fmt.Errorf("can not find SPI access token %s: %w ", spiTokenName, err), r, lg)
				continue
			} else {
				lg.V(logs.DebugLevel).Info("SPI Access Token found : " + accessToken.Name)
			}

			// spiTokenName field is empty
			// check providerUrl field and if not empty - try to find the token for this provider instance
		} else if string(uploadSecret.Data[providerUrlField]) != "" {
			providerUrl := string(uploadSecret.Data[providerUrlField])
			// NOTE: it does not fit advanced policy of matching token!
			// Do we need it as an SPI "API function" which take into account this policy?
			tkn := findTokenByUrl(ctx, providerUrl, uploadSecret.Namespace, r, lg)
			// create new SPIAccessToken if there are no for such provider instance (URL)
			if tkn == nil {

				accessToken, err = createSpiAccessToken(ctx, uploadSecret.Namespace, r, providerUrl, lg)
				if err != nil {
					logError(ctx, uploadSecret, fmt.Errorf("can not create SPI access token for %s: %w", providerUrl, err), r, lg)
					continue
				} else {
					// this is the only place where we can get the name of just created SPIAccessToken
					// which is presumably OK since SPI (binding) controller will look for the token by type/URL ?
					lg.V(logs.DebugLevel).Info("SPI Access Token created : ", "SPIAccessTokenName.name", accessToken.Name)
				}
			} else {
				accessToken = *tkn
				lg.V(logs.DebugLevel).Info("SPI Access Token found by providerUrl : ", "SPIAccessTokenName.name", accessToken.Name)
			}

		} else {
			logError(ctx, uploadSecret, fmt.Errorf("secret is invalid, it is labeled with %s but neither %s nor %s field provided: %w", tokenSecretLabel, spiTokenNameLabel, providerUrlField, err), r, lg)
			continue
		}

		token := spi.Token{
			Username:    string(uploadSecret.Data["userName"]),
			AccessToken: string(uploadSecret.Data["tokenData"]),
		}

		logs.AuditLog(ctx).Info("manual token upload initiated", uploadSecret.Namespace, accessToken.Name)
		err = r.TokenStorage.Store(ctx, &accessToken, &token)
		if err != nil {
			logError(ctx, uploadSecret, fmt.Errorf("token storing failed: %w", err), r, lg)
			logs.AuditLog(ctx).Error(err, "manual token upload failed", uploadSecret.Namespace, accessToken.Name)
			continue
		}
		logs.AuditLog(ctx).Info("manual token upload completed", uploadSecret.Namespace, accessToken.Name)

		tryDeleteEvent(ctx, uploadSecret.Name, req.Namespace, r, lg)

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
func logError(ctx context.Context, secret corev1.Secret, err error, r *TokenUploadReconciler, lg logr.Logger) {

	lg.Error(err, "Secret upload failed:")

	tryDeleteEvent(ctx, secret.Name, secret.Namespace, r, lg)

	secretErrEvent := &corev1.Event{}
	secretErrEvent.Name = secret.Name
	secretErrEvent.Message = err.Error()
	secretErrEvent.Namespace = secret.Namespace
	secretErrEvent.Reason = "Can not upload access token"
	secretErrEvent.InvolvedObject = corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion}
	secretErrEvent.Type = "Error"
	secretErrEvent.LastTimestamp = metav1.NewTime(time.Now())

	err = r.Create(ctx, secretErrEvent)

	if err != nil {
		lg.Error(err, "Event creation failed for Secret: ", "secret.name", secret.Name)
	}

}

func tryDeleteEvent(ctx context.Context, secretName string, ns string, r *TokenUploadReconciler, lg logr.Logger) {
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

func findTokenByUrl(ctx context.Context, url string, ns string, r *TokenUploadReconciler, lg logr.Logger) *spi.SPIAccessToken {

	tokenList := spi.SPIAccessTokenList{}
	if err := r.List(ctx, &tokenList, client.InNamespace(ns)); err != nil {
		lg.Error(err, "Can not get tokenList of tokens ")
		return nil
	}

	for _, token := range tokenList.Items {

		if token.Spec.ServiceProviderUrl == url {
			return &token
		}
	}
	return nil
}

func createSpiAccessToken(ctx context.Context, ns string, r *TokenUploadReconciler, providerUrl string, lg logr.Logger) (spi.SPIAccessToken, error) {

	lg.V(logs.DebugLevel).Info("can not find SPI access token trying to create new one")

	accessToken := spi.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "generated-spi-access-token-",
			Namespace:    ns,
		},
		Spec: spi.SPIAccessTokenSpec{
			ServiceProviderUrl: providerUrl,
		},
	}
	err := r.Create(ctx, &accessToken)
	return accessToken, fmt.Errorf("can not create SPIAccessToken: %w", err)
}
