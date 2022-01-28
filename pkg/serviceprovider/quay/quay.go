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

package quay

import (
	"context"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Quay)(nil)

type Quay struct {
	Configuration config.Configuration
}

var Initializer = serviceprovider.Initializer{
	Probe:       quayProbe{},
	Constructor: serviceprovider.ConstructorFunc(newQuay),
}

func newQuay(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
	return &Quay{
		Configuration: factory.Configuration,
	}, nil
}

var _ serviceprovider.ConstructorFunc = newQuay

func (g *Quay) GetOAuthEndpoint() string {
	return strings.TrimSuffix(g.Configuration.BaseUrl, "/") + "/quay/authenticate"
}

func (g *Quay) GetBaseUrl() string {
	return "https://quay.io"
}

func (g *Quay) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeQuay
}

func (g *Quay) TranslateToScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepository:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{"repo:read"}
		case api.PermissionTypeWrite:
			return []string{"repo:write"}
		case api.PermissionTypeReadWrite:
			return []string{"repo:read", "repo:write"}
		}
	}

	return []string{}
}

func (g *Quay) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	// TODO implement

	// for now just return the first SPIAccessToken that we find so that we prevent infinitely many SPIAccessTokens
	// being created during the tests :)
	ats := &api.SPIAccessTokenList{}
	if err := cl.List(ctx, ats, client.Limit(1)); err != nil {
		return nil, err
	}

	if len(ats.Items) == 0 {
		return nil, nil
	}

	return &ats.Items[0], nil
}

func (g *Quay) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
}

type quayProbe struct{}

var _ serviceprovider.Probe = (*quayProbe)(nil)

func (q quayProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.HasPrefix(url, "https://quay.io") {
		return "https://quay.io", nil
	} else {
		return "", nil
	}
}
