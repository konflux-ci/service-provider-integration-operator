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

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"k8s.io/utils/pointer"

	"k8s.io/client-go/rest"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Github)(nil)

var (
	unableToParsePathError = errors.New("unable to parse path")
	notGithubUrlError      = errors.New("not a github repository url")
	unknownScopeError      = errors.New("unknown scope")
)

type Github struct {
	Configuration   *opconfig.OperatorConfiguration
	lookup          serviceprovider.GenericLookup
	httpClient      rest.HTTPClient
	tokenStorage    tokenstorage.TokenStorage
	ghClientBuilder githubClientBuilder
}

var Initializer = serviceprovider.Initializer{
	Probe:                        githubProbe{},
	Constructor:                  serviceprovider.ConstructorFunc(newGithub),
	SupportsManualUploadOnlyMode: true,
}

func newGithub(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.TtlMetadataExpirationPolicy{Ttl: factory.Configuration.TokenLookupCacheTtl})

	httpClient := serviceprovider.AuthenticatingHttpClient(factory.HttpClient)
	ghClientBuilder := githubClientBuilder{
		tokenStorage: factory.TokenStorage,
		httpClient:   factory.HttpClient,
	}
	github := &Github{
		Configuration: factory.Configuration,
		tokenStorage:  factory.TokenStorage,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitHub,
			TokenFilter:         serviceprovider.NewFilter(factory.Configuration.TokenMatchPolicy, &tokenFilter{}),
			MetadataProvider: &metadataProvider{
				httpClient:      httpClient,
				tokenStorage:    factory.TokenStorage,
				ghClientBuilder: ghClientBuilder,
			},
			MetadataCache:  &cache,
			RepoHostParser: serviceprovider.RepoHostFromUrl,
		},
		httpClient:      factory.HttpClient,
		ghClientBuilder: ghClientBuilder,
	}
	return github, nil
}

var _ serviceprovider.ConstructorFunc = newGithub

func (g *Github) GetOAuthEndpoint() string {
	return g.Configuration.BaseUrl + "/github/authenticate"
}

func (g *Github) GetBaseUrl() string {
	return "https://github.com"
}

func (g *Github) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeGitHub
}

func (g *Github) OAuthScopesFor(permissions *api.Permissions) []string {
	return serviceprovider.GetAllScopes(translateToScopes, permissions)
}

func translateToScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepository, api.PermissionAreaRepositoryMetadata:
		return []string{"repo"}
	case api.PermissionAreaWebhooks:
		if permission.Type.IsWrite() {
			return []string{"write:repo_hook"}
		} else {
			return []string{"read:repo_hook"}
		}
	case api.PermissionAreaUser:
		if permission.Type.IsWrite() {
			return []string{"user"}
		} else {
			return []string{"read:user"}
		}
	}

	return []string{}
}

func (g *Github) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, fmt.Errorf("github token lookup failure: %w", err)
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g *Github) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	if err := g.lookup.PersistMetadata(ctx, token); err != nil {
		return fmt.Errorf("failed to persist github metadata: %w", err)
	}
	return nil
}

func (g *Github) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	url, err := serviceprovider.GetHostWithScheme(repoUrl)
	if err != nil {
		err = fmt.Errorf("failed to get host and scheme from %s: %w", repoUrl, err)
	}

	return url, err
}

func (g *Github) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	repoUrl := accessCheck.Spec.RepoUrl

	status := &api.SPIAccessCheckStatus{
		Type:            api.SPIRepoTypeGit,
		ServiceProvider: api.ServiceProviderTypeGitHub,
		Accessibility:   api.SPIAccessCheckAccessibilityUnknown,
	}

	owner, repo, err := g.parseGithubRepoUrl(accessCheck.Spec.RepoUrl)
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorBadURL
		status.ErrorMessage = err.Error()
		return status, nil //nolint:nilerr // we preserve the error in the status
	}

	publicRepo, err := g.publicRepo(ctx, accessCheck)
	if err != nil {
		return nil, err
	}
	status.Accessible = publicRepo
	if publicRepo {
		status.Accessibility = api.SPIAccessCheckAccessibilityPublic
		return status, nil
	}

	lg := log.FromContext(ctx)

	tokens, lookupErr := g.lookup.Lookup(ctx, cl, accessCheck)
	if lookupErr != nil {
		lg.Error(lookupErr, "failed to lookup token for accesscheck", "accessCheck", accessCheck)
		if !publicRepo {
			status.ErrorReason = api.SPIAccessCheckErrorTokenLookupFailed
			status.ErrorMessage = lookupErr.Error()
		}
		return status, nil
	}

	if len(tokens) > 0 {
		token := &tokens[0]
		ghClient, err := g.ghClientBuilder.createAuthenticatedGhClient(ctx, token)
		if err != nil {
			status.ErrorReason = api.SPIAccessCheckErrorUnknownError
			status.ErrorMessage = err.Error()
			return status, err
		}

		ghRepository, _, err := ghClient.Repositories.Get(ctx, owner, repo)
		if err != nil {
			status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
			status.ErrorMessage = err.Error()
			return status, nil //nolint:nilerr // we preserve the error in the status
		}

		status.Accessible = true
		if pointer.BoolDeref(ghRepository.Private, false) {
			status.Accessibility = api.SPIAccessCheckAccessibilityPrivate
		}
	} else {
		lg.Info("we have no tokens for repository", "repo", repoUrl)
	}

	return status, nil
}

func (g *Github) Validate(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	// only the additional scopes can be invalid. We support the translation for all types
	// of the Permission in github.
	ret := serviceprovider.ValidationResult{}
	for _, s := range validated.Permissions().AdditionalScopes {
		if !IsValidScope(s) {
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w: '%s'", unknownScopeError, s))
		}
	}

	return ret, nil
}

func (g *Github) publicRepo(ctx context.Context, accessCheck *api.SPIAccessCheck) (bool, error) {
	lg := log.FromContext(ctx)
	req, reqErr := http.NewRequestWithContext(ctx, "GET", accessCheck.Spec.RepoUrl, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to prepare request", "accessCheck", accessCheck.Spec)
		return false, fmt.Errorf("error while constructing HTTP request for access check to %s: %w", accessCheck.Spec.RepoUrl, reqErr)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the repo", "repo", accessCheck.Spec.RepoUrl)
		return false, fmt.Errorf("error performing HTTP request for access check to %v: %w", accessCheck.Spec.RepoUrl, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else {
		lg.Info("unexpected return code for repo", "repo", accessCheck.Spec.RepoUrl, "code", resp.StatusCode)
		return false, nil
	}
}

func (g *Github) parseGithubRepoUrl(repoUrl string) (owner, repo string, err error) {
	if !strings.HasPrefix(repoUrl, g.GetBaseUrl()) {
		return "", "", fmt.Errorf("%w: '%s'", notGithubUrlError, repoUrl)
	}
	repoPath := strings.TrimPrefix(repoUrl, g.GetBaseUrl())
	splittedPath := strings.Split(repoPath, "/")
	if len(splittedPath) >= 3 {
		return splittedPath[1], splittedPath[2], nil
	}
	return "", "", fmt.Errorf("%w '%s'", unableToParsePathError, repoUrl)
}

func (g *Github) MapToken(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	return serviceprovider.DefaultMapToken(token, tokenData), nil
}

type githubProbe struct{}

var _ serviceprovider.Probe = (*githubProbe)(nil)

func (g githubProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.HasPrefix(url, "https://github.com") {
		return "https://github.com", nil
	} else {
		return "", nil
	}
}
