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
)

var (
	errNoOAuthConfiguration = errors.New("no OAuth configuration found for the service provider")
	errConfigNoOAuth        = errors.New("found sp configuration has no OAuth information. This should never happen, because Operator should find same configuration and thus not generate OAuth URL and we should not get to the OAuth service at all")
)

// obtainOauthConfig is responsible for getting oauth configuration of service provider.
// Currently, this can be configured with labeled secret living in namespace together with SPIAccessToken.
// If no such secret is found, global configuration of oauth service is used.
func (c *commonController) obtainOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
	spUrl, urlParseErr := url.Parse(info.ServiceProviderUrl)
	if urlParseErr != nil {
		return nil, fmt.Errorf("failed to parse serviceprovider url: %w", urlParseErr)
	}

	// first try to find configuration in user's secret
	spConfig, errSpConfig := config.SpConfigFromUserSecret(ctx, c.InClusterK8sClient, info.TokenNamespace, c.ServiceProviderType, spUrl)
	if errSpConfig != nil {
		return nil, fmt.Errorf("failed to create service provider config from user config secret: %w", errSpConfig)
	}

	if spConfig != nil {
		if spConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("user config error: %w", errConfigNoOAuth)
		}
		oauthConfig := spConfig.OAuth2Config
		oauthConfig.RedirectURL = c.redirectUrl()
		return oauthConfig, nil
	}

	// if we don't have user's config, we
	if globalSpConfig := config.SpConfigFromGlobalConfig(&c.SharedConfiguration, c.ServiceProviderType, config.GetBaseUrl(spUrl)); globalSpConfig != nil {
		if globalSpConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("global config error: %w", errConfigNoOAuth)
		}
		oauthConfig := globalSpConfig.OAuth2Config
		oauthConfig.RedirectURL = c.redirectUrl()
		return oauthConfig, nil
	}

	return nil, fmt.Errorf("%w '%s' url: '%s'", errNoOAuthConfiguration, info.ServiceProviderName, info.ServiceProviderUrl)
}
