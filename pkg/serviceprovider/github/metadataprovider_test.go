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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

const githubRepoListResponse = `[
  {
    "id": 316432537,
    "node_id": "MDEwOlJlcG9zaXRvcnkzMTY0MzI1Mzc=",
    "name": "app-interface",
    "full_name": "app-sre/app-interface",
    "private": false,
    "owner": {
      "login": "app-sre",
      "id": 43133889,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjQzMTMzODg5",
      "avatar_url": "https://avatars.githubusercontent.com/u/43133889?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/app-sre",
      "html_url": "https://github.com/app-sre",
      "followers_url": "https://api.github.com/users/app-sre/followers",
      "following_url": "https://api.github.com/users/app-sre/following{/other_user}",
      "gists_url": "https://api.github.com/users/app-sre/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/app-sre/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/app-sre/subscriptions",
      "organizations_url": "https://api.github.com/users/app-sre/orgs",
      "repos_url": "https://api.github.com/users/app-sre/repos",
      "events_url": "https://api.github.com/users/app-sre/events{/privacy}",
      "received_events_url": "https://api.github.com/users/app-sre/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "html_url": "https://github.com/app-sre/app-interface",
    "description": null,
    "fork": false,
    "url": "https://api.github.com/repos/app-sre/app-interface",
    "forks_url": "https://api.github.com/repos/app-sre/app-interface/forks",
    "keys_url": "https://api.github.com/repos/app-sre/app-interface/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/app-sre/app-interface/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/app-sre/app-interface/teams",
    "hooks_url": "https://api.github.com/repos/app-sre/app-interface/hooks",
    "issue_events_url": "https://api.github.com/repos/app-sre/app-interface/issues/events{/number}",
    "events_url": "https://api.github.com/repos/app-sre/app-interface/events",
    "assignees_url": "https://api.github.com/repos/app-sre/app-interface/assignees{/user}",
    "branches_url": "https://api.github.com/repos/app-sre/app-interface/branches{/branch}",
    "tags_url": "https://api.github.com/repos/app-sre/app-interface/tags",
    "blobs_url": "https://api.github.com/repos/app-sre/app-interface/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/app-sre/app-interface/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/app-sre/app-interface/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/app-sre/app-interface/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/app-sre/app-interface/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/app-sre/app-interface/languages",
    "stargazers_url": "https://api.github.com/repos/app-sre/app-interface/stargazers",
    "contributors_url": "https://api.github.com/repos/app-sre/app-interface/contributors",
    "subscribers_url": "https://api.github.com/repos/app-sre/app-interface/subscribers",
    "subscription_url": "https://api.github.com/repos/app-sre/app-interface/subscription",
    "commits_url": "https://api.github.com/repos/app-sre/app-interface/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/app-sre/app-interface/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/app-sre/app-interface/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/app-sre/app-interface/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/app-sre/app-interface/contents/{+path}",
    "compare_url": "https://api.github.com/repos/app-sre/app-interface/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/app-sre/app-interface/merges",
    "archive_url": "https://api.github.com/repos/app-sre/app-interface/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/app-sre/app-interface/downloads",
    "issues_url": "https://api.github.com/repos/app-sre/app-interface/issues{/number}",
    "pulls_url": "https://api.github.com/repos/app-sre/app-interface/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/app-sre/app-interface/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/app-sre/app-interface/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/app-sre/app-interface/labels{/name}",
    "releases_url": "https://api.github.com/repos/app-sre/app-interface/releases{/id}",
    "deployments_url": "https://api.github.com/repos/app-sre/app-interface/deployments",
    "created_at": "2020-11-27T07:39:05Z",
    "updated_at": "2022-01-10T18:25:47Z",
    "pushed_at": "2022-05-02T06:47:30Z",
    "git_url": "git://github.com/app-sre/app-interface.git",
    "ssh_url": "git@github.com:app-sre/app-interface.git",
    "clone_url": "https://github.com/app-sre/app-interface.git",
    "svn_url": "https://github.com/app-sre/app-interface",
    "homepage": null,
    "size": 173,
    "stargazers_count": 4,
    "watchers_count": 4,
    "language": "Python",
    "has_issues": true,
    "has_projects": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": false,
    "forks_count": 9,
    "mirror_url": null,
    "archived": false,
    "disabled": false,
    "open_issues_count": 5,
    "license": {
      "key": "apache-2.0",
      "name": "Apache License 2.0",
      "spdx_id": "Apache-2.0",
      "url": "https://api.github.com/licenses/apache-2.0",
      "node_id": "MDc6TGljZW5zZTI="
    },
    "allow_forking": true,
    "is_template": false,
    "topics": [

    ],
    "visibility": "public",
    "forks": 9,
    "open_issues": 5,
    "watchers": 4,
    "default_branch": "main",
    "permissions": {
      "admin": false,
      "maintain": false,
      "push": false,
      "triage": false,
      "pull": true
    }
  },
  {
    "id": 245264840,
    "node_id": "MDEwOlJlcG9zaXRvcnkyNDUyNjQ4NDA=",
    "name": "app-sre-errbot",
    "full_name": "app-sre/app-sre-errbot",
    "private": false,
    "owner": {
      "login": "app-sre",
      "id": 43133889,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjQzMTMzODg5",
      "avatar_url": "https://avatars.githubusercontent.com/u/43133889?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/app-sre",
      "html_url": "https://github.com/app-sre",
      "followers_url": "https://api.github.com/users/app-sre/followers",
      "following_url": "https://api.github.com/users/app-sre/following{/other_user}",
      "gists_url": "https://api.github.com/users/app-sre/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/app-sre/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/app-sre/subscriptions",
      "organizations_url": "https://api.github.com/users/app-sre/orgs",
      "repos_url": "https://api.github.com/users/app-sre/repos",
      "events_url": "https://api.github.com/users/app-sre/events{/privacy}",
      "received_events_url": "https://api.github.com/users/app-sre/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "html_url": "https://github.com/app-sre/app-sre-errbot",
    "description": "App-SRE bot built using Errbot",
    "fork": false,
    "url": "https://api.github.com/repos/app-sre/app-sre-errbot",
    "forks_url": "https://api.github.com/repos/app-sre/app-sre-errbot/forks",
    "keys_url": "https://api.github.com/repos/app-sre/app-sre-errbot/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/app-sre/app-sre-errbot/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/app-sre/app-sre-errbot/teams",
    "hooks_url": "https://api.github.com/repos/app-sre/app-sre-errbot/hooks",
    "issue_events_url": "https://api.github.com/repos/app-sre/app-sre-errbot/issues/events{/number}",
    "events_url": "https://api.github.com/repos/app-sre/app-sre-errbot/events",
    "assignees_url": "https://api.github.com/repos/app-sre/app-sre-errbot/assignees{/user}",
    "branches_url": "https://api.github.com/repos/app-sre/app-sre-errbot/branches{/branch}",
    "tags_url": "https://api.github.com/repos/app-sre/app-sre-errbot/tags",
    "blobs_url": "https://api.github.com/repos/app-sre/app-sre-errbot/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/app-sre/app-sre-errbot/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/app-sre/app-sre-errbot/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/app-sre/app-sre-errbot/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/app-sre/app-sre-errbot/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/app-sre/app-sre-errbot/languages",
    "stargazers_url": "https://api.github.com/repos/app-sre/app-sre-errbot/stargazers",
    "contributors_url": "https://api.github.com/repos/app-sre/app-sre-errbot/contributors",
    "subscribers_url": "https://api.github.com/repos/app-sre/app-sre-errbot/subscribers",
    "subscription_url": "https://api.github.com/repos/app-sre/app-sre-errbot/subscription",
    "commits_url": "https://api.github.com/repos/app-sre/app-sre-errbot/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/app-sre/app-sre-errbot/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/app-sre/app-sre-errbot/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/app-sre/app-sre-errbot/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/app-sre/app-sre-errbot/contents/{+path}",
    "compare_url": "https://api.github.com/repos/app-sre/app-sre-errbot/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/app-sre/app-sre-errbot/merges",
    "archive_url": "https://api.github.com/repos/app-sre/app-sre-errbot/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/app-sre/app-sre-errbot/downloads",
    "issues_url": "https://api.github.com/repos/app-sre/app-sre-errbot/issues{/number}",
    "pulls_url": "https://api.github.com/repos/app-sre/app-sre-errbot/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/app-sre/app-sre-errbot/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/app-sre/app-sre-errbot/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/app-sre/app-sre-errbot/labels{/name}",
    "releases_url": "https://api.github.com/repos/app-sre/app-sre-errbot/releases{/id}",
    "deployments_url": "https://api.github.com/repos/app-sre/app-sre-errbot/deployments",
    "created_at": "2020-03-05T20:49:10Z",
    "updated_at": "2022-03-17T18:19:50Z",
    "pushed_at": "2021-05-21T11:19:54Z",
    "git_url": "git://github.com/app-sre/app-sre-errbot.git",
    "ssh_url": "git@github.com:app-sre/app-sre-errbot.git",
    "clone_url": "https://github.com/app-sre/app-sre-errbot.git",
    "svn_url": "https://github.com/app-sre/app-sre-errbot",
    "homepage": null,
    "size": 42,
    "stargazers_count": 1,
    "watchers_count": 1,
    "language": "Python",
    "has_issues": true,
    "has_projects": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": false,
    "forks_count": 2,
    "mirror_url": null,
    "archived": true,
    "disabled": false,
    "open_issues_count": 1,
    "license": {
      "key": "apache-2.0",
      "name": "Apache License 2.0",
      "spdx_id": "Apache-2.0",
      "url": "https://api.github.com/licenses/apache-2.0",
      "node_id": "MDc6TGljZW5zZTI="
    },
    "allow_forking": true,
    "is_template": false,
    "topics": [

    ],
    "visibility": "public",
    "forks": 2,
    "open_issues": 1,
    "watchers": 1,
    "default_branch": "master",
    "permissions": {
      "admin": false,
      "maintain": false,
      "push": false,
      "triage": false,
      "pull": true
    }
  },
  {
    "id": 202777436,
    "node_id": "MDEwOlJlcG9zaXRvcnkyMDI3Nzc0MzY=",
    "name": "aws-resource-exporter",
    "full_name": "app-sre/aws-resource-exporter",
    "private": false,
    "owner": {
      "login": "app-sre",
      "id": 43133889,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjQzMTMzODg5",
      "avatar_url": "https://avatars.githubusercontent.com/u/43133889?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/app-sre",
      "html_url": "https://github.com/app-sre",
      "followers_url": "https://api.github.com/users/app-sre/followers",
      "following_url": "https://api.github.com/users/app-sre/following{/other_user}",
      "gists_url": "https://api.github.com/users/app-sre/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/app-sre/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/app-sre/subscriptions",
      "organizations_url": "https://api.github.com/users/app-sre/orgs",
      "repos_url": "https://api.github.com/users/app-sre/repos",
      "events_url": "https://api.github.com/users/app-sre/events{/privacy}",
      "received_events_url": "https://api.github.com/users/app-sre/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "html_url": "https://github.com/app-sre/aws-resource-exporter",
    "description": "Prometheus exporter for AWS resource metadata and metrics which are not available in Cloudwatch",
    "fork": false,
    "url": "https://api.github.com/repos/app-sre/aws-resource-exporter",
    "forks_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/forks",
    "keys_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/teams",
    "hooks_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/hooks",
    "issue_events_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/issues/events{/number}",
    "events_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/events",
    "assignees_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/assignees{/user}",
    "branches_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/branches{/branch}",
    "tags_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/tags",
    "blobs_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/languages",
    "stargazers_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/stargazers",
    "contributors_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/contributors",
    "subscribers_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/subscribers",
    "subscription_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/subscription",
    "commits_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/contents/{+path}",
    "compare_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/merges",
    "archive_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/downloads",
    "issues_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/issues{/number}",
    "pulls_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/labels{/name}",
    "releases_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/releases{/id}",
    "deployments_url": "https://api.github.com/repos/app-sre/aws-resource-exporter/deployments",
    "created_at": "2019-08-16T18:11:20Z",
    "updated_at": "2022-03-28T14:07:31Z",
    "pushed_at": "2022-06-02T12:18:20Z",
    "git_url": "git://github.com/app-sre/aws-resource-exporter.git",
    "ssh_url": "git@github.com:app-sre/aws-resource-exporter.git",
    "clone_url": "https://github.com/app-sre/aws-resource-exporter.git",
    "svn_url": "https://github.com/app-sre/aws-resource-exporter",
    "homepage": null,
    "size": 2690,
    "stargazers_count": 6,
    "watchers_count": 6,
    "language": "Go",
    "has_issues": true,
    "has_projects": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": false,
    "forks_count": 10,
    "mirror_url": null,
    "archived": false,
    "disabled": false,
    "open_issues_count": 0,
    "license": {
      "key": "apache-2.0",
      "name": "Apache License 2.0",
      "spdx_id": "Apache-2.0",
      "url": "https://api.github.com/licenses/apache-2.0",
      "node_id": "MDc6TGljZW5zZTI="
    },
    "allow_forking": true,
    "is_template": false,
    "topics": [

    ],
    "visibility": "public",
    "forks": 10,
    "open_issues": 0,
    "watchers": 6,
    "default_branch": "master",
    "permissions": {
      "admin": false,
      "maintain": false,
      "push": false,
      "triage": false,
      "pull": true
    }
  }
]
`

var githubUserApiEndpoint *url.URL

func init() {
	githubUrl, err := url.Parse("https://api.github.com/user")
	if err != nil {
		panic(err)
	}
	githubUserApiEndpoint = githubUrl
}

func TestMetadataProvider_Fetch(t *testing.T) {
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {

			if r.URL.String() == githubUserApiEndpoint.String() {
				return &http.Response{
					StatusCode: 200,
					Header: map[string][]string{
						// the letter case is important here, http client is sensitive to this
						"X-Oauth-Scopes": {"a, b, c, d"},
					},
					Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "login": "test_user"}`))),
				}, nil
			} else {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(githubRepoListResponse))),
				}, nil
			}
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	githubClientBuilder := githubClientBuilder{
		httpClient:   httpCl,
		tokenStorage: ts,
	}

	mp := metadataProvider{
		ghClientBuilder: githubClientBuilder,
		httpClient:      httpCl,
		tokenStorage:    &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "42", data.UserId)
	assert.Equal(t, "test_user", data.Username)
	assert.Equal(t, []string{"a", "b", "c", "d"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)

	tokenState := &TokenState{}
	assert.NoError(t, json.Unmarshal(data.ServiceProviderState, tokenState))
	assert.Equal(t, 3, len(tokenState.AccessibleRepos))
	val, ok := tokenState.AccessibleRepos["https://github.com/app-sre/app-interface"]
	assert.True(t, ok)
	assert.Equal(t, RepositoryRecord{ViewerPermission: "READ"}, val)
}

func TestMetadataProvider_Fetch_User_fail(t *testing.T) {
	expectedError := errors.New("math: square root of negative number")
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == githubUserApiEndpoint.String() {
				return nil, expectedError
			} else {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(githubRepoListResponse))),
				}, nil
			}
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	githubClientBuilder := githubClientBuilder{
		httpClient:   httpCl,
		tokenStorage: ts,
	}

	mp := metadataProvider{
		ghClientBuilder: githubClientBuilder,
		httpClient:      httpCl,
		tokenStorage:    &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.Error(t, err, expectedError)
	assert.Nil(t, data)
}

func TestMetadataProvider_FetchAll_fail(t *testing.T) {
	expectedError := errors.New("math: square root of negative number")
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == githubUserApiEndpoint.String() {
				return &http.Response{
					StatusCode: 200,
					Header: map[string][]string{
						// the letter case is important here, http client is sensitive to this
						"X-Oauth-Scopes": {"a, b, c, d"},
					},
					Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "login": "test_user"}`))),
				}, nil
			} else {
				return nil, expectedError
			}
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	githubClientBuilder := githubClientBuilder{
		httpClient:   httpCl,
		tokenStorage: ts,
	}

	mp := metadataProvider{
		ghClientBuilder: githubClientBuilder,
		httpClient:      httpCl,
		tokenStorage:    &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.Error(t, err, expectedError)
	assert.Nil(t, data)
}
