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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceProvider interface {
	LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	GetBaseUrl() string
	TranslateToScopes(permission api.Permission) []string
	GetType() api.ServiceProviderType
	GetOAuthEndpoint() string
}

var allProbes = map[config.ServiceProviderType]serviceProviderProbe{
	config.ServiceProviderTypeGitHub: githubProbe{},
	config.ServiceProviderTypeQuay:   quayProbe{},
}

type Factory struct {
	Configuration config.Configuration
	Client        *http.Client
}

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

func TypeFromURL(url string) (api.ServiceProviderType, error) {
	if strings.HasPrefix(url, "https://github.com") {
		return api.ServiceProviderTypeGitHub, nil
	} else if strings.HasPrefix(url, "https://quay.io") {
		return api.ServiceProviderTypeQuay, nil
	}

	return "", fmt.Errorf("no service provider found for url: %s", url)
}

func GetAllScopes(sp ServiceProvider, perms *api.Permissions) []string {
	allScopes := make([]string, len(perms.AdditionalScopes)+len(perms.Required))

	allScopes = append(allScopes, perms.AdditionalScopes...)

	for _, p := range perms.Required {
		allScopes = append(allScopes, sp.TranslateToScopes(p)...)
	}

	return allScopes
}
