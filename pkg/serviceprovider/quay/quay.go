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

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Quay)(nil)

var (
	unsupportedAreaError      = errors.New("unsupported permission area for Quay")
	unsupportedScopeError     = errors.New("unsupported scope")
	unknownScopeError         = errors.New("unknown scope")
	failedToParseRepoUrlError = errors.New("failed to parse repository URL")
	unexpectedStatusCodeError = errors.New("unexpected status code")
	noResponseError           = errors.New("no response")

	quayApiBaseUrl = config.ServiceProviderTypeQuay.DefaultBaseUrl + "/api/v1"
)

type Quay struct {
	Configuration    *opconfig.OperatorConfiguration
	lookup           serviceprovider.GenericLookup
	metadataProvider *metadataProvider
	httpClient       rest.HTTPClient
	tokenStorage     tokenstorage.TokenStorage
	BaseUrl          string
	OAuthCapability  serviceprovider.OAuthCapability
}
type quayOAuthCapability struct {
	serviceprovider.DefaultOAuthCapability
}

var Initializer = serviceprovider.Initializer{
	Probe:       quayProbe{},
	Constructor: serviceprovider.ConstructorFunc(newQuay),
}

func newQuay(factory *serviceprovider.Factory, spConfig *config.ServiceProviderConfiguration) (serviceprovider.ServiceProvider, error) {
	// in Quay, we invalidate the individual cached repository records, because we're filling up the cache repo-by-repo
	// therefore the metadata as a whole never gets refreshed.
	cache := factory.NewCacheWithExpirationPolicy(&serviceprovider.NeverMetadataExpirationPolicy{})
	mp := &metadataProvider{
		tokenStorage:     factory.TokenStorage,
		httpClient:       factory.HttpClient,
		kubernetesClient: factory.KubernetesClient,
		ttl:              factory.Configuration.TokenLookupCacheTtl,
	}

	var oauthCapability serviceprovider.OAuthCapability
	if spConfig != nil && spConfig.OAuth2Config != nil {
		oauthCapability = &quayOAuthCapability{
			DefaultOAuthCapability: serviceprovider.DefaultOAuthCapability{
				BaseUrl: factory.Configuration.BaseUrl,
			},
		}
	}

	return &Quay{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeQuay,
			TokenFilter:         serviceprovider.NewFilter(factory.Configuration.TokenMatchPolicy, &tokenFilter{}),
			MetadataProvider:    mp,
			MetadataCache:       &cache,
			RepoHostParser:      serviceprovider.RepoHostFromSchemelessUrl,
		},
		httpClient:       factory.HttpClient,
		tokenStorage:     factory.TokenStorage,
		metadataProvider: mp,
		OAuthCapability:  oauthCapability,
	}, nil
}

var _ serviceprovider.ConstructorFunc = newQuay

func (q *Quay) GetBaseUrl() string {
	return config.ServiceProviderTypeQuay.DefaultBaseUrl
}

func (q *Quay) GetType() config.ServiceProviderType {
	return config.ServiceProviderTypeQuay
}

func (q *Quay) GetDownloadFileCapability() serviceprovider.DownloadFileCapability {
	return nil
}

func (q *Quay) GetRefreshTokenCapability() serviceprovider.RefreshTokenCapability {
	return nil
}

func (q *Quay) GetOAuthCapability() serviceprovider.OAuthCapability {
	return q.OAuthCapability
}

func (q *quayOAuthCapability) OAuthScopesFor(ps *api.Permissions) []string {
	// This method is called when constructing the OAuth URL.
	// We basically disregard any request for specific permissions and always require the max usable set of permissions
	// because we cannot change that set later due to a bug in Quay OAuth impl:
	// https://issues.redhat.com/browse/PROJQUAY-3908

	// Note that we don't require org:admin, because that is a super strong permission for which we currently don't
	// have usecase. Users can still require it using the spec.permissions.additionalScopes if needed.
	scopes := map[string]bool{}
	scopes[string(ScopeRepoRead)] = true
	scopes[string(ScopeRepoWrite)] = true
	scopes[string(ScopeRepoCreate)] = true
	scopes[string(ScopeRepoAdmin)] = true

	for _, s := range ps.AdditionalScopes {
		scopes[s] = true
	}

	ret := make([]string, 0, len(scopes))
	for s := range scopes {
		ret = append(ret, s)
	}
	return ret
}

func translateToQuayScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRegistryMetadata:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopeRepoRead)}
		case api.PermissionTypeWrite:
			return []string{string(ScopeRepoWrite)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopeRepoRead), string(ScopeRepoWrite)}
		}
	case api.PermissionAreaRegistry:
		switch permission.Type {
		case api.PermissionTypeRead:
			return []string{string(ScopePull)}
		case api.PermissionTypeWrite:
			return []string{string(ScopePush)}
		case api.PermissionTypeReadWrite:
			return []string{string(ScopePull), string(ScopePush)}
		}
	}

	return []string{}
}

func (q *Quay) LookupTokens(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
	tokens, err := q.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, fmt.Errorf("quay token lookup failure: %w", err)
	}
	return tokens, nil
}

func (q *Quay) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	if err := q.lookup.PersistMetadata(ctx, token); err != nil {
		return fmt.Errorf("failed to persiste quay metadata: %w", err)
	}
	return nil
}

func (q *Quay) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	status := &api.SPIAccessCheckStatus{
		Type:            api.SPIRepoTypeContainerRegistry,
		ServiceProvider: api.ServiceProviderTypeQuay,
		Accessibility:   api.SPIAccessCheckAccessibilityUnknown,
		Accessible:      false,
	}

	lg := log.FromContext(ctx)

	owner, repository, _ := splitToOrganizationAndRepositoryAndVersion(accessCheck.Spec.RepoUrl)
	if owner == "" || repository == "" {
		lg.Error(failedToParseRepoUrlError, "we don't reconcile this resource again as we don't understand the URL. Error written to SPIAccessCheck status.", "repo url", accessCheck.Spec.RepoUrl)
		status.ErrorReason = api.SPIAccessCheckErrorBadURL
		status.ErrorMessage = failedToParseRepoUrlError.Error()
		return status, nil // return nil error, because we don't want to reconcile this again
	}

	tokens, lookupErr := q.lookup.Lookup(ctx, cl, accessCheck)
	if lookupErr != nil {
		lg.Error(lookupErr, "failed to lookup token for accesscheck", "accessCheck", accessCheck)
		status.ErrorReason = api.SPIAccessCheckErrorTokenLookupFailed
		status.ErrorMessage = lookupErr.Error()
		// not returning here. We're still able to detect public repository without the token.
		// The error will still be reported in status.
	}

	var username, token string
	if len(tokens) > 0 {
		lg.Info("found tokens", "count", len(tokens), "taking 1st", tokens[0])
		apiToken, getTokenErr := q.tokenStorage.Get(ctx, &tokens[0])
		if getTokenErr != nil {
			return status, fmt.Errorf("failed to get token: %w", getTokenErr)
		}
		if apiToken != nil {
			username, token = getUsernameAndPasswordFromTokenData(apiToken)
		}
	} else {
		lg.Info("we have no tokens for repository", "repoUrl", accessCheck.Spec.RepoUrl)
	}

	if responseCode, repoInfo, err := q.requestRepoInfo(ctx, owner, repository, token); err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorUnknownError
		status.ErrorMessage = "failed request to Quay API"
		return status, err
	} else {
		switch responseCode {
		case http.StatusOK:
			status.Accessible = true
			status.ErrorReason = ""
			status.ErrorMessage = ""
			if repoInfo["is_public"].(bool) {
				status.Accessibility = api.SPIAccessCheckAccessibilityPublic
			} else {
				status.Accessibility = api.SPIAccessCheckAccessibilityPrivate
			}
		case http.StatusUnauthorized, http.StatusForbidden:
			// if we have no token, we cannot distinguish between non-existent and private repository, so in that case
			// we can assign no new status here...
			if token != "" {
				// ok, we failed to authorize with a token. This means that we either are using a robot token on a
				// private repo or token lookup didn't return a valid token (maybe an expired one or the perms changed
				// in quay in the meantime).
				if username != "" && username != OAuthTokenUserName {
					// yes, a robot token. All we know is that the token lookup succeeded, so this must mean docker login
					// must have succeeded (now or some time ago). So let's just assume here that the repo is accessible.
					// For public repositories, the Quay API repository info query succeeds with any (or none) credentials.
					// Since we're seeing a failure here, this means that this must be a private repo.
					status.Accessible = true
					status.Accessibility = api.SPIAccessCheckAccessibilityPrivate
				} else {
					// hmm.. so the token lookup was wrong about the repository. This is weird...
					lg.Info("quay.io request unauthorized using a looked up token. Have permissions changed in the meantime?")
				}
			}
		case http.StatusNotFound:
			if status.ErrorReason == "" && status.ErrorMessage == "" {
				status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
				status.ErrorMessage = "repository does not exist"
			}
		default:
			status.ErrorReason = api.SPIAccessCheckErrorUnknownError
			status.ErrorMessage = "unexpected response from Quay API"
			return status, fmt.Errorf("%w '%d' for quay.io repository request '%s'", unexpectedStatusCodeError, responseCode, accessCheck.Spec.RepoUrl)
		}
	}

	return status, nil
}

func (q *Quay) requestRepoInfo(ctx context.Context, owner, repository, token string) (int, map[string]interface{}, error) {
	requestUrl := fmt.Sprintf("%s/repository/%s/%s?includeTags=false", quayApiBaseUrl, owner, repository)
	lg := log.FromContext(ctx, "repository", repository, "url", requestUrl)

	resp, err := doQuayRequest(ctx, q.httpClient, requestUrl, token, "GET", nil, "")
	if err != nil {
		lg.Error(err, "failed to request quay.io api for repository info")
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		return code, nil, fmt.Errorf("failed to request quay on %s: %w", requestUrl, err)
	}
	if resp != nil && resp.Body != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				lg.Error(err, "failed to close response body")
			}
		}()
	}

	if resp != nil && resp.StatusCode == http.StatusOK {
		jsonResponse, jsonErr := readResponseBodyToJsonMap(ctx, resp)
		if jsonErr != nil {
			return resp.StatusCode, nil, jsonErr
		}
		return resp.StatusCode, jsonResponse, nil
	} else {
		if resp != nil {
			return resp.StatusCode, nil, nil
		} else {
			return 0, nil, fmt.Errorf("%w for request '%s'", noResponseError, requestUrl)
		}
	}
}

func (q *Quay) MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	lg := log.FromContext(ctx, "bindingName", binding.Name, "bindingNamespace", binding.Namespace)
	lg.Info("mapping quay token")

	mapper := serviceprovider.DefaultMapToken(token, tokenData)

	repoMetadata, err := q.metadataProvider.FetchRepo(ctx, binding.Spec.RepoUrl, token)
	if err != nil {
		lg.Error(err, "failed to fetch repository metadata")
		return serviceprovider.AccessTokenMapper{}, nil
	}

	if repoMetadata == nil {
		return mapper, nil
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

	for _, p := range validated.Permissions().Required {
		switch p.Area {
		case api.PermissionAreaRegistry,
			api.PermissionAreaRegistryMetadata:
			continue
		default:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w: '%s'", unsupportedAreaError, p.Area))
		}
	}

	for _, s := range validated.Permissions().AdditionalScopes {
		switch Scope(s) {
		case ScopeUserRead, ScopeUserAdmin:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w '%s'", unsupportedScopeError, s))
		case ScopeRepoRead, ScopeRepoWrite, ScopeRepoCreate, ScopeRepoAdmin, ScopeOrgAdmin, ScopePull, ScopePush:
			{
			}
		default:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w: '%s'", unknownScopeError, s))
		}
	}

	return ret, nil
}

type quayProbe struct{}

var _ serviceprovider.Probe = (*quayProbe)(nil)

func (q quayProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.HasPrefix(url, config.ServiceProviderTypeQuay.DefaultBaseUrl) || strings.HasPrefix(url, config.ServiceProviderTypeQuay.DefaultHost) {
		return config.ServiceProviderTypeQuay.DefaultBaseUrl, nil
	} else {
		return "", nil
	}
}
