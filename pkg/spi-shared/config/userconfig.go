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
	"errors"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	oauthCfgSecretFieldClientId     = "clientId"
	oauthCfgSecretFieldClientSecret = "clientSecret"
	oauthCfgSecretFieldAuthUrl      = "authUrl"
	oauthCfgSecretFieldTokenUrl     = "tokenUrl"
)

var (
	errMultipleMatchingSecrets = errors.New("found multiple matching oauth config secrets")
)

// findUserServiceProviderConfigSecret tries to find user's service provider configuration secret in given namespace based on labels.
// Secret must match `spi.appstudio.redhat.com/service-provider-type` label with given `ServiceProviderType`.
// For service providers running on non-default host, `spi.appstudio.redhat.com/service-provider-host` must match with given `spHost`.
// If `spi.appstudio.redhat.com/service-provider-host` is not set, we can still pick the secret for service providers running on default host (like `github.com`).
// Returned *Secret can be nil in cases we haven't find any matching secret.
// Error in cases when we:
//   - find multiple secrets that matches given parametes
//   - kubernetes request error
func findUserServiceProviderConfigSecret(ctx context.Context, k8sClient client.Client, tokenNamespace string, spType ServiceProviderType, spHost string) (*corev1.Secret, error) {
	lg := log.FromContext(ctx).WithValues("spHost", spHost)

	lg.V(logs.DebugLevel).Info("looking for sp configuration secrets", "namespace", tokenNamespace, "spname", spType.Name, "sphost", spHost)

	secrets := &corev1.SecretList{}
	if listErr := k8sClient.List(ctx, secrets, client.InNamespace(tokenNamespace), client.MatchingLabels{
		v1beta1.ServiceProviderTypeLabel: string(spType.Name),
	}); listErr != nil {
		if kuberrors.IsForbidden(listErr) {
			lg.Info("not enough permissions to list the secrets")
			return nil, nil
		} else if kuberrors.IsUnauthorized(listErr) {
			lg.Info("request is not authorized to list the secrets in user's namespace")
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
		}
	}

	lg.V(logs.DebugLevel).Info("found secrets with oauth configuration", "count", len(secrets.Items))

	if len(secrets.Items) < 1 {
		return nil, nil
	}

	var oauthSecretWithoutHost *corev1.Secret
	var oauthSecretWithHost *corev1.Secret

	// go through all found oauth secret configs
	for _, oauthSecret := range secrets.Items {
		// if we find one labeled for sp host, we take it
		if labelSpHost, hasLabel := oauthSecret.ObjectMeta.Labels[v1beta1.ServiceProviderHostLabel]; hasLabel {
			if labelSpHost == spHost {
				if oauthSecretWithHost != nil { // if we found one before, return error because we can't tell which one to use
					return nil, errMultipleMatchingSecrets
				}
				oauthSecretWithHost = oauthSecret.DeepCopy()
			}
		} else { // if we found one without host label we check if it matches with default sp host value, then we can save it for later
			if spHost != spType.DefaultHost {
				continue
			}
			if oauthSecretWithoutHost != nil { // if we found one before, return error because we can't tell which one to use
				return nil, errMultipleMatchingSecrets
			}
			oauthSecretWithoutHost = oauthSecret.DeepCopy()
		}
	}

	if oauthSecretWithHost != nil {
		return oauthSecretWithHost, nil
	} else if oauthSecretWithoutHost != nil {
		return oauthSecretWithoutHost, nil
	} else {
		return nil, nil
	}
}

// createServiceProviderConfigurationFromSecret creates `ServiceProviderConfiguration` of given `ServiceProviderType` with given `baseUrl`.
// It extracts data from given user configuration `Secret` and set it to `OAuth2Config` property of returned `ServiceProviderConfiguration`.
func createServiceProviderConfigurationFromSecret(configSecret *corev1.Secret, baseUrl string, spType ServiceProviderType) *ServiceProviderConfiguration {
	return &ServiceProviderConfiguration{
		ServiceProviderType:    spType,
		ServiceProviderBaseUrl: baseUrl,
		Extra:                  map[string]string{},
		OAuth2Config:           initializeOAuthConfigFromSecret(configSecret, spType),
	}
}

// initializeOAuthConfigFromSecret creates `oauth2.Config` from given `Secret`.
// In case Secret doesn't have both `clientId` and `clientSecret` keys set, we just return nil.
// Endpoint is initially set from given `ServiceProviderType` defaults, but can be overwritten with `authUrl` and `tokenUrl` Secret keys.
func initializeOAuthConfigFromSecret(secret *corev1.Secret, spType ServiceProviderType) *oauth2.Config {
	oauthCfg := &oauth2.Config{
		Endpoint: spType.DefaultOAuthEndpoint,
	}
	if clientId, has := secret.Data[oauthCfgSecretFieldClientId]; has {
		oauthCfg.ClientID = string(clientId)
	} else {
		// in case we don't have client id, we consider configuration to not have oauth
		return nil
	}

	if clientSecret, has := secret.Data[oauthCfgSecretFieldClientSecret]; has {
		oauthCfg.ClientSecret = string(clientSecret)
	} else {
		// in case we don't have client secret, we consider configuration to not have oauth
		return nil
	}

	if authUrl, has := secret.Data[oauthCfgSecretFieldAuthUrl]; has && len(authUrl) > 0 {
		oauthCfg.Endpoint.AuthURL = string(authUrl)
	}

	if tokenUrl, has := secret.Data[oauthCfgSecretFieldTokenUrl]; has && len(tokenUrl) > 0 {
		oauthCfg.Endpoint.TokenURL = string(tokenUrl)
	}

	return oauthCfg
}
