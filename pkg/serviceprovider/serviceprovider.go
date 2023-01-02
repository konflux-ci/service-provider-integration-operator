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

	// OAuthScopesFor translates all the permissions into a list of service-provider-specific scopes. This method
	// is used to compose the OAuth flow URL. There is a generic helper, GetAllScopes, that can be used if all that is
	// needed is just a translation of permissions into scopes.
	OAuthScopesFor(permissions *api.Permissions) []string

	// GetType merely returns the type of the service provider this instance talks to.
	GetType() api.ServiceProviderType

	CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)

	// GetOAuthEndpoint returns the URL of the OAuth initiation. This must point to the SPI oauth service, NOT
	//the service provider itself.
	GetOAuthEndpoint() string

	// GetDownloadFileCapability returns capability object for the providers which are able to download files from the repository
	// or nil for those which are not
	GetDownloadFileCapability() DownloadFileCapability

	// MapToken creates an access token mapper for given binding and token using the service-provider specific data.
	// The implementations can use the DefaultMapToken method if they don't use any custom logic.
	MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (AccessTokenMapper, error)

	// Validate checks that the provided object (token or binding) is valid in this service provider
	Validate(ctx context.Context, validated Validated) (ValidationResult, error)

	RefreshToken(ctx context.Context, token *api.Token, clientId string, clientSecret string) (*api.Token, error)
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
	// this method is ready for multiple instances of some service provider configured with different base urls.
	// currently, we don't have any like that though :)

	serviceProvidersBaseUrls, err := f.getBaseUrlsFromConfigs(ctx, namespace)
	if err != nil {
		return nil, err
	}

	for _, spc := range f.Configuration.ServiceProviders {
		initializer, ok := f.Initializers[spc.ServiceProviderType]
		if !ok {
			continue
		}
		if sp := f.initializeServiceProvider(initializer, repoUrl, serviceProvidersBaseUrls[spc.ServiceProviderType]); sp != nil {
			return sp, nil
		}
	}

	for serviceProviderType, initializer := range f.Initializers {
		if initializer.SupportsManualUploadOnlyMode {
			if sp := f.initializeServiceProvider(initializer, repoUrl, serviceProvidersBaseUrls[serviceProviderType]); sp != nil {
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

// TODO: replace with GetAllServiceProviderConfigs
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

func (f *Factory) GetAllServiceProviderConfigs(ctx context.Context, namespace string) ([]config.ServiceProviderConfiguration, error) {
	configurations := make([]config.ServiceProviderConfiguration, len(f.Configuration.SharedConfiguration.ServiceProviders))
	copy(configurations, f.Configuration.SharedConfiguration.ServiceProviders)

	for _, spConfig := range configurations {
		if spConfig.ServiceProviderBaseUrl == "" {
			spConfig.ServiceProviderBaseUrl = f.Initializers[spConfig.ServiceProviderType].SaasBaseUrl
		}
	}

	secretList := &corev1.SecretList{}
	err := f.KubernetesClient.List(ctx, secretList, client.InNamespace(namespace), client.HasLabels{
		api.ServiceProviderHostLabel, api.ServiceProviderTypeLabel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list oauth config secrets: %w", err)
	}
	for _, secret := range secretList.Items {
		conf := config.ServiceProviderConfiguration{
			ClientId:               string(secret.Data["clientId"]),
			ClientSecret:           string(secret.Data["clientSecret"]),
			ServiceProviderType:    config.ServiceProviderType(secret.ObjectMeta.Labels[api.ServiceProviderTypeLabel]),
			ServiceProviderBaseUrl: secret.ObjectMeta.Labels[api.ServiceProviderHostLabel],
		}
		configurations = append(configurations, conf)
	}
	return configurations, nil
}

func (f *Factory) initializeServiceProvider(initializer Initializer, repoUrl string, baseUrlsForProvider []string) ServiceProvider {
	probe := initializer.Probe
	ctor := initializer.Constructor
	if probe == nil || ctor == nil {
		return nil
	}

	baseUrl := ""
	for _, providerBaseUrl := range baseUrlsForProvider {
		repoUrlTrimmed := strings.TrimPrefix(repoUrl, "https://")
		baseUrlTrimmed := strings.TrimPrefix(providerBaseUrl, "https://")
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
