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
	"fmt"
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func SetupAllReconcilers(mgr controllerruntime.Manager, cfg *config.OperatorConfiguration, ts tokenstorage.TokenStorage, initializers *serviceprovider.Initializers) error {
	spf := serviceprovider.Factory{
		Configuration:    cfg,
		KubernetesClient: mgr.GetClient(),
		HttpClient:       &http.Client{Transport: httptransport.HttpMetricCollectingRoundTripper{RoundTripper: http.DefaultTransport}},
		Initializers:     initializers,
		TokenStorage:     ts,
	}

	var err error

	if err = serviceprovider.RegisterCommonMetrics(metrics.Registry); err != nil {
		return fmt.Errorf("failed to register the metrics with k8s metrics registry: %w", err)
	}

	if err = (&SPIAccessTokenReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           ts,
		ServiceProviderFactory: spf,
		Configuration:          cfg,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err = (&SPIAccessTokenBindingReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           ts,
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

	if cfg.EnableTokenUpload {
		// Setup tokenUpload controller if configured
		// Important: need NotifyingTokenStorage to reconcile related SPIAccessToken
		notifyingStorage := tokenstorage.NotifyingTokenStorage{
			ClientFactory: kubernetesclient.SingleInstanceClientFactory{
				Client: mgr.GetClient(),
			},
			TokenStorage: ts,
		}

		if err = (&TokenUploadReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			TokenStorage: notifyingStorage,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
	}

	return nil
}
