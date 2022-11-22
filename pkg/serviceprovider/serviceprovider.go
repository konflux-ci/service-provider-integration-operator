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
func (f *Factory) FromRepoUrl(ctx context.Context, repoUrl string) (ServiceProvider, error) {
	lg := log.FromContext(ctx)
	// this method is ready for multiple instances of some service provider configured with different base urls.
	// currently, we don't have any like that though :)
	tryInitialize := func(initializer Initializer) ServiceProvider {
		probe := initializer.Probe
		ctor := initializer.Constructor
		if probe == nil || ctor == nil {
			return nil
		}

		baseUrl, err := probe.Examine(f.HttpClient, repoUrl)
		if err != nil {
			return nil
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
	for _, spc := range f.Configuration.ServiceProviders {
		initializer, ok := f.Initializers[spc.ServiceProviderType]
		if !ok {
			continue
		}
		if sp := tryInitialize(initializer); sp != nil {
			return sp, nil
		}
	}

	for _, initializer := range f.Initializers {
		if initializer.SupportsManualUploadOnlyMode {
			if sp := tryInitialize(initializer); sp != nil {
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
