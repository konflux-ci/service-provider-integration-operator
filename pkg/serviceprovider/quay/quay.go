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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Quay)(nil)

type Quay struct {
	Configuration    config.Configuration
	lookup           serviceprovider.GenericLookup
	metadataProvider *metadataProvider
	httpClient       rest.HTTPClient
	BaseUrl          string
}

var Initializer = serviceprovider.Initializer{
	Probe:       quayProbe{},
	Constructor: serviceprovider.ConstructorFunc(newQuay),
}

func newQuay(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {

	// in Quay, we invalidate the individual cached repository records, because we're filling up the cache repo-by-repo
	// therefore the metadata as a whole never gets refreshed.
	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})
	mp := &metadataProvider{
		tokenStorage:     factory.TokenStorage,
		httpClient:       factory.HttpClient,
		kubernetesClient: factory.KubernetesClient,
		ttl:              factory.Configuration.TokenLookupCacheTtl,
	}
	return &Quay{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeQuay,
			TokenFilter: &tokenFilter{
				metadataProvider: mp,
			},
			MetadataProvider: mp,
			MetadataCache:    &cache,
			RepoHostParser: serviceprovider.RepoHostParserFunc(func(repoUrl string) (string, error) {
				schemeIndex := strings.Index(repoUrl, "://")
				if schemeIndex == -1 {
					repoUrl = "https://" + repoUrl
				}

				return serviceprovider.RepoHostFromUrl(repoUrl)
			}),
		},
		httpClient:       factory.HttpClient,
		metadataProvider: mp,
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
	// This method is called when constructing the OAuth URL.
	// We represent the ability to pull/push images using fake scopes that don't exist in the Quay model and are used
	// only to represent the permissions of the robot accounts. Since this is an OAuth URL, we need to replace those
	// scopes with their "real" equivalents in the OAuth APIs - i.e. pull == repo:read and push == repo:write

	fullScopes := translateToQuayScopes(permission)

	replace := func(str *string) {
		if *str == string(ScopePull) {
			*str = string(ScopeRepoRead)
		} else if *str == string(ScopePush) {
			*str = string(ScopeRepoWrite)
		}
	}

	// we only return 0, 1 or 2 elements in the arrays, so let's be very concrete here
	if len(fullScopes) == 0 {
		return fullScopes
	} else if len(fullScopes) == 1 {
		replace(&fullScopes[0])
		return fullScopes
	} else if len(fullScopes) == 2 {
		replace(&fullScopes[0])
		replace(&fullScopes[1])
		return fullScopes
	}

	// the generic case in case translateToQuayScopes() returns something longer than 0, 1 or 2 elements

	scopeMap := map[string]bool{}

	for _, s := range fullScopes {
		if s == string(ScopePull) {
			s = string(ScopeRepoRead)
		} else if s == string(ScopePush) {
			s = string(ScopeRepoWrite)
		}

		scopeMap[s] = true
	}

	ret := make([]string, 0, len(scopeMap))
	for s := range scopeMap {
		ret = append(ret, s)
	}

	return ret
}

func translateToQuayScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepositoryMetadata:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopeRepoRead)}
		case api.PermissionTypeWrite:
			return []string{string(ScopeRepoWrite)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopeRepoRead), string(ScopeRepoWrite)}
		}
	case api.PermissionAreaRepository:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopePull)}
		case api.PermissionTypeWrite:
			return []string{string(ScopePush)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopePull), string(ScopePush)}
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

func (g *Quay) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	return g.lookup.PersistMetadata(ctx, token)
}

func (g *Quay) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
}

func (q *Quay) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	log.FromContext(ctx).Info("trying SPIAccessCheck on quay.io. This is not supported yet.")
	return &api.SPIAccessCheckStatus{
		Accessibility: api.SPIAccessCheckAccessibilityUnknown,
		ErrorReason:   api.SPIAccessCheckErrorNotImplemented,
		ErrorMessage:  "Access check for quay.io is not implemented.",
	}, nil
}

func (g *Quay) MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	lg := log.FromContext(ctx, "bindingName", binding.Name, "bindingNamespace", binding.Namespace)
	lg.Info("mapping quay token")

	mapper, err := serviceprovider.DefaultMapToken(token, tokenData)
	if err != nil {
		lg.Error(err, "default mapping failed")
		return serviceprovider.AccessTokenMapper{}, err
	}

	repoMetadata, err := g.metadataProvider.FetchRepo(ctx, binding.Spec.RepoUrl, token)
	if err != nil {
		lg.Error(err, "failed to fetch repository metadata")
		return serviceprovider.AccessTokenMapper{}, nil
	}

	allScopes := make([]Scope, 0, 2)
	allScopes = append(allScopes, repoMetadata.Repository.PossessedScopes...)
	allScopes = append(allScopes, repoMetadata.Organization.PossessedScopes...)

	scopeStrings := make([]string, len(allScopes))
	for i, s := range allScopes {
		scopeStrings[i] = string(s)
	}

	mapper.Scopes = scopeStrings

	return mapper, nil
}

func (q *Quay) Validate(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	ret := serviceprovider.ValidationResult{}

	userPermissionAreaRequested := false
	for _, p := range validated.Permissions().Required {
		if p.Area == api.PermissionAreaUser && !userPermissionAreaRequested {
			ret.ScopeValidation = append(ret.ScopeValidation, errors.New("user-related permissions are not supported for Quay"))
			userPermissionAreaRequested = true
		}
	}

	for _, s := range validated.Permissions().AdditionalScopes {
		switch Scope(s) {
		case ScopeUserRead, ScopeUserAdmin:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("scope '%s' is not supported", s))
		case ScopeRepoRead, ScopeRepoWrite, ScopeRepoCreate, ScopeRepoAdmin, ScopeOrgAdmin, ScopePull, ScopePush:
			{
			}
		default:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("unknown scope: '%s'", s))
		}
	}

	return ret, nil
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
