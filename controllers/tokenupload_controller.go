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
	stdErrors "errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"sigs.k8s.io/controller-runtime/pkg/builder"
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
	tokenSecretLabel           = "spi.appstudio.redhat.com/upload-secret" //#nosec G101 -- false positive, this is not a token
	spiTokenNameField          = "spiTokenName"                           //#nosec G101 -- false positive, this is not a token
	providerUrlField           = "providerUrl"
	remoteSecretNameAnnotation = "spi.appstudio.redhat.com/remote-secret-name" //#nosec G101 -- false positive, this is not a token
	targetTypeAnnotation       = "spi.appstudio.redhat.com/remote-secret-target-type"
	targetNameAnnotation       = "spi.appstudio.redhat.com/remote-secret-target-name"
)

var (
	targetTypeNotSetError = stdErrors.New("target type not set")
	targetNameNotSetError = stdErrors.New("target name not set")
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
	lg.V(logs.DebugLevel).Info("starting reconciliation")
	defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile Upload Secret")

	uploadSecret := &corev1.Secret{}

	if err := r.Get(ctx, req.NamespacedName, uploadSecret); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("upload secret already gone from the cluster. skipping reconciliation")
		}
		return ctrl.Result{}, fmt.Errorf("failed to load the upload secret from the cluster: %w", err)
	}

	if uploadSecret.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("upload secret is being deleted. skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// we immediately delete the Secret
	err := r.Delete(ctx, uploadSecret)
	if err != nil {
		// We failed to delete the secret, so we error out so that we can try again in the next reconciliation round.
		// We therefore also DON'T create the error event in this case like we do later on in this method.
		return ctrl.Result{}, fmt.Errorf("cannot delete the Secret: %w", err)
	}

	// NOTE: it is useless to return any error to the controller runtime after this point, because we just
	// deleted the secret that is being reconciled and so its repeated reconciliation would short-circuit
	// on the non-nil DeletionTimestamp. Therefore, in case of errors, we just create the error event and
	// return a "success" to the controller runtime.

	secretType := uploadSecret.GetLabels()[tokenSecretLabel]
	switch secretType {
	case "token":
		err = r.reconcileToken(ctx, uploadSecret)
	case "secret":
		err = r.reconcileRemoteSecret(ctx, uploadSecret)
	}

	if err != nil {
		r.createErrorEvent(ctx, uploadSecret, err, lg)
	} else {
		r.tryDeleteEvent(ctx, uploadSecret.Name, req.Namespace, lg)
	}

	return ctrl.Result{}, nil
}

func (r *TokenUploadReconciler) reconcileToken(ctx context.Context, uploadSecret *corev1.Secret) error {
	lg := log.FromContext(ctx)
	// try to find the SPIAccessToken
	accessToken, err := r.findSpiAccessToken(ctx, uploadSecret, lg)
	if err != nil {
		return fmt.Errorf("cannot find SPI access token: %w", err)
	} else if accessToken == nil {
		// SPIAccessToken does not exist, so create it
		accessToken, err = r.createSpiAccessToken(ctx, uploadSecret, lg)
		if err != nil {
			return fmt.Errorf("can not create SPI access token: %w ", err)
		}
	}

	token := spi.Token{
		Username:    string(uploadSecret.Data["userName"]),
		AccessToken: string(uploadSecret.Data["tokenData"]),
	}

	auditLog := logs.AuditLog(ctx).WithValues("SPIAccessToken.name", accessToken.Name)

	auditLog.Info("manual token upload initiated", "action", "UPDATE")
	// Upload Token, it will cause update SPIAccessToken State as well
	err = r.TokenStorage.Store(ctx, accessToken, &token)
	if err != nil {
		err = fmt.Errorf("failed to store the token: %w", err)
		auditLog.Error(err, "manual token upload failed")
		return err
	}
	auditLog.Info("manual token upload completed")

	return nil
}

func (r *TokenUploadReconciler) reconcileRemoteSecret(ctx context.Context, uploadSecret *corev1.Secret) error {
	lg := log.FromContext(ctx)
	// try to find  RemoteSecret
	remoteSecret, err := r.findRemoteSecret(ctx, uploadSecret, lg)
	if err != nil {
		return fmt.Errorf("can not find RemoteSecret: %w ", err)
	} else if remoteSecret == nil {
		// SPIAccessToken does not exist, so create it
		remoteSecret, err = r.createRemoteSecret(ctx, uploadSecret, lg)
		if err != nil {
			return fmt.Errorf("can not create RemoteSecret: %w ", err)
		}
	}

	auditLog := logs.AuditLog(ctx).WithValues("remoteSecretName", remoteSecret.Name)
	auditLog.Info("manual secret upload initiated")
	auditLog.Info("manual secret upload completed")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenUploadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      tokenSecretLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct the predicate for matching secrets. This should not happen: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(pred)).
		Complete(r); err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
		return err
	}
	return nil
}

// reports error in both Log and Event in current Namespace
func (r *TokenUploadReconciler) createErrorEvent(ctx context.Context, secret *corev1.Secret, err error, lg logr.Logger) {
	r.tryDeleteEvent(ctx, secret.Name, secret.Namespace, lg)

	secretErrEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Message:        err.Error(),
		Reason:         "cannot process upload secret",
		InvolvedObject: corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion},
		Type:           "Error",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}

	err = r.Create(ctx, secretErrEvent)
	if err != nil {
		lg.Error(err, "event creation failed for upload secret")
	}

}

// Contract: having exactly one event if upload failed and no events if uploaded.
// For this need to delete the event every attempt
func (r *TokenUploadReconciler) tryDeleteEvent(ctx context.Context, secretName string, ns string, lg logr.Logger) {
	stored := &corev1.Event{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, stored)

	if err == nil {
		lg.V(logs.DebugLevel).Info("event found and will be deleted", "event.name", stored.Name)
		err = r.Delete(ctx, stored)
		if err != nil {
			lg.Error(err, "can not delete event")
		}
	}
}

func (r *TokenUploadReconciler) findSpiAccessToken(ctx context.Context, uploadSecret *corev1.Secret, lg logr.Logger) (*spi.SPIAccessToken, error) {
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

func (r *TokenUploadReconciler) createSpiAccessToken(ctx context.Context, uploadSecret *corev1.Secret, lg logr.Logger) (*spi.SPIAccessToken, error) {
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

func (r *TokenUploadReconciler) findRemoteSecret(ctx context.Context, uploadSecret *corev1.Secret, lg logr.Logger) (*spi.RemoteSecret, error) {
	remoteSecretName := uploadSecret.Annotations[remoteSecretNameAnnotation]
	if remoteSecretName == "" {
		lg.V(logs.DebugLevel).Info("No remoteSecretName found, will try to create with generated ")
		return nil, nil
	}

	remoteSecret := spi.RemoteSecret{}
	err := r.Get(ctx, types.NamespacedName{Name: remoteSecretName, Namespace: uploadSecret.Namespace}, &remoteSecret)

	if err != nil {
		if kuberrors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("RemoteSecret NOT found, will try to create  ", "RemoteSecret.name", remoteSecret.Name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("can not find RemoteSecret %s: %w ", remoteSecretName, err)
		}
	} else {
		lg.V(logs.DebugLevel).Info("RemoteSecret found : ", "RemoteSecret.name", remoteSecret.Name)
		return &remoteSecret, nil
	}
}

func (r *TokenUploadReconciler) createRemoteSecret(ctx context.Context, uploadSecret *corev1.Secret, lg logr.Logger) (*spi.RemoteSecret, error) {
	targetType, ok := uploadSecret.Annotations[targetTypeAnnotation]
	if !ok {
		return nil, targetTypeNotSetError
	}
	targetName, ok := uploadSecret.Annotations[targetNameAnnotation]
	if !ok {
		return nil, targetNameNotSetError
	}
	remoteSecretName := uploadSecret.Annotations[remoteSecretNameAnnotation]

	targetSpec := spi.RemoteSecretTarget{}
	switch targetType {
	case "namespace":
		targetSpec.Namespace = targetName
	case "environment":
		targetSpec.Environment = targetName
	}

	remoteSecret := spi.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: uploadSecret.Namespace,
		},
		Spec: spi.RemoteSecretSpec{
			Target: targetSpec,
		},
	}

	if remoteSecretName == "" {
		remoteSecret.GenerateName = "generated-"
	} else {
		remoteSecret.Name = remoteSecretName
	}

	err := r.Create(ctx, &remoteSecret)
	if err == nil {
		lg.V(logs.DebugLevel).Info("RemoteSecret created : ", "RemoteSecret.name", remoteSecret.Name, "targetType", targetType, "targetName", targetName)
		return &remoteSecret, nil
	} else {
		return nil, fmt.Errorf("can not create RemoteSecret %s. Reason: %w", remoteSecret.Name, err)
	}
}
