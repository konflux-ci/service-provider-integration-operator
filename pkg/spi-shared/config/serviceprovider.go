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

package config

import (
	"context"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SpConfigFromUserSecret(ctx context.Context, k8sClient client.Client, namespace string, spType ServiceProviderType, repoUrl *url.URL) (*ServiceProviderConfiguration, error) {
	// first try to find service provider configuration in user's secrets
	foundSecret, configSecret, findErr := findUserServiceProviderConfigSecret(ctx, k8sClient, namespace, spType, repoUrl.Host)
	if findErr != nil {
		return nil, findErr
	}
	if foundSecret {
		return createServiceProviderConfigurationFromSecret(configSecret, GetBaseUrl(repoUrl), spType), nil
	}
	return nil, nil
}

func SpConfigFromGlobalConfig(globalConfiguration *SharedConfiguration, spType ServiceProviderType, repoBaseUrl string) *ServiceProviderConfiguration {
	for _, configuredSp := range globalConfiguration.ServiceProviders {
		if configuredSp.ServiceProviderType.Name != spType.Name {
			continue
		}

		if configuredSp.ServiceProviderBaseUrl == repoBaseUrl {
			return &configuredSp
		}
	}

	if spType.DefaultBaseUrl == repoBaseUrl {
		return &ServiceProviderConfiguration{
			ServiceProviderType:    spType,
			ServiceProviderBaseUrl: spType.DefaultBaseUrl,
		}
	}

	return nil
}

func SpConfigWithBaseUrl(spType ServiceProviderType, baseUrl string) *ServiceProviderConfiguration {
	return &ServiceProviderConfiguration{
		ServiceProviderType:    spType,
		ServiceProviderBaseUrl: baseUrl,
	}
}
