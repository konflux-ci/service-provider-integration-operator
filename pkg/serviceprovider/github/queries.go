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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	"github.com/machinebox/graphql"
)

const (
	allAccessibleAffiliationsReposQuery = `
		query($after: String) {
			viewer {
				repositories(first: 100, after: $after, affiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER]) {
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						viewerPermission
						url
					}
				}
			}
		}`
	allAccessibleOwnerAffiliationsReposQuery = `
		query($after: String) {
			viewer {
				repositories(first: 100, after: $after, ownerAffiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER]) {
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						viewerPermission
						url
					}
				}
			}
		}`
)

// AllAccessibleRepos lists all the repositories accessible by the current user
type AllAccessibleRepos struct {
	Viewer struct {
		Repositories struct {
			PageInfo PageInfo `json:"pageInfo"`
			Nodes    []struct {
				ViewerPermission string `json:"viewerPermission"`
				Url              string `json:"url"`
			} `json:"nodes"`
		} `json:"repositories"`
	} `json:"viewer"`
}

func (r *AllAccessibleRepos) FetchAll(ctx context.Context, client *graphql.Client, accessToken string, state *TokenState) error {
	if accessToken == "" {
		return sperrors.ServiceProviderError{
			StatusCode: 401,
			Response:   "the access token is empty, service provider not contacted at all",
		}
	}

	ctx = httptransport.WithBearerToken(ctx, accessToken)

	if err := _fetchAll(r, ctx, client, allAccessibleAffiliationsReposQuery, state); err != nil {
		return err
	}

	if err := _fetchAll(r, ctx, client, allAccessibleOwnerAffiliationsReposQuery, state); err != nil {
		return err
	}
	lg := log.FromContext(ctx)
	lg.Info("_fetchAll state.AccessibleRepos.len=" + strconv.Itoa(len(state.AccessibleRepos)))
	return nil
}

// PageInfo should be reused in all the queries that need to be paged
type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func _fetchAll(r *AllAccessibleRepos, ctx context.Context, client *graphql.Client, request string, state *TokenState) error {

	lg := log.FromContext(ctx)
	err := _fetchAllWithPages(ctx, client, graphql.NewRequest(request), r, func() PageInfo {
		return r.Viewer.Repositories.PageInfo
	}, func() {
		for _, node := range r.Viewer.Repositories.Nodes {
			state.AccessibleRepos[RepositoryUrl(node.Url)] = RepositoryRecord{ViewerPermission: ViewerPermission(node.ViewerPermission)}
		}
	})
	if err != nil {

		lg.Error(err, "Error in FetchAll", "request", request)
		return err
	}
	return nil
}

func _fetchAllWithPages(ctx context.Context, client *graphql.Client, req *graphql.Request, self interface{}, pageInfoFromSelf func() PageInfo, processSelf func()) error {
	req.Var("after", nil)
	for {
		if err := client.Run(ctx, req, self); err != nil {
			return err
		}

		processSelf()

		page := pageInfoFromSelf()

		if !page.HasNextPage {
			return nil
		}

		req.Var("after", page.EndCursor)
	}
}
