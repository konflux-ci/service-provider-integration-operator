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
	"time"

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

const tokenSeretLabel = "spi.appstudio.redhat.com/token"

// TokenUploadReconciler reconciles a Secret object
type TokenUploadReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	TokenStorage tokenstorage.NotifyingTokenStorage
}

func (r *TokenUploadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = log.FromContext(ctx)

	lg := log.FromContext(ctx)

	list := &corev1.SecretList{}

	err := r.List(context.TODO(), list, client.HasLabels{tokenSeretLabel})
	if err != nil {
		lg.Error(err, "Can not get list of secrets ")
		return ctrl.Result{}, err
	}

	for _, s := range list.Items {

		var accessToken spi.SPIAccessToken

		// we immediatelly delete the Secret
		err := r.Delete(context.TODO(), &s)
		if err != nil {
			logError(s, err, r, "can not delete the Secret ", lg)
			return ctrl.Result{}, err
		}

		// if spiTokenName field is not empty - try to find SPIAccessToken by it
		if len(s.Data["spiTokenName"]) > 0 {
			accessToken = spi.SPIAccessToken{}
			err = r.Get(context.TODO(), types.NamespacedName{Name: string(s.Data["spiTokenName"]), Namespace: s.Namespace}, &accessToken)

			if err != nil {
				logError(s, err, r, "can not find SPI access token "+string(s.Data["spiTokenName"]), lg)
				return ctrl.Result{}, err
			} else {
				lg.Info("SPI Access Token found : " + accessToken.Name)
			}
			// spiTokenName field is empty
			// check providerUrl field and if not empty - try to find the token for this provider instance

		} else if len(s.Data["providerUrl"]) > 0 {

			// NOTE: it does not fit advanced policy of matching token!
			// Do we need it as an SPI "API function" which take into account this policy?
			tkn := findTokenByUrl(string(s.Data["providerUrl"]), r, lg)
			// create new SPIAccessToken if there are no for such provider instance (URL)
			if tkn == nil {

				lg.Info("can not find SPI access token trying to create new one")
				accessToken = spi.SPIAccessToken{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "generated-spi-access-token-",
						Namespace:    s.Namespace,
					},
					Spec: spi.SPIAccessTokenSpec{
						ServiceProviderUrl: string(s.Data["providerUrl"]),
					},
				}
				err = r.Create(context.TODO(), &accessToken)
				if err != nil {
					logError(s, err, r, " can not create SPI access token for "+string(s.Data["providerUrl"]), lg)
					return ctrl.Result{}, err
				} else {
					// this is the only place where we can get the name of just created SPIAccessToken
					// which is presumably OK since SPI (binding) controller will look for the token by type/URL ?
					lg.Info("SPI Access Token created : " + accessToken.Name)
				}
			} else {
				accessToken = *tkn
				lg.Info("SPI Access Token found by providerUrl : " + accessToken.Name)
			}

		} else {
			logError(s, err, r, "Secret is invalid, neither spiTokenName nor providerUrl key found", lg)
			return ctrl.Result{}, err
		}

		token := spi.Token{
			Username:    string(s.Data["userName"]),
			AccessToken: string(s.Data["tokenData"]),
		}

		err = r.TokenStorage.Store(ctx, &accessToken, &token)
		if err != nil {
			logError(s, err, r, "store failed ", lg)
			return ctrl.Result{}, err
		}

		tryDeleteEvent(s.Name, req.Namespace, r, lg)

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenUploadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(createTokenPredicate()).
		Complete(r)
}

func createTokenPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.GetLabels()[tokenSeretLabel]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

func logError(secret corev1.Secret, err error, r *TokenUploadReconciler, msg string, lg logr.Logger) {

	lg.Info("Error Event for Secret: " + secret.Name + " " + msg)

	tryDeleteEvent(secret.Name, secret.Namespace, r, lg)

	if err != nil {
		secretErrEvent := &corev1.Event{}
		secretErrEvent.Name = secret.Name
		secretErrEvent.Message = msg
		secretErrEvent.Namespace = secret.Namespace
		secretErrEvent.Reason = "Can not upload access token"
		//secretErrEvent.Source = corev1.EventSource{}
		secretErrEvent.InvolvedObject = corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion}
		secretErrEvent.Type = "Error"
		//secretErrEvent.EventTime = metav1.NewTime(Now())
		secretErrEvent.LastTimestamp = metav1.NewTime(time.Now())

		err1 := r.Create(context.TODO(), secretErrEvent)

		log.Log.Error(err, msg)

		if err1 != nil {
			lg.Error(err1, "Event creation failed ")
		}
	}

}

func tryDeleteEvent(secretName string, ns string, r *TokenUploadReconciler, lg logr.Logger) {
	stored := &corev1.Event{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: ns}, stored)

	if err == nil {

		lg.Info("event Found and will be deleted: " + stored.Name)
		err = r.Delete(context.TODO(), stored)
		if err != nil {
			lg.Error(err, " can not delete Event ")
		}

	}
}

func findTokenByUrl(url string, r *TokenUploadReconciler, lg logr.Logger) *spi.SPIAccessToken {

	list := spi.SPIAccessTokenList{}
	err := r.List(context.TODO(), &list)
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
