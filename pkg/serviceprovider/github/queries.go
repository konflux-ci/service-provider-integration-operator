package github

import (
	"context"
	"github.com/machinebox/graphql"
)

const (
	allAccessibleReposQuery = `
		query($after: String) {
			viewer {
				repositories(first: 100, after: $after, affiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER], ownerAffiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER]) {
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
	viewer struct {
		repositories struct {
			pageInfo
			nodes []struct {
				viewerPermission string
				url              string
			}
		}
	}
}

func (r AllAccessibleRepos) FetchAll(ctx context.Context, client *graphql.Client, accessToken string) (TokenState, error) {
	results := TokenState{}

	req := _newAuthorizedRequest(accessToken, allAccessibleReposQuery)

	err := _fetchAll(ctx, client, req, &r, func() pageInfo {
		return r.viewer.repositories.pageInfo
	}, func() {
		for _, node := range r.viewer.repositories.nodes {
			results[RepositoryUrl(node.url)] = RepositoryRecord{ViewerPermission: ViewerPermission(node.viewerPermission)}
		}
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// pagePrelude should be reused in all the queries that need to be paged
type pageInfo struct {
	hasNextPage bool
	endCursor   string
}

func _fetchAll(ctx context.Context, client *graphql.Client, req *graphql.Request, self interface{}, pageInfoFromSelf func() pageInfo, processSelf func()) error {
	req.Var("after", nil)
	for {
		if err := client.Run(ctx, req, self); err != nil {
			return err
		}

		processSelf()

		page := pageInfoFromSelf()

		if !page.hasNextPage {
			return nil
		}

		req.Var("after", page.endCursor)
	}
}

func _newAuthorizedRequest(accessToken string, query string) *graphql.Request {
	req := graphql.NewRequest(query)

	req.Header.Set("Authorization", "Bearer"+accessToken)

	return req
}
