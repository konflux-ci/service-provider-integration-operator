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
	LookupToken(ctx context.Context, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	GetServiceProviderUrlForRepo(repoUrl string) (string, error)
}

func ByType(serviceProviderType api.ServiceProviderType, cl client.Client) (ServiceProvider, error) {
	switch serviceProviderType {
	case api.ServiceProviderTypeGithub:
		return &Github{Client: cl}, nil
	case api.ServiceProviderTypeQuay:
		return &Quay{Client: cl}, nil
	}

	return nil, fmt.Errorf("unknown service provider type: %s", serviceProviderType)
}

func FromURL(repoUrl string, cl client.Client) (ServiceProvider, error) {
	spType, err := ServiceProviderTypeFromURL(repoUrl)
	if err != nil {
		return nil, err
	}

	return ByType(spType, cl)
}

func ServiceProviderTypeFromURL(url string) (api.ServiceProviderType, error) {
	if strings.HasPrefix(url, "https://github.com") {
		return api.ServiceProviderTypeGithub, nil
	} else if strings.HasPrefix(url, "https://quay.io") {
		return api.ServiceProviderTypeQuay, nil
	}

	return "", fmt.Errorf("no service provider found for url: %s", url)
}
