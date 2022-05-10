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
	"fmt"
	"net/http"
	"strings"

	"k8s.io/utils/pointer"

	"k8s.io/client-go/rest"

	"github.com/google/go-github/v43/github"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/machinebox/graphql"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Github)(nil)

type Github struct {
	Configuration config.Configuration
	lookup        serviceprovider.GenericLookup
	httpClient    rest.HTTPClient
	tokenStorage  tokenstorage.TokenStorage
}

var Initializer = serviceprovider.Initializer{
	Probe:       githubProbe{},
	Constructor: serviceprovider.ConstructorFunc(newGithub),
}

func newGithub(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
	cache := serviceprovider.NewMetadataCache(factory.Configuration.TokenLookupCacheTtl, factory.KubernetesClient)

	httpClient := serviceprovider.AuthenticatingHttpClient(factory.HttpClient)

	return &Github{
		Configuration: factory.Configuration,
		tokenStorage:  factory.TokenStorage,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitHub,
			TokenFilter:         &tokenFilter{},
			MetadataProvider: &metadataProvider{
				graphqlClient: graphql.NewClient("https://api.github.com/graphql", graphql.WithHTTPClient(httpClient)),
				httpClient:    httpClient,
				tokenStorage:  factory.TokenStorage,
			},
			MetadataCache: &cache,
		},
		httpClient: factory.HttpClient,
	}, nil
}

var _ serviceprovider.ConstructorFunc = newGithub

func (g *Github) GetOAuthEndpoint() string {
	return strings.TrimSuffix(g.Configuration.BaseUrl, "/") + "/github/authenticate"
}

func (g *Github) GetBaseUrl() string {
	return "https://github.com"
}

func (g *Github) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeGitHub
}

func (g *Github) TranslateToScopes(permission api.Permission) []string {
	return translateToScopes(permission)
}

func translateToScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepository:
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
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g *Github) PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
	return g.lookup.PersistMetadata(ctx, token)
}

func (g *Github) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
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
		return status, nil
	}

	publicRepo, err := g.publicRepo(ctx, accessCheck)
	if err != nil {
		return nil, err
	}
	status.Accessible = publicRepo
	if publicRepo {
		status.Accessibility = api.SPIAccessCheckAccessibilityPublic
	}

	lg := log.FromContext(ctx)

	tokens, lookupErr := g.lookup.Lookup(ctx, cl, accessCheck)
	if lookupErr != nil {
		lg.Error(lookupErr, "failed to lookup token for accesscheck", "accessCheck", accessCheck)
		return status, lookupErr
	}

	if len(tokens) > 0 {
		token := &tokens[0]
		ghClient, err := g.createAuthenticatedGhClient(ctx, token)
		if err != nil {
			status.ErrorReason = api.SPIAccessCheckErrorUnknownError
			status.ErrorMessage = err.Error()
			return status, err
		}

		ghRepository, _, err := ghClient.Repositories.Get(ctx, owner, repo)
		if err != nil {
			status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
			status.ErrorMessage = err.Error()
			return status, nil
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

func (g *Github) createAuthenticatedGhClient(ctx context.Context, spiToken *api.SPIAccessToken) (*github.Client, error) {
	token, tsErr := g.tokenStorage.Get(ctx, spiToken)
	if tsErr != nil {
		lg := log.FromContext(ctx)
		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, tsErr
	}
	ctx = context.WithValue(context.TODO(), oauth2.HTTPClient, g.httpClient)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.AccessToken})
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}

func (g *Github) publicRepo(ctx context.Context, accessCheck *api.SPIAccessCheck) (bool, error) {
	lg := log.FromContext(ctx)
	req, reqErr := http.NewRequestWithContext(ctx, "GET", accessCheck.Spec.RepoUrl, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to prepare request")
		return false, reqErr
	}

	if resp, err := g.httpClient.Do(req); err != nil {
		lg.Error(err, "failed to request the repo", "repo", accessCheck.Spec.RepoUrl)
		return false, err
	} else if resp.StatusCode == http.StatusOK {
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
		return "", "", fmt.Errorf("unable to parse path '%s'. looks like it's not a github repo url", repoUrl)
	}
	repoPath := strings.TrimPrefix(repoUrl, g.GetBaseUrl())
	splittedPath := strings.Split(repoPath, "/")
	if len(splittedPath) >= 3 {
		return splittedPath[1], splittedPath[2], nil
	}
	return "", "", fmt.Errorf("unable to parse path '%s'", repoUrl)
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
