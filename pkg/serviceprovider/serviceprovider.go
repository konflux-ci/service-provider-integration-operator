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

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
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

	// TranslateToScopes translates the provided permission object into (a set of) service-provider-specific scopes.
	TranslateToScopes(permission api.Permission) []string

	// GetType merely returns the type of the service provider this instance talks to.
	GetType() api.ServiceProviderType

	CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)

	// GetOAuthEndpoint returns the URL of the OAuth initiation. This must point to the SPI oauth service, NOT
	//the service provider itself.
	GetOAuthEndpoint() string
}

// Factory is able to construct service providers from repository URLs.
type Factory struct {
	Configuration    config.Configuration
	KubernetesClient client.Client
	HttpClient       *http.Client
	Initializers     map[config.ServiceProviderType]Initializer
	TokenStorage     tokenstorage.TokenStorage
}

// FromRepoUrl returns the service provider instance able to talk to the repository on the provided URL.
func (f *Factory) FromRepoUrl(repoUrl string) (ServiceProvider, error) {
	// this method is ready for multiple instances of some service provider configured with different base urls.
	// currently, we don't have any like that though :)
	for _, spc := range f.Configuration.ServiceProviders {
		initializer, ok := f.Initializers[spc.ServiceProviderType]
		if !ok {
			continue
		}

		probe := initializer.Probe
		ctor := initializer.Constructor
		if probe == nil || ctor == nil {
			continue
		}

		baseUrl, err := probe.Examine(f.HttpClient, repoUrl)
		if err != nil {
			continue
		}

		if baseUrl != "" {
			sp, err := ctor.Construct(f, baseUrl)
			if err != nil {
				continue
			}

			return sp, nil
		}
	}

	return nil, fmt.Errorf("could not determine service provider for url: %s", repoUrl)
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
				return sperrors.FromHttpStatusCode(response.StatusCode)
			}),
		},
		CheckRedirect: cl.CheckRedirect,
		Jar:           cl.Jar,
		Timeout:       cl.Timeout,
	}
}

type Matchable interface {
	RepoUrl() string
	ObjNamespace() string
	Permissions() *api.Permissions
}

var _ Matchable = (*api.SPIAccessCheck)(nil)
var _ Matchable = (*api.SPIAccessTokenBinding)(nil)
