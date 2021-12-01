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
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceProvider interface {
	client.Client
	LookupToken(ctx context.Context, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	GetServiceProviderUrlForRepo(repoUrl string) (string, error)
}

type urlMatcher func(string) bool

type serviceProviderSetup struct {
	matcher urlMatcher
	factory func(client.Client) (ServiceProvider, error)
}

var (
	githubUrlMatcher urlMatcher = func(url string) bool {
		return strings.HasPrefix(url, "https://github.com")
	}

	quayUrlMatcher urlMatcher = func(url string) bool {
		return strings.HasPrefix(url, "https://quay.io")
	}

	allKnownProviders = map[api.ServiceProviderType]serviceProviderSetup{
		api.ServiceProviderTypeGitHub: {
			matcher: githubUrlMatcher,
			factory: func(c client.Client) (ServiceProvider, error) {
				return &Github{Client: c}, nil
			},
		},
		api.ServiceProviderTypeQuay: {
			matcher: quayUrlMatcher,
			factory: func(c client.Client) (ServiceProvider, error) {
				return &Quay{Client: c}, nil
			},
		},
	}
)

func ByType(serviceProviderType api.ServiceProviderType, cl client.Client) (ServiceProvider, error) {
	if setup, ok := allKnownProviders[serviceProviderType]; ok {
		return setup.factory(cl)
	}

	return nil, fmt.Errorf("unknown service provider type: %s", serviceProviderType)
}

func FromURL(repoUrl string, cl client.Client) (ServiceProvider, error) {
	spType, err := TypeFromURL(repoUrl)
	if err != nil {
		return nil, err
	}

	return ByType(spType, cl)
}

func TypeFromURL(url string) (api.ServiceProviderType, error) {
	for spType, spSetup := range allKnownProviders {
		if spSetup.matcher(url) {
			return spType, nil
		}
	}

	return "", fmt.Errorf("no service provider found for url: %s", url)
}
