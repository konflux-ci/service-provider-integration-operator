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
	"fmt"
	"net/http"

	"github.com/redhat-appstudio/remote-secret/controllers/remotesecretstorage"

	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"

	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/redhat-appstudio/remote-secret/pkg/httptransport"
	rsecretstorage "github.com/redhat-appstudio/remote-secret/pkg/secretstorage"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func SetupAllReconcilers(mgr controllerruntime.Manager, cfg *config.OperatorConfiguration, secretStorage rsecretstorage.SecretStorage, initializers *serviceprovider.Initializers) error {
	ctx := context.Background()

	// note that calling the initialize method on the different storages constructed here is essentially useless
	// at the moment because they require an already initialized secret storage and the returned token
	// storage doesn't hold any state of its own at the moment. But let's stick to the protocol so that this
	// doesn't bite us in the future where we may add some state.

	tokenStorage := tokenstorage.NewJSONSerializingTokenStorage(secretStorage)
	if err := tokenStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize the token storage: %w", err)
	}

	remoteSecretStorage := remotesecretstorage.NewJSONSerializingRemoteSecretStorage(secretStorage)
	if err := remoteSecretStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize the remote secret storage: %w", err)
	}

	spf := serviceprovider.Factory{
		Configuration:    cfg,
		KubernetesClient: mgr.GetClient(),
		HttpClient:       &http.Client{Transport: httptransport.HttpMetricCollectingRoundTripper{RoundTripper: http.DefaultTransport}},
		Initializers:     initializers,
		TokenStorage:     tokenStorage,
	}

	var err error

	if err = serviceprovider.RegisterCommonMetrics(metrics.Registry); err != nil {
		return fmt.Errorf("failed to register the metrics with k8s metrics registry: %w", err)
	}

	if err = (&SPIAccessTokenReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           tokenStorage,
		ServiceProviderFactory: spf,
		Configuration:          cfg,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err = (&SPIAccessTokenBindingReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           tokenStorage,
		ServiceProviderFactory: spf,
		Configuration:          cfg,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err = (&SPIAccessTokenDataUpdateReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err = (&SPIAccessCheckReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		ServiceProviderFactory: spf,
		Configuration:          cfg,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err = (&SPIFileContentRequestReconciler{
		K8sClient:              mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		HttpClient:             spf.HttpClient,
		ServiceProviderFactory: spf,
		Configuration:          cfg,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if cfg.EnableRemoteSecrets {

		if err = (&SnapshotEnvironmentBindingReconciler{
			k8sClient:     mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
			Configuration: cfg,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
	}

	if cfg.EnableTokenUpload {
		// Setup tokenUpload controller if configured
		// Important: need NotifyingTokenStorage to reconcile related SPIAccessToken and RemoteSecret
		notifTokenStorage := tokenstorage.NewJSONSerializingTokenStorage(&secretstorage.NotifyingSecretStorage{
			ClientFactory: kubernetesclient.SingleInstanceClientFactory{
				Client: mgr.GetClient(),
			},
			SecretStorage: secretStorage,
			Group:         api.GroupVersion.Group,
			Kind:          "SPIAccessToken",
		})
		if err = notifTokenStorage.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize the notifying token storage: %w", err)
		}

		notifRemoteSecretStorage := remotesecretstorage.NewJSONSerializingRemoteSecretStorage(&secretstorage.NotifyingSecretStorage{
			ClientFactory: kubernetesclient.SingleInstanceClientFactory{
				Client: mgr.GetClient(),
			},
			SecretStorage: secretStorage,
			Group:         api.GroupVersion.Group,
			Kind:          "RemoteSecret",
		})
		if err = notifRemoteSecretStorage.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize the notifying remote secret storage: %w", err)
		}

		if err = (&TokenUploadReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			TokenStorage: notifTokenStorage,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
	}

	return nil
}
