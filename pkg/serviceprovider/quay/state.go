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
func fetchRepositoryRecord(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token, info LoginTokenInfo) (*EntityRecord, error) {
	username, password := getUsernameAndPasswordFromTokenData(tokenData)

	if username != "$oauthtoken" {
		// we're dealing with robot account
		return robotAccountRepositoryRecord(repoUrl, info)
	} else {
		// we're dealing with an oauth token
		return oauthRepositoryRecord(ctx, cl, repoUrl, password, info)
	}
}

func getUsernameAndPasswordFromTokenData(tokenData *api.Token) (username, password string) {
	username = tokenData.Username

	if username == "" {
		username = "$oauthtoken"
	}

	password = tokenData.AccessToken

	return
}

// fetchOrganizationRecord fetches the metadata about what access does the token have on the provided organization.
func fetchOrganizationRecord(ctx context.Context, cl *http.Client, organization string, tokenData *api.Token, _ LoginTokenInfo) (*EntityRecord, error) {
	username, accessToken := getUsernameAndPasswordFromTokenData(tokenData)

	if username != "$oauthtoken" {
		return nil, nil
	}

	orgAdmin, err := hasOrgAdmin(ctx, cl, organization, accessToken)
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

func robotAccountRepositoryRecord(repository string, info LoginTokenInfo) (*EntityRecord, error) {
	repoInfo, ok := info.Repositories[repository]
	if !ok {
		return nil, nil
	}

	var possessedScopes []Scope
	if repoInfo.Pullable {
		possessedScopes = append(possessedScopes, ScopeRepoRead)
	}

	if repoInfo.Pushable {
		possessedScopes = append(possessedScopes, ScopeRepoWrite)
	}

	return &EntityRecord{
		PossessedScopes: possessedScopes,
	}, nil
}

func oauthRepositoryRecord(ctx context.Context, cl *http.Client, repository string, token string, info LoginTokenInfo) (*EntityRecord, error) {
	// first try to figure out repo:read and repo:write just by trying to log in
	rr, err := robotAccountRepositoryRecord(repository, info)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, nil
	}

	repoAdmin, err := hasRepoAdmin(ctx, cl, repository, token)
	if err != nil {
		return nil, err
	}
	if repoAdmin {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoAdmin)
	}

	repoCreate, err := hasRepoCreate(ctx, cl, repository, token)
	if err != nil {
		return nil, err
	}
	if repoCreate {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoCreate)
	}

	return rr, nil
}

func hasRepoCreate(ctx context.Context, cl *http.Client, repository string, token string) (bool, error) {
	slashIdx := strings.Index(repository, "/")
	var organization string
	if slashIdx >= 0 {
		organization = repository[0:slashIdx]
	}

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

func hasRepoAdmin(ctx context.Context, cl *http.Client, repository string, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/repository/"+repository+"/notification/", token)
}

func hasOrgAdmin(ctx context.Context, cl *http.Client, organization string, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, "https://quay.io/api/v1/organization/"+organization+"/robots?limit=1&token=false&permissions=false", token)
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
