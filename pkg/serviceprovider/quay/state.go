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
	"io"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// EntityRecord stores the scopes possessed by some token for given "entity" (either repository or organization).
type EntityRecord struct {
	// LastRefreshTime is used to determine whether this record should be refreshed or not
	LastRefreshTime int64
	// PossessedScopes is the list of scopes possessed by the token on a given entity
	PossessedScopes []Scope
}

// TokenState represents the all the known scopes for all repositories for some token. This is persisted in the status
// of the SPIAccessToken object.
type TokenState struct {
	Repositories  map[string]EntityRecord
	Organizations map[string]EntityRecord
}

// Scope represents a Quay OAuth scope
type Scope string

const (
	OAuthTokenUserName       = "$oauthtoken"
	ScopeRepoRead      Scope = "repo:read"
	ScopeRepoWrite     Scope = "repo:write"
	ScopeRepoAdmin     Scope = "repo:admin"
	ScopeRepoCreate    Scope = "repo:create"
	ScopeUserRead      Scope = "user:read"
	ScopeUserAdmin     Scope = "user:admin"
	ScopeOrgAdmin      Scope = "org:admin"
	// These are not real scopes in Quay, but we represent the permissions of the robot tokens with them
	ScopePush Scope = "push"
	ScopePull Scope = "pull"
)

var descriptionNotStringError = errors.New("the repository data 'description' is not a string")

// Implies returns true if the scope implies the other scope. A scope implies itself.
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

// IsIncluded determines if a scope is included (either directly or through implication) in the provided list of scopes.
func (s Scope) IsIncluded(scopes []Scope) bool {
	for _, sc := range scopes {
		if sc.Implies(s) {
			return true
		}
	}

	return false
}

// fetchRepositoryRecord fetches the metadata about what access does the token have on the provided repository.
func fetchRepositoryRecord(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token, info LoginTokenInfo) (*EntityRecord, error) {
	username, password := getUsernameAndPasswordFromTokenData(tokenData)

	if username != OAuthTokenUserName {
		// we're dealing with robot account
		return robotAccountRepositoryRecord(ctx, repoUrl, info)
	} else {
		// we're dealing with an oauth token
		return oauthRepositoryRecord(ctx, cl, repoUrl, password)
	}
}

func getUsernameAndPasswordFromTokenData(tokenData *api.Token) (username, password string) {
	username = tokenData.Username

	if username == "" {
		username = OAuthTokenUserName
	}

	password = tokenData.AccessToken

	return
}

// fetchOrganizationRecord fetches the metadata about what access does the token have on the provided organization.
func fetchOrganizationRecord(ctx context.Context, cl *http.Client, organization string, tokenData *api.Token, _ LoginTokenInfo) (*EntityRecord, error) {
	lg := log.FromContext(ctx)

	username, accessToken := getUsernameAndPasswordFromTokenData(tokenData)

	if username != OAuthTokenUserName {
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

func oauthRepositoryRecord(ctx context.Context, cl *http.Client, repository string, token string) (*EntityRecord, error) {
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
	url := quayApiBaseUrl + "/repository/" + repository
	lg := log.FromContext(ctx, "url", url)

	resp, err := doQuayRequest(ctx, cl, url, token, "GET", nil, "")
	if err != nil {
		return false, nil, err
	}
	if resp == nil {
		return false, nil, nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body")
		}
	}()

	if resp.StatusCode != 200 {
		return false, nil, nil
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		lg.Error(err, "read the response")
		return false, nil, fmt.Errorf("failed to read the response from %s: %w", url, err)
	}
	data := map[string]interface{}{}
	if err = json.Unmarshal(bytes, &data); err != nil {
		lg.Error(err, "failed to read the response as JSON")
		return false, nil, fmt.Errorf("failed to unmarshal repo read scope check response: %w", err)
	}

	descriptionObj, ok := data["description"]
	if !ok {
		return true, nil, nil
	}

	var description *string
	if descriptionObj != nil {
		str, ok := descriptionObj.(string)
		if !ok {
			lg.Info("the repository data 'description' is not a string", "description", descriptionObj)
			return false, nil, fmt.Errorf("%w: %v", descriptionNotStringError, descriptionObj)
		}
		description = &str
	}

	return true, description, nil
}

func hasRepoWrite(ctx context.Context, cl *http.Client, repository string, token string, description *string) (bool, error) {
	url := quayApiBaseUrl + "/repository/" + repository
	lg := log.FromContext(ctx, "url", url)

	var val string
	if description == nil {
		val = "null"
	} else {
		val = `"` + *description + `"`
	}
	data := strings.NewReader(`{"description": ` + val + `}`)

	resp, err := doQuayRequest(ctx, cl, url, token, "PUT", data, "application/json")
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body")
		}
	}()

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

	resp, err := doQuayRequest(ctx, cl, url, token, "POST", data, "application/json")
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body")
		}
	}()

	// here, we exploit the order of input validation in Quay. The ability to write to a certain namespace is checked
	// before the validity of the repository name.
	// Therefore, if we can't write to the namespace, we get 403.
	// If we can write to the namespace, we get 400 because our repository name is invalid.
	return resp.StatusCode == 400, nil
}

func hasRepoAdmin(ctx context.Context, cl *http.Client, repository string, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, quayApiBaseUrl+"/repository/"+repository+"/notification/", token)
}

func hasOrgAdmin(ctx context.Context, cl *http.Client, organization string, token string) (bool, error) {
	return isSuccessfulRequest(ctx, cl, quayApiBaseUrl+"/organization/"+organization+"/robots?limit=1&token=false&permissions=false", token)
}
