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
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceProvider abstracts the interaction with some service provider
type ServiceProvider interface {
	// LookupTokens tries to match an SPIAccessToken object with the requirements expressed in the provided binding.
	// This usually searches kubernetes (using the provided client) and the service provider itself (using some specific
	// mechanism (usually an http client)).
	LookupTokens(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error)

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
	GetType() config.ServiceProviderType

	CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)

	// GetDownloadFileCapability returns capability object for the providers which are able to download files from the repository
	// or nil for those which are not
	GetDownloadFileCapability() DownloadFileCapability

	// GetRefreshTokenCapability returns capability object for the providers which are able to refresh OAuth access tokens.
	// or nil
	GetRefreshTokenCapability() RefreshTokenCapability

	// GetOAuthCapability returns oauth capability of the service provider.
	// It can be null in case service provider don't support OAuth or it is not configured.
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
	Initializers     *Initializers
	TokenStorage     tokenstorage.TokenStorage
}

var (
	errNoConstructorImplemented = errors.New("service provider has no constructor")
	errNoInitializer            = errors.New("service provider has no initializer")
)

// FromRepoUrl returns the service provider instance able to talk to the repository on the provided URL.
func (f *Factory) FromRepoUrl(ctx context.Context, repoUrl string, namespace string) (ServiceProvider, error) {
	lg := log.FromContext(ctx)
	// this method is ready for multiple instances of some service provider configured with different base urls.
	// currently, we don't have any like that though :)

	parsedRepoUrl, errUrlParse := url.Parse(repoUrl)
	if errUrlParse != nil {
		return nil, fmt.Errorf("failed to parse repo url: %w", errUrlParse)
	}

	for _, sp := range config.SupportedServiceProviderTypes {
		var spConfig *config.ServiceProviderConfiguration
		var err error
		// first try to find configuration in secret
		if spConfig, err = config.SpConfigFromUserSecret(ctx, f.KubernetesClient, namespace, sp, parsedRepoUrl); err != nil {
			return nil, fmt.Errorf("failed to create service provider configuration from user secret: %w", err)
		} else if spConfig == nil { // then try to find it in global configuration
			spConfig = config.SpConfigFromGlobalConfig(&f.Configuration.SharedConfiguration, sp, config.GetBaseUrl(parsedRepoUrl))
		}

		// we try to initialize with what we have. if spConfig is nil, this function tries probe as last chance
		if sp, err := f.initializeServiceProvider(ctx, sp, spConfig, config.GetBaseUrl(parsedRepoUrl)); err != nil {
			return nil, err
		} else if sp != nil {
			return sp, nil
		}
	}

	lg.Info("Specific provider is not found for given URL. General credentials provider will be used", "repositoryURL", repoUrl)
	return f.createHostCredentialsProvider(parsedRepoUrl)
}

func (f *Factory) createHostCredentialsProvider(repoUrl *url.URL) (ServiceProvider, error) {
	hostCredentialsInitializer, errHostCredsInitializerFind := f.Initializers.GetInitializer(config.ServiceProviderTypeHostCredentials)
	if errHostCredsInitializerFind != nil {
		return nil, fmt.Errorf("initializer for host credentials service provider not found: %w", errHostCredsInitializerFind)
	}
	hostCredentialsConstructor := hostCredentialsInitializer.Constructor
	cfg, err := spConfigWithBaseUrl(config.ServiceProviderTypeHostCredentials, config.GetBaseUrl(repoUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to build service provider configuration from base URL: %w", err)
	}
	hostCredentialProvider, err := hostCredentialsConstructor.Construct(f, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to construct host credentials provider: %w", err)
	}
	return hostCredentialProvider, nil
}

// NewCacheWithExpirationPolicy returns a new metadata cache instance configured using this factory and the supplied
// expiration policy
func (f *Factory) NewCacheWithExpirationPolicy(policy MetadataExpirationPolicy) MetadataCache {
	return MetadataCache{
		Client:                    f.KubernetesClient,
		ExpirationPolicy:          policy,
		CacheServiceProviderState: f.Configuration.TokenMatchPolicy == opconfig.ExactTokenPolicy,
	}
}

func (f *Factory) initializeServiceProvider(_ context.Context, spType config.ServiceProviderType, spConfig *config.ServiceProviderConfiguration, repoBaseUrl string) (ServiceProvider, error) {
	initializer, errFindInitializer := f.Initializers.GetInitializer(spType)
	if errFindInitializer != nil {
		return nil, fmt.Errorf("failed to initialize service provider '%s': %w", spType.Name, errNoInitializer)
	}

	ctor := initializer.Constructor
	if ctor == nil {
		return nil, fmt.Errorf("service provider '%s': %w", spConfig.ServiceProviderType.Name, errNoConstructorImplemented)
	}

	if spConfig != nil {
		if err := config.ValidateStruct(spConfig); err != nil {
			return nil, fmt.Errorf("failed to create runtime configuration for service provider %s: %w", spType.Name, err)
		}
		sp, errConstructSp := ctor.Construct(f, spConfig)
		if errConstructSp != nil {
			return nil, fmt.Errorf("failed to construct service provider: %w", errConstructSp)
		}
		return sp, nil
	} else {
		if initializer.Probe != nil {
			probeBaseUrl, errProbe := initializer.Probe.Examine(f.HttpClient, repoBaseUrl)
			if errProbe != nil {
				// in current implementation of some probes, we have to consider probe error as not match state
				return nil, nil //nolint:nilerr
			}
			if probeBaseUrl != "" {
				cfg, err := spConfigWithBaseUrl(spType, probeBaseUrl)
				if err != nil {
					return nil, fmt.Errorf("failed to build service provider configuration from probe: %w", err)
				}
				sp, errConstructSp := ctor.Construct(f, cfg)
				if errConstructSp != nil {
					return nil, fmt.Errorf("failed to construct service provider after probing: %w", errConstructSp)
				}
				return sp, nil
			}
		}
	}

	return nil, nil
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

// spConfigWithBaseUrl simple helper function that creates `config.ServiceProviderConfiguration` of given `ServiceProviderType` with base url set by given `baseUrl`
func spConfigWithBaseUrl(spType config.ServiceProviderType, baseUrl string) (*config.ServiceProviderConfiguration, error) {
	cfg := &config.ServiceProviderConfiguration{
		ServiceProviderType:    spType,
		ServiceProviderBaseUrl: baseUrl,
	}
	if err := config.ValidateStruct(cfg); err != nil {
		return nil, fmt.Errorf("failed to validate service provider configuration: %w", err)
	}
	return cfg, nil
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
