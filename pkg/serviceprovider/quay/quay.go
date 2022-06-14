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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

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
	tokenStorage     tokenstorage.TokenStorage
	BaseUrl          string
}

var Initializer = serviceprovider.Initializer{
	Probe:       quayProbe{},
	Constructor: serviceprovider.ConstructorFunc(newQuay),
}

const quayUrlBase = "https://quay.io"
const quayApiUrlBase = quayUrlBase + "/api/v1"

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
		tokenStorage:     factory.TokenStorage,
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

func (q *Quay) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	status := &api.SPIAccessCheckStatus{
		Type:            api.SPIRepoTypeContainerRegistry,
		ServiceProvider: api.ServiceProviderTypeQuay,
		Accessibility:   api.SPIAccessCheckAccessibilityUnknown,
		Accessible:      false,
	}

	owner, repo, err := q.parseQuayRepoUrl(accessCheck.Spec.RepoUrl)
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorBadURL
		status.ErrorMessage = err.Error()
		return status, nil
	}

	lg := log.FromContext(ctx)

	tokens, lookupErr := q.lookup.Lookup(ctx, cl, accessCheck)
	if lookupErr != nil {
		lg.Error(lookupErr, "failed to lookup token for accesscheck", "accessCheck", accessCheck)
		return status, lookupErr
	}

	token := ""
	if len(tokens) > 0 {
		lg.Info("found tokens", "cnt", len(tokens), "1st token", tokens[0])
		if apiToken, getTokenErr := q.tokenStorage.Get(ctx, &tokens[0]); getTokenErr == nil {
			token = apiToken.AccessToken
		} else {
			return nil, getTokenErr
		}
	} else {
		lg.Info("we have no tokens for repository", "repo", accessCheck.Spec.RepoUrl)
	}

	requestUrl := fmt.Sprintf("%s/repository/%s/%s?includeTags=false", quayApiUrlBase, owner, repo)
	if resp, err := doQuayRequest(ctx, q.httpClient, requestUrl, token, "GET", nil, ""); err != nil {
		lg.Error(err, "failed to request the repo", "repo", accessCheck.Spec.RepoUrl)
		return nil, err
	} else if resp.StatusCode == http.StatusOK {
		status.Accessible = true

		defer resp.Body.Close()

		var j map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&j)
		if err != nil {
			return nil, err
		}
		if j["is_public"].(bool) {
			status.Accessibility = api.SPIAccessCheckAccessibilityPublic
		} else {
			status.Accessibility = api.SPIAccessCheckAccessibilityPrivate
		}
		lg.Info("received ok response for quay repo", "body", j)
	} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		lg.Info("quay.io request unauthorized. Probably private repository for we don't have a token.")
	} else if resp.StatusCode == http.StatusNotFound {
		status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
		status.ErrorMessage = "repository does not exist"
	} else {
		return nil, fmt.Errorf("unexpected return code '%d' for quay.io repository request '%s'", resp.StatusCode, accessCheck.Spec.RepoUrl)
	}

	return status, nil
}

func (q *Quay) parseQuayRepoUrl(repoUrl string) (string, string, error) {
	if !strings.HasPrefix(repoUrl, "http") {
		repoUrl = "https://" + repoUrl
	}
	parsedUrl, parseErr := url.Parse(repoUrl)
	if parseErr != nil {
		return "", "", parseErr
	}

	splittedPath := strings.Split(parsedUrl.Path, "/")
	if len(splittedPath) < 2 {
		return "", "", fmt.Errorf("unexpected quay url")
	}
	return splittedPath[len(splittedPath)-2], splittedPath[len(splittedPath)-1], nil
}

func (q *Quay) publicRepo(ctx context.Context, accessCheck *api.SPIAccessCheck, owner string, repository string) (bool, error) {
	lg := log.FromContext(ctx)

	requestUrl := fmt.Sprintf("%s/repository/%s/%s?includeTags=false", quayApiUrlBase, owner, repository)
	if resp, err := doQuayRequest(ctx, q.httpClient, requestUrl, "", "GET", nil, ""); err != nil {
		lg.Error(err, "failed to request the repo", "repo", accessCheck.Spec.RepoUrl)
		return false, err
	} else if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return false, nil
	} else {
		lg.Info("unexpected return code for repo", "repo", accessCheck.Spec.RepoUrl, "code", resp.StatusCode)
		return false, nil
	}
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
