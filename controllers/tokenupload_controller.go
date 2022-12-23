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

const tokenSecretLabel = "spi.appstudio.redhat.com/upload-secret" //#nosec G101 -- false positive, this is not a token
const spiTokenNameLabel = "spi.appstudio.redhat.com/token-name"   //#nosec G101 -- false positive, this is not a token
const providerUrlLabel = "spi.appstudio.redhat.com/providerUrl"

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

	list := &corev1.SecretList{}

	// list secrets labeled with "spi.appstudio.redhat.com/upload-secret"
	err := r.List(ctx, list, client.HasLabels{tokenSecretLabel})
	if err != nil {
		lg.Error(err, "can not get list of secrets")
		return ctrl.Result{}, fmt.Errorf("can not get list of secrets: %w,", err)
	}

	for i, s := range list.Items {

		var accessToken spi.SPIAccessToken

		// we immediatelly delete the Secret
		err := r.Delete(ctx, &list.Items[i])
		if err != nil {
			logError(ctx, s, fmt.Errorf("can not delete the Secret: %w", err), r, lg)
			continue
		}

		// if spiTokenName field is not empty - try to find SPIAccessToken by it
		if s.Labels[spiTokenNameLabel] != "" {
			spiTokenName := s.Labels[spiTokenNameLabel]
			accessToken = spi.SPIAccessToken{}
			err = r.Get(ctx, types.NamespacedName{Name: spiTokenName, Namespace: s.Namespace}, &accessToken)

			if err != nil {
				logError(ctx, s, fmt.Errorf("can not find SPI access token %s: %w ", spiTokenName, err), r, lg)
				continue
			} else {
				lg.V(logs.DebugLevel).Info("SPI Access Token found : " + accessToken.Name)
			}

			// spiTokenName field is empty
			// check providerUrl field and if not empty - try to find the token for this provider instance
		} else if s.Labels[providerUrlLabel] != "" {
			providerUrl := s.Labels[providerUrlLabel]
			// NOTE: it does not fit advanced policy of matching token!
			// Do we need it as an SPI "API function" which take into account this policy?
			tkn := findTokenByUrl(ctx, providerUrl, s.Namespace, r, lg)
			// create new SPIAccessToken if there are no for such provider instance (URL)
			if tkn == nil {

				lg.V(logs.DebugLevel).Info("can not find SPI access token trying to create new one")
				accessToken = spi.SPIAccessToken{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "generated-spi-access-token-",
						Namespace:    s.Namespace,
					},
					Spec: spi.SPIAccessTokenSpec{
						ServiceProviderUrl: providerUrl,
					},
				}
				err = r.Create(ctx, &accessToken)
				if err != nil {
					logError(ctx, s, fmt.Errorf("can not create SPI access token for %s: %w", providerUrl, err), r, lg)
					continue
				} else {
					// this is the only place where we can get the name of just created SPIAccessToken
					// which is presumably OK since SPI (binding) controller will look for the token by type/URL ?
					lg.V(logs.DebugLevel).Info("SPI Access Token created : " + accessToken.Name)
				}
			} else {
				accessToken = *tkn
				lg.V(logs.DebugLevel).Info("SPI Access Token found by providerUrl : " + accessToken.Name + " nothing changed.")
			}

		} else {
			logError(ctx, s, fmt.Errorf("Secret is invalid, it is labeled with %s but neither %s nor %s label provided: %w", tokenSecretLabel, spiTokenNameLabel, providerUrlLabel, err), r, lg)
			continue
		}

		token := spi.Token{
			Username:    string(s.Data["userName"]),
			AccessToken: string(s.Data["tokenData"]),
		}

		logs.AuditLog(ctx).Info("manual token upload initiated", s.Namespace, accessToken.Name)
		err = r.TokenStorage.Store(ctx, &accessToken, &token)
		if err != nil {
			logError(ctx, s, fmt.Errorf("token storing failed: %w", err), r, lg)
			logs.AuditLog(ctx).Error(err, "manual token upload failed", s.Namespace, accessToken.Name)
			continue
		}
		logs.AuditLog(ctx).Info("manual token upload completed", s.Namespace, accessToken.Name)

		tryDeleteEvent(ctx, s.Name, req.Namespace, r, lg)

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

	lg.V(logs.DebugLevel).Info("Error Event for Secret: " + secret.Name + " " + err.Error())

	tryDeleteEvent(ctx, secret.Name, secret.Namespace, r, lg)

	secretErrEvent := &corev1.Event{}
	secretErrEvent.Name = secret.Name
	secretErrEvent.Message = err.Error()
	secretErrEvent.Namespace = secret.Namespace
	secretErrEvent.Reason = "Can not upload access token"
	secretErrEvent.InvolvedObject = corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion}
	secretErrEvent.Type = "Error"
	secretErrEvent.LastTimestamp = metav1.NewTime(time.Now())

	err1 := r.Create(ctx, secretErrEvent)

	if err1 != nil {
		lg.Error(err1, "Event creation failed for Secret: "+secret.Name)
	}

	lg.Error(err, "Secret upload failed:")
}

func tryDeleteEvent(ctx context.Context, secretName string, ns string, r *TokenUploadReconciler, lg logr.Logger) {
	stored := &corev1.Event{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, stored)

	if err == nil {
		lg.V(logs.DebugLevel).Info("event Found and will be deleted: " + stored.Name)
		err = r.Delete(ctx, stored)
		if err != nil {
			lg.Error(err, " can not delete Event ")
		}
	}
}

func findTokenByUrl(ctx context.Context, url string, ns string, r *TokenUploadReconciler, lg logr.Logger) *spi.SPIAccessToken {

	list := spi.SPIAccessTokenList{}
	err := r.List(ctx, &list, client.InNamespace(ns))
	if err != nil {
		lg.Error(err, "Can not get list of tokens ")
		return nil
	}

	for _, t := range list.Items {

		if t.Spec.ServiceProviderUrl == url {
			return &t
		}
	}
	return nil
}
