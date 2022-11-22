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

package gitlab

import (
	"context"
	"errors"
	"fmt"
	http "net/http"
	"net/url"
	"strings"

	"github.com/xanzy/go-gitlab"

	"k8s.io/utils/strings/slices"

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var unsupportedScopeError = errors.New("unsupported scope for GitLab")
var unsupportedAreaError = errors.New("unsupported permission area for GitLab")
var unsupportedUserWritePermissionError = errors.New("user write permission is not supported by GitLab")

// Temp
var notGitlabUrlError = errors.New("not a gitlab repository url")

var _ serviceprovider.ServiceProvider = (*Gitlab)(nil)

type Gitlab struct {
	Configuration          *opconfig.OperatorConfiguration
	lookup                 serviceprovider.GenericLookup
	metadataProvider       *metadataProvider
	httpClient             rest.HTTPClient
	tokenStorage           tokenstorage.TokenStorage
	glClientBuilder        gitlabClientBuilder
	baseUrl                string
	downloadFileCapability downloadFileCapability
}

var _ serviceprovider.ConstructorFunc = newGitlab

var Initializer = serviceprovider.Initializer{
	Probe:                        gitlabProbe{},
	Constructor:                  serviceprovider.ConstructorFunc(newGitlab),
	SupportsManualUploadOnlyMode: true,
}

func newGitlab(factory *serviceprovider.Factory, baseUrl string) (serviceprovider.ServiceProvider, error) {
	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})
	glClientBuilder := gitlabClientBuilder{
		httpClient:   factory.HttpClient,
		tokenStorage: factory.TokenStorage,
	}
	mp := &metadataProvider{
		tokenStorage:    factory.TokenStorage,
		httpClient:      factory.HttpClient,
		glClientBuilder: glClientBuilder,
		baseUrl:         baseUrl,
	}

	return &Gitlab{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitLab,
			TokenFilter:         serviceprovider.NewFilter(factory.Configuration.TokenMatchPolicy, &tokenFilter{}),
			MetadataProvider:    mp,
			MetadataCache:       &cache,
			RepoHostParser:      serviceprovider.RepoHostFromSchemelessUrl,
		},
		tokenStorage:           factory.TokenStorage,
		metadataProvider:       mp,
		httpClient:             factory.HttpClient,
		glClientBuilder:        glClientBuilder,
		baseUrl:                baseUrl,
		downloadFileCapability: NewDownloadFileCapability(factory.HttpClient, glClientBuilder, baseUrl),
	}, nil
}

func (g Gitlab) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, fmt.Errorf("gitlab token lookup failure: %w", err)
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g Gitlab) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	if err := g.lookup.PersistMetadata(ctx, token); err != nil {
		return fmt.Errorf("failed to persist gitlab metadata: %w", err)
	}
	return nil
}

func (g Gitlab) GetBaseUrl() string {
	return g.baseUrl
}

func (g *Gitlab) GetDownloadFileCapability() downloadFileCapability {
	return g.downloadFileCapability
}

func (g *Gitlab) OAuthScopesFor(permissions *api.Permissions) []string {
	// We need ScopeReadUser by default to be able to read user metadata.
	scopes := serviceprovider.GetAllScopes(translateToGitlabScopes, permissions)
	if !slices.Contains(scopes, string(ScopeReadUser)) {
		scopes = append(scopes, string(ScopeReadUser))
	}
	return scopes
}

func translateToGitlabScopes(permission api.Permission) []string {
	switch permission.Area {
	case api.PermissionAreaRepository, api.PermissionAreaRepositoryMetadata:
		if permission.Type.IsWrite() {
			return []string{string(ScopeWriteRepository)}
		}
		return []string{string(ScopeReadRepository)}
	case api.PermissionAreaRegistry:
		if permission.Type.IsWrite() {
			return []string{string(ScopeWriteRegistry)}
		}
		return []string{string(ScopeReadRegistry)}
	case api.PermissionAreaUser:
		return []string{string(ScopeReadUser)}
	}

	return []string{}
}

func (g Gitlab) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeGitLab
}

func (g Gitlab) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	// We currently only check access to git repository on GitLab.
	repoUrl := accessCheck.Spec.RepoUrl
	status := &api.SPIAccessCheckStatus{
		Type:            api.SPIRepoTypeGit,
		ServiceProvider: api.ServiceProviderTypeGitLab,
		Accessibility:   api.SPIAccessCheckAccessibilityUnknown,
	}

	repo, err := g.parseGitlabRepoUrl(accessCheck.Spec.RepoUrl)
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorBadURL
		status.ErrorMessage = err.Error()
		return status, nil //nolint:nilerr // we preserve the error in the status
	}

	publicRepo, err := g.isPublicRepo(ctx, accessCheck)
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

	if len(tokens) < 1 {
		lg.Info("we have no tokens for repository", "repo", repoUrl)
		return status, nil
	}
	token := &tokens[0]

	if err := g.checkPrivateRepoAccess(ctx, token, repo, status); err != nil {
		return nil, err
	}
	return status, nil
}

func (g *Gitlab) checkPrivateRepoAccess(ctx context.Context, token *api.SPIAccessToken, repo string, status *api.SPIAccessCheckStatus) error {
	glClient, err := g.glClientBuilder.createGitlabAuthClient(ctx, token, g.baseUrl)
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorUnknownError
		status.ErrorMessage = err.Error()
		return err
	}

	project, response, err := glClient.Projects.GetProject(repo, nil, gitlab.WithContext(ctx))
	if err != nil {
		status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
		status.ErrorMessage = err.Error()
		return nil //nolint:nilerr // we preserve the error in the status
	}
	if response.StatusCode != http.StatusOK {
		status.ErrorReason = api.SPIAccessCheckErrorRepoNotFound
		status.ErrorMessage = fmt.Sprintf("GitLab responded with non-ok status code: %d", response.StatusCode)
		return nil
	}
	status.Accessible = true

	// "Internal projects can be cloned by any signed-in user except external users."
	// This means that a repo cannot be accessed without user context thus the repo is not public.
	// https://docs.gitlab.com/ee/user/public_access.html#internal-projects-and-groups
	if project.Visibility == gitlab.PrivateVisibility || project.Visibility == gitlab.InternalVisibility {
		status.Accessibility = api.SPIAccessCheckAccessibilityPrivate
	}
	return nil
}

func (g *Gitlab) isPublicRepo(ctx context.Context, accessCheck *api.SPIAccessCheck) (bool, error) {
	lg := log.FromContext(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, accessCheck.Spec.RepoUrl, nil)
	if err != nil {
		lg.Error(err, "failed to construct request to assess if repo is public", "accessCheck", accessCheck)
		return false, fmt.Errorf("error while constructing HTTP request for access check to %s: %w", accessCheck.Spec.RepoUrl, err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the repo to assess if it is public", "accessCheck", accessCheck)
		return false, fmt.Errorf("error performing HTTP request for access check to %s: %w", accessCheck.Spec.RepoUrl, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "unable to close body of request for access check", "accessCheck", accessCheck)
		}
	}()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode != http.StatusNotFound {
		lg.Info("unexpected return code for repo", "accessCheck", accessCheck, "code", resp.StatusCode)
	}
	return false, nil
}

// Temp
func (g *Gitlab) parseGitlabRepoUrl(repoUrl string) (repoPath string, err error) {
	if !strings.HasPrefix(repoUrl, g.GetBaseUrl()) {
		return "", fmt.Errorf("%w: '%s'", notGitlabUrlError, repoUrl)
	}
	return strings.TrimPrefix(repoUrl, g.GetBaseUrl()), nil
}

func (g Gitlab) GetOAuthEndpoint() string {
	return g.Configuration.BaseUrl + "/gitlab/authenticate"
}

func (g Gitlab) MapToken(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	return serviceprovider.DefaultMapToken(token, tokenData), nil
}

func (g Gitlab) Validate(_ context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	ret := serviceprovider.ValidationResult{}

	for _, p := range validated.Permissions().Required {
		switch p.Area {
		case api.PermissionAreaRepository,
			api.PermissionAreaRepositoryMetadata,
			api.PermissionAreaRegistry:
			continue
		case api.PermissionAreaUser:
			if p.Type.IsWrite() {
				ret.ScopeValidation = append(ret.ScopeValidation, unsupportedUserWritePermissionError)
			}
		default:
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w: '%s'", unsupportedAreaError, p.Area))
		}
	}

	for _, s := range validated.Permissions().AdditionalScopes {
		if !IsValidScope(s) {
			ret.ScopeValidation = append(ret.ScopeValidation, fmt.Errorf("%w: '%s'", unsupportedScopeError, s))
		}
	}

	return ret, nil
}

type gitlabProbe struct{}

var _ serviceprovider.Probe = (*gitlabProbe)(nil)

// Examine checks whether the URL host contains gitlab as a substring.
// Note that parsing url without scheme, such as "gitlab.etc/whatever" results in empty host which
// this function discriminates against.
// TODO: In the future, this method should compare the repoUrl to base urls of all configured gitlab service providers.
func (p gitlabProbe) Examine(_ *http.Client, repoUrl string) (string, error) {
	parsed, err := url.Parse(repoUrl)
	if err != nil {
		return "", fmt.Errorf("unable to parse repoUrl: %w", err)
	}

	if strings.Contains(parsed.Host, "gitlab") {
		return parsed.Scheme + "://" + parsed.Host, nil
	}
	return "", nil
}
