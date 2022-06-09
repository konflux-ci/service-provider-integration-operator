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
	"strconv"

	"github.com/google/go-github/v45/github"
	"github.com/machinebox/graphql"
	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AllAccessibleRepos lists all the repositories accessible by the current user
type AllAccessibleRepos struct {
	Viewer struct {
		Repositories struct {
			//PageInfo PageInfo `json:"pageInfo"`
			Nodes []struct {
				ViewerPermission string `json:"viewerPermission"`
				Url              string `json:"url"`
			} `json:"nodes"`
		} `json:"repositories"`
	} `json:"viewer"`
}

func (r *AllAccessibleRepos) FetchAll(ctx context.Context, client *graphql.Client, accessToken string, state *TokenState) error {
	lg := log.FromContext(ctx)
	if accessToken == "" {
		return sperrors.ServiceProviderError{
			StatusCode: 401,
			Response:   "the access token is empty, service provider not contacted at all",
		}
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	githubClient := github.NewClient(tc)

	// list all repositories for the authenticated user
	opt := &github.RepositoryListOptions{}
	opt.ListOptions.Page = 0
	opt.ListOptions.PerPage = 100
	for {
		lg.Info("1")

		repos, resp, err := githubClient.Repositories.List(ctx, "", opt)
		lg.Info("2")
		if err != nil {
			lg.Info("3")

			lg.Error(err, "Error during fetching Github repositories list")
			return err
		}
		lg.Info("4", "resp.NextPage", resp.NextPage)
		lg.Info("5", "resp.len", len(repos))
		for _, k := range repos {
			if k.Permissions["admin"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionAdmin}
			} else if k.Permissions["push"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionWrite}
			} else if k.Permissions["pull"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionRead}
			} else {
				lg.Info("Unknown permission", "permission", k.Permissions)
			}

		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	lg.Info("_fetchAll 2", "state.AccessibleRepos.len", strconv.Itoa(len(state.AccessibleRepos)))
	lg.Info("state", "AccessibleRepos", state.AccessibleRepos)
	return nil
}
