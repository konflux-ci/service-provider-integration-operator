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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	oauthCfgSecretFieldClientId     = "clientId"
	oauthCfgSecretFieldClientSecret = "clientSecret"
	oauthCfgSecretFieldAuthUrl      = "authUrl"
	oauthCfgSecretFieldTokenUrl     = "tokenUrl"
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

	spUrl, urlParseErr := url.Parse(info.ServiceProviderUrl)
	if urlParseErr != nil {
		return nil, fmt.Errorf("failed to parse serviceprovider url: %w", urlParseErr)
	}

	defaultOauthConfig, foundDefaultOauthConfig := c.ServiceProviderInstance[spUrl.Host]

	oauthCfg := &oauth2.Config{
		RedirectURL: c.redirectUrl(),
	}
	if foundDefaultOauthConfig {
		oauthCfg.Endpoint = defaultOauthConfig.Endpoint
	} else {
		// guess oauth endpoint urls now. It will be overwritten later if user oauth config secret has the values
		oauthCfg.Endpoint = createDefaultEndpoint(info.ServiceProviderUrl)
	}

	found, oauthCfgSecret, findErr := config.FindServiceProviderConfigSecret(ctx, c.K8sClient, info.TokenNamespace, spUrl.Host, c.ServiceProviderType)
	if findErr != nil {
		return nil, findErr
	}

	if found {
		if createOauthCfgErr := initializeConfigFromSecret(oauthCfgSecret, oauthCfg); createOauthCfgErr == nil {
			lg.V(logs.DebugLevel).Info("using custom user oauth config")
			return oauthCfg, nil
		} else {
			return nil, fmt.Errorf("failed to create oauth config from the secret: %w", createOauthCfgErr)
		}
	}

	if foundDefaultOauthConfig {
		lg.V(logs.DebugLevel).Info("using default oauth config")
		oauthCfg.ClientID = defaultOauthConfig.Config.ClientId
		oauthCfg.ClientSecret = defaultOauthConfig.Config.ClientSecret
		return oauthCfg, nil
	}

	return nil, fmt.Errorf("%w '%s' url: '%s'", errUnknownServiceProvider, info.ServiceProviderType, info.ServiceProviderUrl)
}

func initializeConfigFromSecret(secret *corev1.Secret, oauthCfg *oauth2.Config) error {
	if clientId, has := secret.Data[oauthCfgSecretFieldClientId]; has {
		oauthCfg.ClientID = string(clientId)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientId': %w", secret.Namespace, secret.Name, errMissingField)
	}

	if clientSecret, has := secret.Data[oauthCfgSecretFieldClientSecret]; has {
		oauthCfg.ClientSecret = string(clientSecret)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientSecret': %w", secret.Namespace, secret.Name, errMissingField)
	}

	if authUrl, has := secret.Data[oauthCfgSecretFieldAuthUrl]; has && len(authUrl) > 0 {
		oauthCfg.Endpoint.AuthURL = string(authUrl)
	}

	if tokenUrl, has := secret.Data[oauthCfgSecretFieldTokenUrl]; has && len(tokenUrl) > 0 {
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
