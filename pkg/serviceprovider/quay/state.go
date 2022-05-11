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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type EntityRecord struct {
	LastRefreshTime int64
	PossessedScopes []Scope
}

type TokenState struct {
	Repositories  map[string]EntityRecord
	Organizations map[string]EntityRecord
	Users         map[string]EntityRecord
}

type Scope string

const (
	ScopeRepoRead   Scope = "repo:read"
	ScopeRepoWrite  Scope = "repo:write"
	ScopeRepoAdmin  Scope = "repo:admin"
	ScopeRepoCreate Scope = "repo:create"
	ScopeUserRead   Scope = "user:read"
	ScopeUserAdmin  Scope = "user:admin"
	ScopeOrgAdmin   Scope = "org:admin"
)

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}

	switch s {
	case ScopeRepoWrite:
		return other == ScopeRepoRead
	case ScopeRepoAdmin:
		return other == ScopeRepoRead || other == ScopeRepoWrite || other == ScopeRepoCreate
	case ScopeUserAdmin:
		return other == ScopeUserRead
	}

	return false
}

func (s Scope) IsIncluded(scopes []Scope) bool {
	for _, sc := range scopes {
		if sc.Implies(s) {
			return true
		}
	}

	return false
}

// fetchRepositoryRecord fetches the metadat about what access does the token have on the provided repository.
func fetchRepositoryRecord(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token) (*EntityRecord, error) {
	password := tokenData.AccessToken
	username := tokenData.Username

	if username == "" {
		username = "$oauthtoken"
	}

	if username != "$oauthtoken" {
		// we're dealing with robot account
		return robotAccountRepositoryRecord(ctx, cl, repoUrl, username, password)
	} else {
		// we're dealing with an oauth token
		return oauthRepositoryRecord(ctx, cl, repoUrl, password)
	}
}

// fetchOrganizationRecord fetches the metadata about what access does the token have on the provided organization.
func fetchOrganizationRecord(ctx context.Context, cl *http.Client, organization string, tokenData *api.Token) (*EntityRecord, error) {
	username := tokenData.Username

	if username == "" {
		username = "$oauthtoken"
	}

	if username != "$oauthtoken" {
		return nil, nil
	}

	orgAdmin, err := hasOrgAdmin(ctx, cl, organization, tokenData.AccessToken)
	if err != nil {
		return nil, err
	}

	scopes := []Scope{}

	if orgAdmin {
		scopes = append(scopes, ScopeOrgAdmin)
	}

	return &EntityRecord{
		PossessedScopes: scopes,
	}, nil
}

// fetchUserRecord fetches the metadata about what access does the token have on the user the token acts on behalf of.
// the unused string parameter is kept for signature compatibility with the other fetch* functions so that they can
// be used in the metadataProvider.
func fetchUserRecord(ctx context.Context, cl *http.Client, _ string, tokenData *api.Token) (*EntityRecord, error) {
	username := tokenData.Username

	if username == "" {
		username = "$oauthtoken"
	}

	if username != "$oauthtoken" {
		return nil, nil
	}

	userAdmin, err := hasUserAdmin(ctx, cl, tokenData.AccessToken)
	if err != nil {
		return nil, err
	}

	userRead, err := hasUserRead(ctx, cl, tokenData.AccessToken)
	if err != nil {
		return nil, err
	}

	scopes := []Scope{}

	if userAdmin {
		scopes = append(scopes, ScopeUserAdmin)
	}

	if userRead {
		scopes = append(scopes, ScopeUserRead)
	}

	return &EntityRecord{
		PossessedScopes: scopes,
	}, nil
}

func robotAccountRepositoryRecord(ctx context.Context, cl *http.Client, repoUrl string, username string, password string) (*EntityRecord, error) {
	repository, _ := splitToImageAndVersion(repoUrl)
	loginToken, err := DockerLogin(ctx, cl, repository, username, password)
	if err != nil {
		return nil, err
	}

	tokenInfos, err := AnalyzeLoginToken(loginToken)
	if err != nil {
		return nil, err
	}

	for _, ti := range tokenInfos {
		if ti.Repository == repository {
			var possessedScopes []Scope

			if ti.Pullable {
				possessedScopes = append(possessedScopes, ScopeRepoRead)
			}

			if ti.Pushable {
				possessedScopes = append(possessedScopes, ScopeRepoWrite)
			}

			return &EntityRecord{
				PossessedScopes: possessedScopes,
			}, nil
		}
	}

	return nil, nil
}

func oauthRepositoryRecord(ctx context.Context, cl *http.Client, repoUrl string, token string) (*EntityRecord, error) {
	// first try to figure out repo:read and repo:write just by trying to log in
	rr, err := robotAccountRepositoryRecord(ctx, cl, repoUrl, "$oauthtoken", token)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, nil
	}

	repoAdmin, err := hasRepoAdmin(ctx, cl, repoUrl, token)
	if err != nil {
		return nil, err
	}
	if repoAdmin {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoAdmin)
	}

	repoCreate, err := hasRepoCreate(ctx, cl, repoUrl, token)
	if err != nil {
		return nil, err
	}
	if repoCreate {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoCreate)
	}

	return rr, nil
}

func hasRepoCreate(ctx context.Context, cl *http.Client, repoUrl string, token string) (bool, error) {
	organization, _, _ := splitToOrganizationAndRepositoryAndVersion(repoUrl)
	data := strings.NewReader(`{
		"repository": "an/intentionally/invalid/name",
        "visibility": "public",
        "namespace": "` + organization + `",
		"description": "",
        "repo_kind": "image"
    }`)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://quay.io/api/v1/repository", data)
	if err != nil {
		return false, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cl.Do(req)
	if err != nil {
		return false, err
	}

	// here, we exploit the order of input validation in Quay. The ability to write to a certain namespace is checked
	// before the validity of the repository name.
	// Therefore, if we can't write to the namespace, we get 403.
	// If we can write to the namespace, we get 400 because our repository name is invalid.
	return resp.StatusCode == 400, nil
}

func hasRepoAdmin(ctx context.Context, cl *http.Client, repoUrl string, token string) (bool, error) {
	img, _ := splitToImageAndVersion(repoUrl)
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/repository/"+img+"/notification/", token)
}

func hasUserRead(ctx context.Context, cl *http.Client, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/user/", token)
}

func hasUserAdmin(ctx context.Context, cl *http.Client, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/user/robots?limit=1&token=false&permissions=false", token)
}

func hasOrgAdmin(ctx context.Context, cl *http.Client, repository string, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/organization/"+repository+"/robots?limit=1&token=false&permissions=false", token)
}

func isSuccessfulRequest(ctx context.Context, cl *http.Client, url string, token string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cl.Do(req)
	if err != nil {
		return false, err
	}

	return resp.StatusCode == 200, nil
}

func splitToImageAndVersion(image string) (string, string) {
	repo, img, ver := splitToOrganizationAndRepositoryAndVersion(image)
	return repo + "/" + img, ver
}

func splitToOrganizationAndRepositoryAndVersion(image string) (string, string, string) {
	schemeIndex := strings.Index(image, "://")
	if schemeIndex > 0 {
		image = image[(schemeIndex + 3):]
	}

	parts := strings.Split(image, "/")
	if len(parts) != 3 {
		return "", "", ""
	}

	host := parts[0]

	if host != "quay.io" {
		return "", "", ""
	}

	repo := parts[1]
	img := parts[2]
	imgParts := strings.Split(img, ":")
	version := ""
	if len(imgParts) == 2 {
		version = imgParts[1]
	}

	return repo, imgParts[0], version
}
