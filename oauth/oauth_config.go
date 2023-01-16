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

package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	errMissingField           = errors.New("missing mandatory field in oauth configuration")
	errUnknownServiceProvider = errors.New("haven't found oauth configuration for service provider")
)

// obtainOauthConfig is responsible for getting oauth configuration of service provider.
// Currently, this can be configured with labeled secret living in namespace together with SPIAccessToken.
// If no such secret is found, global configuration of oauth service is used.
func (c *commonController) obtainOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
	lg := log.FromContext(ctx).WithValues("oauthInfo", info)

	lg.Info("we have these service providers", "sp", c.ServiceProviderConfigurations)

	spUrl, urlParseErr := url.Parse(info.ServiceProviderUrl)
	if urlParseErr != nil {
		return nil, fmt.Errorf("failed to parse serviceprovider url: %w", urlParseErr)
	}

	// first try to find configuration in user's secret
	noAuthCtx := WithAuthIntoContext("", ctx) // we want to use ServiceAccount to find the secret, so we need to use context without user's token
	if found, spCfgSecret, findErr :=
		config.FindUserServiceProviderConfigSecret(noAuthCtx, c.K8sClient, info.TokenNamespace, c.ServiceProviderType, spUrl.Host); findErr != nil {
		return nil, findErr
	} else if found {
		spConfig := config.CreateServiceProviderConfigurationFromSecret(spCfgSecret, info.ServiceProviderUrl, c.ServiceProviderType)
		if spConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("we found user's sp config, but it has no oauth information. This should not happend because we should not have the oauth url at all!")
		}
		oauthConfig := spConfig.OAuth2Config
		oauthConfig.RedirectURL = c.redirectUrl()
		return oauthConfig, nil
	}

	// if we don't have user's config, we
	globalSpConfig, foundGlobalSpConfig := c.ServiceProviderConfigurations[spUrl.Host]
	if foundGlobalSpConfig {
		if globalSpConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("we want to use global sp config, but it has no oauth information. This should not happend because we should not have the oauth url at all!")
		}
		oauthConfig := globalSpConfig.OAuth2Config
		oauthConfig.RedirectURL = c.redirectUrl()
		return oauthConfig, nil
	}

	return nil, fmt.Errorf("%w '%s' url: '%s'", errUnknownServiceProvider, info.ServiceProviderName, info.ServiceProviderUrl)
}

func initializeConfigFromSecret(secret *corev1.Secret, oauthCfg *oauth2.Config) error {
	if clientId, has := secret.Data[config.OAuthCfgSecretFieldClientId]; has {
		oauthCfg.ClientID = string(clientId)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientId': %w", secret.Namespace, secret.Name, errMissingField)
	}

	if clientSecret, has := secret.Data[config.OAuthCfgSecretFieldClientSecret]; has {
		oauthCfg.ClientSecret = string(clientSecret)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientSecret': %w", secret.Namespace, secret.Name, errMissingField)
	}

	if authUrl, has := secret.Data[config.OAuthCfgSecretFieldAuthUrl]; has && len(authUrl) > 0 {
		oauthCfg.Endpoint.AuthURL = string(authUrl)
	}

	if tokenUrl, has := secret.Data[config.OAuthCfgSecretFieldTokenUrl]; has && len(tokenUrl) > 0 {
		oauthCfg.Endpoint.TokenURL = string(tokenUrl)
	}

	return nil
}

func createDefaultEndpoint(spBaseUrl string) oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:  spBaseUrl + "/oauth/authorize",
		TokenURL: spBaseUrl + "/oauth/token",
	}
}
