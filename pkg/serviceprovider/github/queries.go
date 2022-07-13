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
	"time"

	"github.com/google/go-github/v45/github"
	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AllAccessibleRepos lists all the repositories accessible by the current user
type AllAccessibleRepos struct {
	Viewer struct {
		Repositories struct {
			Nodes []struct {
				ViewerPermission string `json:"viewerPermission"`
				Url              string `json:"url"`
			} `json:"nodes"`
		} `json:"repositories"`
	} `json:"viewer"`
}

func (r *AllAccessibleRepos) FetchAll(ctx context.Context, githubClient *github.Client, accessToken string, state *TokenState) error {

	lg := log.FromContext(ctx)
	defer logs.TimeTrack(lg, time.Now(), "fetch all github repositories")
	if accessToken == "" {
		return sperrors.ServiceProviderHttpError{
			StatusCode: 401,
			Response:   "the access token is empty, service provider not contacted at all",
		}
	}
	lg.V(logs.DebugLevel).Info("Fetching metadata request")
	// list all repositories for the authenticated user
	opt := &github.RepositoryListOptions{}
	opt.ListOptions.PerPage = 100
	for {
		repos, resp, err := githubClient.Repositories.List(ctx, "", opt)
		if err != nil {
			lg.Error(err, "Error during fetching Github repositories list")
			return fmt.Errorf("failed to list github repositories: %w", err)
		}

		lg.V(logs.DebugLevel).Info("Received a list of available repositories from Github", "len", len(repos), "nextPage", resp.NextPage, "lastPage", resp.LastPage, "rate", resp.Rate)
		for _, k := range repos {
			if k.Permissions["admin"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionAdmin}
			} else if k.Permissions["push"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionWrite}
			} else if k.Permissions["pull"] {
				state.AccessibleRepos[RepositoryUrl(*k.HTMLURL)] = RepositoryRecord{ViewerPermission: ViewerPermissionRead}
			}

		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	lg.V(logs.DebugLevel).Info("Fetching metadata complete", "len", len(state.AccessibleRepos))
	return nil
}
