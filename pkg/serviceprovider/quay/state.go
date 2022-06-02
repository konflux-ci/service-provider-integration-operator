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
	"fmt"
	"io"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

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
	// These are not real scopes in Quay, but we represent the permissions of the robot tokens with them
	ScopePush Scope = "push"
	ScopePull Scope = "pull"
)

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}

	switch s {
	case ScopeRepoRead:
		return other == ScopePull
	case ScopeRepoWrite:
		return other == ScopeRepoRead || other == ScopePush || other == ScopePull
	case ScopeRepoAdmin:
		return other == ScopeRepoRead || other == ScopeRepoWrite || other == ScopeRepoCreate || other == ScopePush || other == ScopePull
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
		return robotAccountRepositoryRecord(ctx, repoUrl, info)
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
	lg := log.FromContext(ctx)

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

	lg.Info("detected org-level scopes", "scopes", scopes)
	return &EntityRecord{
		PossessedScopes: scopes,
	}, nil
}

func robotAccountRepositoryRecord(ctx context.Context, repository string, info LoginTokenInfo) (*EntityRecord, error) {
	repoInfo, ok := info.Repositories[repository]
	if !ok {
		return nil, nil
	}

	var possessedScopes []Scope
	if repoInfo.Pullable {
		possessedScopes = append(possessedScopes, ScopePull)
	}

	if repoInfo.Pushable {
		possessedScopes = append(possessedScopes, ScopePush)
	}

	log.FromContext(ctx).Info("detected robot-account-compatible scopes", "scopes", possessedScopes)

	return &EntityRecord{
		PossessedScopes: possessedScopes,
	}, nil
}

func oauthRepositoryRecord(ctx context.Context, cl *http.Client, repository string, token string, info LoginTokenInfo) (*EntityRecord, error) {
	lg := log.FromContext(ctx)

	rr := &EntityRecord{}

	repoRead, description, err := hasRepoRead(ctx, cl, repository, token)
	if err != nil {
		lg.Error(err, "failed to detect repo:read scope")
		return nil, err
	}
	if repoRead {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoRead, ScopePull)
	}

	var repoWrite bool
	if repoRead {
		repoWrite, err = hasRepoWrite(ctx, cl, repository, token, description)
		if err != nil {
			lg.Error(err, "failed to detect repo:write scope")
			return nil, err
		}
		if repoWrite {
			rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoWrite, ScopePush)
		}
	}

	repoAdmin, err := hasRepoAdmin(ctx, cl, repository, token)
	if err != nil {
		lg.Error(err, "failed to detect repo:admin scope")
		return nil, err
	}
	if repoAdmin {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoAdmin)
	}

	repoCreate, err := hasRepoCreate(ctx, cl, repository, token)
	if err != nil {
		lg.Error(err, "failed to detect repo:create scope")
		return nil, err
	}
	if repoCreate {
		rr.PossessedScopes = append(rr.PossessedScopes, ScopeRepoCreate)
	}

	lg.Info("detected OAuth token scopes", "scopes", rr.PossessedScopes)

	return rr, nil
}
func hasRepoRead(ctx context.Context, cl *http.Client, repository string, token string) (bool, *string, error) {
	url := "https://quay.io/api/v1/repository/" + repository

	resp, err := doQuayRequest(ctx, cl, url, token, "GET", nil)
	if err != nil {
		return false, nil, err
	}
	if resp == nil {
		return false, nil, nil
	}

	if resp.StatusCode != 200 {
		return false, nil, nil
	}

	lg := log.FromContext(ctx, "url", url)

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		lg.Error(err, "read the response")
		return false, nil, err
	}
	data := map[string]interface{}{}
	if err = json.Unmarshal(bytes, &data); err != nil {
		lg.Error(err, "failed to read the response as JSON")
		return false, nil, err
	}

	descriptionObj, ok := data["description"]
	if !ok {
		return true, nil, nil
	}

	var description *string
	if descriptionObj != nil {
		str, ok := descriptionObj.(string)
		if !ok {
			lg.Info("the repository data 'description' is not a string")
			return false, nil, fmt.Errorf("the repository data 'description' is not a string: %v", descriptionObj)
		}
		description = &str
	}

	return true, description, nil
}

func hasRepoWrite(ctx context.Context, cl *http.Client, repository string, token string, description *string) (bool, error) {
	url := "https://quay.io/api/v1/repository/" + repository

	var val string
	if description == nil {
		val = "null"
	} else {
		val = `"` + *description + `"`
	}
	data := strings.NewReader(`{"description": ` + val + `}`)

	resp, err := doQuayRequest(ctx, cl, url, token, "PUT", data)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.StatusCode == 200, nil
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

	url := "https://quay.io/api/v1/repository"

	lg := log.FromContext(ctx, "url", url)

	lg.Info("asking quay API")

	resp, err := doQuayRequest(ctx, cl, url, token, "POST", data)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
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
	resp, err := doQuayRequest(ctx, cl, url, token, "GET", nil)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.StatusCode == 200, nil
}

func doQuayRequest(ctx context.Context, cl *http.Client, url string, token string, method string, body io.Reader) (*http.Response, error) {
	lg := log.FromContext(ctx, "url", url)

	lg.Info("asking quay API")

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		lg.Error(err, "failed to compose the request")
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cl.Do(req)
	if err != nil {
		lg.Error(err, "failed to perform the request")
		return nil, err
	}

	return resp, nil
}

// splitToOrganizationAndRepositoryAndVersion tries to parse the provided repository ID into the organization,
// repository and version parts. It supports both scheme-less docker-like repository ID and URLs of the repositories in
// the quay UI.
func splitToOrganizationAndRepositoryAndVersion(repository string) (string, string, string) {
	schemeIndex := strings.Index(repository, "://")
	if schemeIndex > 0 {
		repository = repository[(schemeIndex + 3):]
	}

	parts := strings.Split(repository, "/")
	if len(parts) < 3 {
		return "", "", ""
	}

	isUIUrl := len(parts) == 4 && parts[1] == "repository"
	if !isUIUrl && len(parts) == 4 {
		return "", "", ""
	}

	host := parts[0]

	if host != "quay.io" {
		return "", "", ""
	}

	repo := parts[1]
	img := parts[2]
	if isUIUrl {
		repo = parts[2]
		img = parts[3]
	}

	imgParts := strings.Split(img, ":")
	version := ""
	if len(imgParts) == 2 {
		version = imgParts[1]
	}

	return repo, imgParts[0], version
}
