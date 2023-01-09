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
	"io"
	"net/http"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"

	"github.com/stretchr/testify/assert"
)

var allScopes = []Scope{
	ScopeRepoRead,
	ScopeRepoWrite,
	ScopeRepoAdmin,
	ScopeRepoCreate,
	ScopeUserRead,
	ScopeUserAdmin,
	ScopeOrgAdmin,
	ScopePull,
	ScopePush,
}

func TestScope_Implies(t *testing.T) {
	testImplies := func(t *testing.T, scope Scope, impliedScopes ...Scope) {
		shouldImply := func(s Scope) bool {
			if scope == s {
				return true
			}

			for _, i := range impliedScopes {
				if i == s {
					return true
				}
			}

			return false
		}

		for _, s := range allScopes {
			assert.Equal(t, shouldImply(s), scope.Implies(s), "tested %s with %s", scope, s)
		}
	}

	t.Run(string(ScopeRepoRead), func(t *testing.T) {
		testImplies(t, ScopeRepoRead, ScopePull)
	})

	t.Run(string(ScopeRepoWrite), func(t *testing.T) {
		testImplies(t, ScopeRepoWrite, ScopeRepoRead, ScopePull, ScopePush)
	})

	t.Run(string(ScopeRepoAdmin), func(t *testing.T) {
		testImplies(t, ScopeRepoAdmin, ScopeRepoWrite, ScopeRepoRead, ScopeRepoCreate, ScopePull, ScopePush)
	})

	t.Run(string(ScopeRepoCreate), func(t *testing.T) {
		testImplies(t, ScopeRepoCreate /* nothing implied */)
	})

	t.Run(string(ScopeUserRead), func(t *testing.T) {
		testImplies(t, ScopeUserRead /* nothing implied */)
	})

	t.Run(string(ScopeUserAdmin), func(t *testing.T) {
		testImplies(t, ScopeUserAdmin, ScopeUserRead)
	})

	t.Run(string(ScopeOrgAdmin), func(t *testing.T) {
		testImplies(t, ScopeOrgAdmin /* nothing implied */)
	})
}

func TestScope_IsIncluded(t *testing.T) {
	t.Run("handles implied scopes", func(t *testing.T) {
		assert.True(t, ScopeRepoRead.IsIncluded([]Scope{ScopeRepoWrite, ScopeUserAdmin}))
	})

	t.Run("simple equality", func(t *testing.T) {
		assert.True(t, ScopeRepoWrite.IsIncluded([]Scope{ScopeRepoWrite, ScopeUserAdmin}))
	})
}

func TestFetchRepositoryRecord(t *testing.T) {
	repo := "testorg/repo"
	repoWithDescription := `{"description": "asdf"}`
	repoWithNullDescription := `{"description": null}`
	repoWithoutDescription := `{}`

	testingHttpClient := func(repoCreateTested, repoWriteTested, repoReadTested, repoAdminTested *bool, repoDetailsBody string) *http.Client {
		return &http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == config.ServiceProviderTypeQuay.DefaultHost {
					auth := r.Header.Get("Authorization")
					assert.Equal(t, "Bearer token", auth)
					if r.URL.Path == "/api/v1/repository" {
						*repoCreateTested = true
						return &http.Response{StatusCode: 400}, nil
					} else if r.URL.Path == "/api/v1/repository/"+repo {
						if r.Method == "PUT" {
							*repoWriteTested = true
							return &http.Response{StatusCode: 200}, nil
						} else if r.Method == "GET" {
							*repoReadTested = true
							return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(repoDetailsBody))}, nil
						}
					} else if r.URL.Path == "/api/v1/repository/"+repo+"/notification/" {
						*repoAdminTested = true
						return &http.Response{StatusCode: 200}, nil
					}
				}

				assert.Fail(t, "unexpected request", "url", r.URL)
				return nil, nil
			}),
		}
	}

	t.Run("robot account", func(t *testing.T) {
		var repoCreateTested, repoAdminTested, repoWriteTested, repoReadTested bool
		httpClient := testingHttpClient(&repoCreateTested, &repoWriteTested, &repoReadTested, &repoAdminTested, repoWithDescription)

		rec, err := fetchRepositoryRecord(context.TODO(), httpClient, repo, &api.Token{
			Username:    "test+test",
			AccessToken: "token",
		}, LoginTokenInfo{
			Username: "test+test",
			Repositories: map[string]LoginTokenRepositoryInfo{
				repo: {
					Pushable: true,
					Pullable: true,
				},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, rec)
		assert.Equal(t, 2, len(rec.PossessedScopes))
		assert.Contains(t, rec.PossessedScopes, ScopePull)
		assert.Contains(t, rec.PossessedScopes, ScopePush)
		assert.False(t, repoCreateTested)
		assert.False(t, repoWriteTested)
		assert.False(t, repoReadTested)
		assert.False(t, repoAdminTested)
	})

	t.Run("oauth app", func(t *testing.T) {
		var repoCreateTested, repoAdminTested, repoWriteTested, repoReadTested bool
		httpClient := testingHttpClient(&repoCreateTested, &repoWriteTested, &repoReadTested, &repoAdminTested, repoWithDescription)

		rec, err := fetchRepositoryRecord(context.TODO(), httpClient, repo, &api.Token{
			Username:    "",
			AccessToken: "token",
		}, LoginTokenInfo{
			Username: "",
			Repositories: map[string]LoginTokenRepositoryInfo{
				repo: {
					Pushable: true,
					Pullable: true,
				},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, rec)
		assert.Equal(t, 6, len(rec.PossessedScopes))
		assert.Contains(t, rec.PossessedScopes, ScopePush)
		assert.Contains(t, rec.PossessedScopes, ScopePull)
		assert.Contains(t, rec.PossessedScopes, ScopeRepoRead)
		assert.Contains(t, rec.PossessedScopes, ScopeRepoWrite)
		assert.Contains(t, rec.PossessedScopes, ScopeRepoCreate)
		assert.Contains(t, rec.PossessedScopes, ScopeRepoAdmin)
		assert.True(t, repoCreateTested)
		assert.True(t, repoWriteTested)
		assert.True(t, repoReadTested)
		assert.True(t, repoAdminTested)
	})

	t.Run("null repo description", func(t *testing.T) {
		var unused bool
		httpClient := testingHttpClient(&unused, &unused, &unused, &unused, repoWithNullDescription)

		rec, err := fetchRepositoryRecord(context.TODO(), httpClient, repo, &api.Token{
			Username:    "",
			AccessToken: "token",
		}, LoginTokenInfo{
			Username: "",
			Repositories: map[string]LoginTokenRepositoryInfo{
				repo: {
					Pushable: true,
					Pullable: true,
				},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, rec)
	})

	t.Run("no repo description", func(t *testing.T) {
		var unused bool
		httpClient := testingHttpClient(&unused, &unused, &unused, &unused, repoWithoutDescription)

		rec, err := fetchRepositoryRecord(context.TODO(), httpClient, repo, &api.Token{
			Username:    "",
			AccessToken: "token",
		}, LoginTokenInfo{
			Username: "",
			Repositories: map[string]LoginTokenRepositoryInfo{
				repo: {
					Pushable: true,
					Pullable: true,
				},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, rec)
	})
}

func TestFetchOrganizationRecord(t *testing.T) {
	org := "testorg"
	httpClient := http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == config.ServiceProviderTypeQuay.DefaultHost && r.URL.Path == "/api/v1/organization/"+org+"/robots" {
				auth := r.Header.Get("Authorization")
				assert.Equal(t, "Bearer token", auth)
				return &http.Response{StatusCode: 200}, nil
			}

			assert.Fail(t, "unexpected request", "url", r.URL)
			return nil, nil
		}),
	}

	rec, err := fetchOrganizationRecord(context.TODO(), &httpClient, org, &api.Token{
		AccessToken: "token",
	}, LoginTokenInfo{
		Username:     "",
		Repositories: map[string]LoginTokenRepositoryInfo{},
	})

	assert.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, 1, len(rec.PossessedScopes))
	assert.Equal(t, ScopeOrgAdmin, rec.PossessedScopes[0])
}

func TestSplitToOrganizationAndRepositoryAndVersion(t *testing.T) {
	cases := map[string][]string{
		"quay.io/org/repo:latest":                    {"org", "repo", "latest"},
		"quay.io/org/repo":                           {"org", "repo", ""},
		"http://quay.io/org/repo:latest":             {"org", "repo", "latest"},
		"http://quay.io/org/repo":                    {"org", "repo", ""},
		"https://quay.io/org/repo:latest":            {"org", "repo", "latest"},
		"https://quay.io/org/repo":                   {"org", "repo", ""},
		"http://quay.io/repository/org/repo:latest":  {"org", "repo", "latest"},
		"http://quay.io/repository/org/repo":         {"org", "repo", ""},
		"https://quay.io/repository/org/repo:latest": {"org", "repo", "latest"},
		"https://quay.io/repository/org/repo":        {"org", "repo", ""},
		"docker.io/org/repo":                         {"", "", ""},
		"https:/docker.io/org/repo":                  {"", "", ""},
		"google.com":                                 {"", "", ""},
		"completely invalid something":               {"", "", ""},
	}

	for url, expected := range cases {
		t.Run(url, func(t *testing.T) {
			org, repo, version := splitToOrganizationAndRepositoryAndVersion(url)
			assert.Equal(t, expected[0], org)
			assert.Equal(t, expected[1], repo)
			assert.Equal(t, expected[2], version)
		})
	}
}
