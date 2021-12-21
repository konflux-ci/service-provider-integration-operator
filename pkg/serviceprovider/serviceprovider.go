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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceProvider abstracts the interaction with some service provider
type ServiceProvider interface {
	// LookupToken tries to match an SPIAccessToken object with the requirements expressed in the provided binding.
	// This usually searches kubernetes (using the provided client) and the service provider itself (using some specific
	// mechanism (usually an http client)).
	LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)

	// GetBaseUrl returns the base URL of the service provider this instance talks to. This info is saved with the
	// SPIAccessTokens so that later on, the OAuth service can use it to construct the OAuth flow URLs.
	GetBaseUrl() string

	// TranslateToScopes translates the provided permission object into (a set of) service-provider-specific scopes.
	TranslateToScopes(permission api.Permission) []string

	// GetType merely returns the type of the service provider this instance talks to.
	GetType() api.ServiceProviderType

	// GetOAuthEndpoint returns the URL of the OAuth initiation. This must point to the SPI oauth service, NOT
	//the service provider itself.
	GetOAuthEndpoint() string
}

// allProbes contains serviceProviderProbe instances for all known service provider types. This is used to determine
// what service provider should handle certain URL.
var allProbes = map[config.ServiceProviderType]serviceProviderProbe{
	config.ServiceProviderTypeGitHub: githubProbe{},
	config.ServiceProviderTypeQuay:   quayProbe{},
}

type Factory struct {
	Configuration config.Configuration
	Client        *http.Client
}

// FromRepoUrl returns the service provider instance able to talk to the repository on the provided URL.
func (f *Factory) FromRepoUrl(repoUrl string) (ServiceProvider, error) {
	// this method is ready for multiple instances of some service provider configured with different base urls.
	// currently, we don't have any like that though :)
	for _, spc := range f.Configuration.ServiceProviders {
		probe := allProbes[spc.ServiceProviderType]
		baseUrl, err := probe.Probe(f.Client, repoUrl)
		if err != nil {
			continue
		}
		if baseUrl != "" {
			switch spc.ServiceProviderType {
			case config.ServiceProviderTypeGitHub:
				return &Github{Configuration: f.Configuration}, nil
			case config.ServiceProviderTypeQuay:
				return &Quay{Configuration: f.Configuration}, nil
			}
		}
	}

	return nil, fmt.Errorf("could not determine service provider for url: %s", repoUrl)
}

// GetAllScopes is a helper method to translate all the provided permissions into a list of service-provided-specific
// scopes.
func GetAllScopes(sp ServiceProvider, perms *api.Permissions) []string {
	allScopes := make([]string, len(perms.AdditionalScopes)+len(perms.Required))

	allScopes = append(allScopes, perms.AdditionalScopes...)

	for _, p := range perms.Required {
		allScopes = append(allScopes, sp.TranslateToScopes(p)...)
	}

	return allScopes
}
