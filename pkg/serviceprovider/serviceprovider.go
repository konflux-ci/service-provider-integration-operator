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

package serviceprovider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PUBLIC_GITHUB_URL = "https://github.com"
const PUBLIC_QUAY_URL = "https://quay.io"
const PUBLIC_GITLAB_URL = "https://gitlab.com"

// ServiceProvider abstracts the interaction with some service provider
type ServiceProvider interface {
	// LookupToken tries to match an SPIAccessToken object with the requirements expressed in the provided binding.
	// This usually searches kubernetes (using the provided client) and the service provider itself (using some specific
	// mechanism (usually an http client)).
	LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)

	// PersistMetadata tries to use the OAuth access token associated with the provided token (if any) and persists any
	// state and metadata required for the token lookup. The metadata must be stored in the Status.TokenMetadata field
	// of the provided token.
	// Implementors should make sure that this method returns InvalidAccessTokenError if the reason for the failure is
	// an invalid token. This is important to distinguish between environmental errors and errors in the data itself.
	PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error

	// GetBaseUrl returns the base URL of the service provider this instance talks to. This info is saved with the
	// SPIAccessTokens so that later on, the OAuth service can use it to construct the OAuth flow URLs.
	GetBaseUrl() string

	// GetType merely returns the type of the service provider this instance talks to.
	GetType() api.ServiceProviderType

	CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)

	// GetDownloadFileCapability returns capability object for the providers which are able to download files from the repository
	// or nil for those which are not
	GetDownloadFileCapability() DownloadFileCapability

	GetOAuthCapability() OAuthCapability

	// MapToken creates an access token mapper for given binding and token using the service-provider specific data.
	// The implementations can use the DefaultMapToken method if they don't use any custom logic.
	MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (AccessTokenMapper, error)

	// Validate checks that the provided object (token or binding) is valid in this service provider
	Validate(ctx context.Context, validated Validated) (ValidationResult, error)
}

// ValidationResult represents the results of the ServiceProvider.Validate method.
type ValidationResult struct {
	// ScopeValidation is the reasons for the scopes and permissions to be invalid
	ScopeValidation []error
}

// Factory is able to construct service providers from repository URLs.
type Factory struct {
	Configuration    *opconfig.OperatorConfiguration
	KubernetesClient client.Client
	HttpClient       *http.Client
	Initializers     map[config.ServiceProviderType]Initializer
	TokenStorage     tokenstorage.TokenStorage
}

// FromRepoUrl returns the service provider instance able to talk to the repository on the provided URL.
func (f *Factory) FromRepoUrl(ctx context.Context, repoUrl string, namespace string) (ServiceProvider, error) {
	lg := log.FromContext(ctx)
	lg.Info("try to create sp from", "repoUrl", repoUrl)

	serviceProvidersConfigurations, err := f.getServiceProviderConfigurations(ctx, repoUrl, namespace)
	if err != nil {
		return nil, err
	}

	lg.Info("found", "sp configs", serviceProvidersConfigurations)

	for _, spc := range f.Configuration.ServiceProviders {
		initializer, ok := f.Initializers[spc.ServiceProviderType]
		if !ok {
			continue
		}
		if sp := f.initializeServiceProvider(initializer, repoUrl, serviceProvidersConfigurations[spc.ServiceProviderType]); sp != nil {
			return sp, nil
		}
	}

	for serviceProviderType, initializer := range f.Initializers {
		if initializer.SupportsManualUploadOnlyMode {
			if sp := f.initializeServiceProvider(initializer, repoUrl, serviceProvidersConfigurations[serviceProviderType]); sp != nil {
				return sp, nil
			}
		}
	}

	lg.Info("Specific provided is not found for given URL. General credentials provider will be used", "repositoryURL", repoUrl)
	hostCredentialsInitializer := f.Initializers[config.ServiceProviderTypeHostCredentials]
	hostCredentialsConstructor := hostCredentialsInitializer.Constructor
	hostCredentialProvider, err := hostCredentialsConstructor.Construct(f, repoUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to construct host credentials provider: %w", err)
	}
	return hostCredentialProvider, nil
}

func (f *Factory) getServiceProviderConfigurations(ctx context.Context, repoUrl string, namespace string) (map[config.ServiceProviderType][]config.ServiceProviderConfiguration, error) {
	// order of each array is important. Configurations must come in following order:
	// user config secret > global config file > defaults
	foundConfigurations := map[config.ServiceProviderType][]config.ServiceProviderConfiguration{}

	repoUrlTrimmed := strings.TrimPrefix(repoUrl, "https://")

	for _, spDefault := range config.SupportedServiceProvidersDefaults {
		spTypeConfigurations := []config.ServiceProviderConfiguration{}

		// first we need to find and create configurations from user secrets
		if userSecretConfigs, err := f.createConfigsFromUserConfigSecrets(ctx, spDefault, namespace); err == nil {
			spTypeConfigurations = append(spTypeConfigurations, userSecretConfigs...)
		} else {
			return nil, err
		}

		// then we take service providers from global configuration
		for _, sp := range f.Configuration.ServiceProviders {
			if sp.ServiceProviderType == spDefault.SpType && strings.HasPrefix(repoUrlTrimmed, sp.ServiceProviderBaseUrl) {
				spTypeConfigurations = append(spTypeConfigurations, sp)
			}
		}

		foundConfigurations[spDefault.SpType] = spTypeConfigurations
	}

	return foundConfigurations, nil
}

func (f *Factory) createConfigsFromUserConfigSecrets(ctx context.Context, spDefault config.ServiceProviderDefaults, namespace string) ([]config.ServiceProviderConfiguration, error) {
	spTypeConfigurations := []config.ServiceProviderConfiguration{}

	spConfigSecrets := &corev1.SecretList{}
	if listErr := f.KubernetesClient.List(ctx, spConfigSecrets, client.InNamespace(namespace), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(spDefault.SpType),
	}); listErr != nil {
		return nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
	}

	// we've found some sp configuration in user's secret
	if len(spConfigSecrets.Items) > 0 {
		for _, spConfigSecret := range spConfigSecrets.Items {
			newSpConfiguration := config.ServiceProviderConfiguration{
				ServiceProviderType:    spDefault.SpType,
				ServiceProviderBaseUrl: spDefault.UrlHost,
			}

			// having oauth configuration empty is ok for us here, because we don't need the values. We just need to know whether we have configuration for oauth
			oauthClientId, hasClientId := spConfigSecret.Data[config.OAuthCfgSecretFieldClientId]
			oauthClientSecret, hasClientSecret := spConfigSecret.Data[config.OAuthCfgSecretFieldClientSecret]
			if hasClientId && hasClientSecret {
				newSpConfiguration.Oauth2Config = &oauth2.Config{ClientID: string(oauthClientId), ClientSecret: string(oauthClientSecret)}
			}

			spTypeConfigurations = append(spTypeConfigurations, newSpConfiguration)
		}
	}

	return spTypeConfigurations, nil
}

// func (c *commonController) obtainOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
// 	lg := log.FromContext(ctx).WithValues("oauthInfo", info)

// 	spUrl, urlParseErr := url.Parse(info.ServiceProviderUrl)
// 	if urlParseErr != nil {
// 		return nil, fmt.Errorf("failed to parse serviceprovider url: %w", urlParseErr)
// 	}

// 	defaultOauthConfig, foundDefaultOauthConfig := c.ServiceProviderInstance[spUrl.Host]

// 	oauthCfg := &oauth2.Config{
// 		RedirectURL: c.redirectUrl(),
// 	}
// 	if foundDefaultOauthConfig {
// 		oauthCfg.Endpoint = defaultOauthConfig.Endpoint
// 	} else {
// 		// guess oauth endpoint urls now. It will be overwritten later if user oauth config secret has the values
// 		oauthCfg.Endpoint = createDefaultEndpoint(info.ServiceProviderUrl)
// 	}

// 	noAuthCtx := WithAuthIntoContext("", ctx) // we want to use ServiceAccount to find the secret, so we need to use context without user's token
// 	found, oauthCfgSecret, findErr := config.FindServiceProviderConfigSecret(noAuthCtx, c.K8sClient, info.TokenNamespace, spUrl.Host, c.ServiceProviderType)
// 	if findErr != nil {
// 		return nil, findErr
// 	}

// 	if found {
// 		if createOauthCfgErr := initializeConfigFromSecret(oauthCfgSecret, oauthCfg); createOauthCfgErr == nil {
// 			lg.V(logs.DebugLevel).Info("using custom user oauth config")
// 			return oauthCfg, nil
// 		} else {
// 			return nil, fmt.Errorf("failed to create oauth config from the secret: %w", createOauthCfgErr)
// 		}
// 	}

// 	if foundDefaultOauthConfig {
// 		lg.V(logs.DebugLevel).Info("using default oauth config")
// 		oauthCfg.ClientID = defaultOauthConfig.Config.Oauth2Config.ClientID
// 		oauthCfg.ClientSecret = defaultOauthConfig.Config.Oauth2Config.ClientSecret
// 		return oauthCfg, nil
// 	}

// 	return nil, fmt.Errorf("%w '%s' url: '%s'", errUnknownServiceProvider, info.ServiceProviderType, info.ServiceProviderUrl)
// }

func (f *Factory) getBaseUrlsFromConfigs(ctx context.Context, namespace string) (map[config.ServiceProviderType][]string, error) {
	secretList := &corev1.SecretList{}
	listErr := f.KubernetesClient.List(ctx, secretList, client.InNamespace(namespace), client.HasLabels{
		api.ServiceProviderTypeLabel,
	})
	if listErr != nil {
		return nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
	}

	// We need to add all known service provider urls as they might not be filled out in shared config.
	// In case they are, adding them does not cause an issue.
	allBaseUrls := make(map[config.ServiceProviderType][]string)
	allBaseUrls[config.ServiceProviderTypeGitHub] = []string{PUBLIC_GITHUB_URL}
	allBaseUrls[config.ServiceProviderTypeQuay] = []string{PUBLIC_QUAY_URL}
	allBaseUrls[config.ServiceProviderTypeGitLab] = []string{PUBLIC_GITLAB_URL}

	for _, spConfig := range f.Configuration.SharedConfiguration.ServiceProviders {
		if spConfig.ServiceProviderBaseUrl != "" {
			allBaseUrls[spConfig.ServiceProviderType] = append(allBaseUrls[spConfig.ServiceProviderType], spConfig.ServiceProviderBaseUrl)
		}
	}

	for _, secret := range secretList.Items {
		if baseUrl, hasLabel := secret.ObjectMeta.Labels[api.ServiceProviderHostLabel]; hasLabel {
			spType := config.ServiceProviderType(secret.ObjectMeta.Labels[api.ServiceProviderTypeLabel])
			allBaseUrls[spType] = append(allBaseUrls[spType], baseUrl)
		}
	}

	return allBaseUrls, nil
}

func (f *Factory) initializeServiceProvider(initializer Initializer, repoUrl string, serviceProviderConfigurations []config.ServiceProviderConfiguration) ServiceProvider {
	probe := initializer.Probe
	ctor := initializer.Constructor
	if probe == nil || ctor == nil {
		return nil
	}

	baseUrl := ""
	for _, spConfig := range serviceProviderConfigurations {
		repoUrlTrimmed := strings.TrimPrefix(repoUrl, "https://")
		baseUrlTrimmed := strings.TrimPrefix(spConfig.ServiceProviderBaseUrl, "https://")
		if strings.HasPrefix(repoUrlTrimmed, baseUrlTrimmed) {
			baseUrl = "https://" + baseUrlTrimmed
			break
		}
	}
	if baseUrl == "" {
		var err error
		if baseUrl, err = probe.Examine(f.HttpClient, repoUrl); err != nil {
			return nil
		}
	}

	if baseUrl != "" {
		sp, err := ctor.Construct(f, baseUrl)
		if err != nil {
			return nil
		}
		return sp
	}
	return nil
}

func AuthenticatingHttpClient(cl *http.Client) *http.Client {
	transport := cl.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &http.Client{
		Transport: httptransport.ExaminingRoundTripper{
			RoundTripper: httptransport.AuthenticatingRoundTripper{RoundTripper: transport},
			Examiner: httptransport.RoundTripExaminerFunc(func(request *http.Request, response *http.Response) error {
				return sperrors.FromHttpResponse(response) //nolint:wrapcheck // the users of the HTTP client are supposed to handle this error
			}),
		},
		CheckRedirect: cl.CheckRedirect,
		Jar:           cl.Jar,
		Timeout:       cl.Timeout,
	}
}

type Validated interface {
	Permissions() *api.Permissions
}

type Matchable interface {
	Validated
	RepoUrl() string
	ObjNamespace() string
}

var _ Matchable = (*api.SPIAccessCheck)(nil)
var _ Matchable = (*api.SPIAccessTokenBinding)(nil)

func DefaultMapToken(tokenObject *api.SPIAccessToken, tokenData *api.Token) AccessTokenMapper {
	var userId, userName string
	var scopes []string

	if tokenObject.Status.TokenMetadata != nil {
		userName = tokenObject.Status.TokenMetadata.Username
		userId = tokenObject.Status.TokenMetadata.UserId
		scopes = tokenObject.Status.TokenMetadata.Scopes
	}

	return AccessTokenMapper{
		Name:                    tokenObject.Name,
		Token:                   tokenData.AccessToken,
		ServiceProviderUrl:      tokenObject.Spec.ServiceProviderUrl,
		ServiceProviderUserName: userName,
		ServiceProviderUserId:   userId,
		UserId:                  "",
		ExpiredAfter:            &tokenData.Expiry,
		Scopes:                  scopes,
	}
}
