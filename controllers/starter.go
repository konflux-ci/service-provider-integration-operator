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
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	sconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func SetupAllReconcilers(mgr controllerruntime.Manager, cfg *config.OperatorConfiguration, ts tokenstorage.TokenStorage, initializers map[sconfig.ServiceProviderType]serviceprovider.Initializer) error {
	spf := serviceprovider.Factory{
		Configuration:    cfg,
		KubernetesClient: mgr.GetClient(),
		HttpClient:       &http.Client{Transport: httptransport.HttpMetricCollectingRoundTripper{RoundTripper: http.DefaultTransport}},
		Initializers:     initializers,
		TokenStorage:     ts,
	}

	var err error

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
		K8sClient:  mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		HttpClient: spf.HttpClient,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}
