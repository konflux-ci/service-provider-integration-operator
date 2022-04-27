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

	"github.com/google/go-github/v43/github"
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
	httpClient    *http.Client
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

func (g *Github) GetRepositoryInfo(ctx context.Context, repoUrl string) *api.SPIAccessCheckStatus {
	ghClient := github.NewClient(g.httpClient)
	owner, repo, err := g.parseGithubRepoUrl(repoUrl)
	status := &api.SPIAccessCheckStatus{
		RepoURL:         repoUrl,
		Accessible:      false,
		Type:            api.SPIRepoTypeGit,
		ServiceProvider: api.ServiceProviderTypeGitHub,
	}
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorBadURL
		status.ErrorMessage = err.Error()
		return status
	}

	ghRepository, _, err := ghClient.Repositories.Get(ctx, owner, repo)
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
		status.ErrorMessage = err.Error()
		return status
	}

	status.Accessible = true
	status.Private = *ghRepository.Private
	return status
}

func (g *Github) parseGithubRepoUrl(repoUrl string) (owner, repo string, err error) {
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
