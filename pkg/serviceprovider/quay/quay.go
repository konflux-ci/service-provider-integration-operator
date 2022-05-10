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
	"net/http"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Quay)(nil)

type Quay struct {
	Configuration config.Configuration
	lookup        serviceprovider.GenericLookup
	httpClient    *http.Client
	BaseUrl       string
}

var Initializer = serviceprovider.Initializer{
	Probe:       quayProbe{},
	Constructor: serviceprovider.ConstructorFunc(newQuay),
}

func newQuay(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {

	// in Quay, we invalidate the individual cached repository records, because we're filling up the cache repo-by-repo
	// therefore the metadata as a whole never gets refreshed.
	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})
	return &Quay{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeQuay,
			TokenFilter: &tokenFilter{
				kubernetesClient: factory.KubernetesClient,
				httpClient:       factory.HttpClient,
				tokenStorage:     factory.TokenStorage,
				ttl:              factory.Configuration.TokenLookupCacheTtl,
			},
			MetadataProvider: &metadataProvider{
				tokenStorage: factory.TokenStorage,
			},
			MetadataCache: &cache,
		},
		httpClient: factory.HttpClient,
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
	return translateToQuayScopes(permission)
}

func translateToQuayScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepository:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopeRepoRead)}
		case api.PermissionTypeWrite:
			return []string{string(ScopeRepoWrite)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopeRepoRead), string(ScopeRepoWrite)}
		}
	case api.PermissionAreaUser:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopeUserRead)}
		case api.PermissionTypeWrite:
			return []string{string(ScopeUserAdmin)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopeUserAdmin)}
		}
	}

	return []string{}
}

func (g *Quay) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g *Quay) PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
	return g.lookup.PersistMetadata(ctx, token)
}

func (g *Quay) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
}

type quayProbe struct{}

var _ serviceprovider.Probe = (*quayProbe)(nil)

func (q quayProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.HasPrefix(url, "https://quay.io") || strings.HasPrefix(url, "quay.io") {
		return "https://quay.io", nil
	} else {
		return "", nil
	}
}
