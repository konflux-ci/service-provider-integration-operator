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
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	"github.com/stretchr/testify/assert"
)

const repositoriesAffiliationsFakeResponse = `
{
  "data": {
    "viewer": {
      "repositories": {
        "pageInfo": {
          "hasNextPage": false,
          "endCursor": ""
        },
        "nodes": [
          {
            "viewerPermission": "ADMIN",
            "url": "https://github.com/jdoe/RHQ-old"
          },
          {
            "viewerPermission": "WRITE",
            "url": "https://github.com/openshiftio/openshift.io"
          },
          {
            "viewerPermission": "READ",
            "url": "https://github.com/openshiftio/booster-parent"
          },
          {
            "viewerPermission": "READ",
            "url": "https://github.com/redhat-developer/opencompose-old"
          },
          {
            "viewerPermission": "ADMIN",
            "url": "https://github.com/jdoe/emonTH"
          },
          {
            "viewerPermission": "WRITE",
            "url": "https://github.com/redhat-developer/rh-che"
          },
          {
            "viewerPermission": "ADMIN",
            "url": "https://github.com/jdoe/far2go"
          }
        ]
      }
    }
  }
}
`
const repositoriesOwnerAffiliationsFakeResponse = `
{
  "data": {
    "viewer": {
      "repositories": {
        "pageInfo": {
          "hasNextPage": false,
          "endCursor": ""
        },
        "nodes": [
         {
            "viewerPermission": "READ",
            "url": "https://github.com/eclipse/jdtc"
          },
          {
            "viewerPermission": "READ",
            "url": "https://github.com/eclipse/manifest"
          },
          {
            "viewerPermission": "ADMIN",
            "url": "https://github.com/jdoe/mockitong"
          },
          {
            "viewerPermission": "ADMIN",
            "url": "https://github.com/jdoe/everrest-assured"
          }
        ]
      }
    }
  }
}
`

func Test1(t *testing.T) {
	ctx := context.Background()
	//ts := oauth2.StaticTokenSource(
	//	&oauth2.Token{AccessToken:os.Getenv("???")},
	//)
	//tc := oauth2.NewClient(ctx, ts)
	httpClient := serviceprovider.AuthenticatingHttpClient(&http.Client{})
	authenticatedContext := httptransport.WithBearerToken(ctx, os.Getenv("??"))
	client := github.NewClient(httpClient)

	// list all repositories for the authenticated user
	opt := &github.RepositoryListOptions{}
	//opt.ListOptions.Page = 0
	opt.ListOptions.PerPage = 50
	var reposCount = 0
	for {
		repos, resp, err := client.Repositories.List(authenticatedContext, "", opt)
		if err != nil {
			fmt.Println(err)
		}

		for _, k := range repos {
			//fmt.Println( *k.CloneURL, k.Permissions)
			fmt.Println(*k.HTMLURL)
			reposCount++
		}
		//	opts.Since = *repos[len(repos)-1].ID
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	fmt.Println("repos:", reposCount)
}
func TestAllAccessibleRepos(t *testing.T) {
	aar := &AllAccessibleRepos{}

	ts := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetUsersByUsername,
			github.User{
				Name: github.String("foobar"),
			},
		),
		mock.WithRequestMatch(
			mock.GetUserRepos,
			[]github.Repository{
				{
					Name:    github.String("RHQ-old"),
					HTMLURL: github.String("https://github.com/jdoe/RHQ-old"),
					Permissions: map[string]bool{
						"admin": true,
					},
				},
			},
		),
	)
	githubClient := github.NewClient(serviceprovider.AuthenticatingHttpClient(mockedHTTPClient))
	err := aar.FetchAll(httptransport.WithBearerToken(context.TODO(), "access token"), githubClient, "access token", ts)
	//err := aar.FetchAll(context.TODO(),  github.NewClient(&http.Client{
	//	Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
	//		requestBody, err := ioutil.ReadAll(r.Body)
	//		if err != nil {
	//			panic(err)
	//		}
	//		if strings.Contains(string(requestBody), "ownerAffiliations") {
	//			return &http.Response{
	//				StatusCode: 200,
	//				Header:     http.Header{},
	//				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(repositoriesOwnerAffiliationsFakeResponse))),
	//				Request:    r,
	//			}, nil
	//		}
	//		return &http.Response{
	//			StatusCode: 200,
	//			Header:     http.Header{},
	//			Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(repositoriesAffiliationsFakeResponse))),
	//			Request:    r,
	//		}, nil
	//	}),
	//})), "access token", ts)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(ts.AccessibleRepos))
}

//
//func TestAllAccessibleRepos_fail(t *testing.T) {
//	aar := &AllAccessibleRepos{}
//
//	ts := &TokenState{
//		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
//	}
//
//	cl := serviceprovider.AuthenticatingHttpClient(&http.Client{
//		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
//			return &http.Response{
//				StatusCode: 401,
//				Header:     http.Header{},
//				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`{"message": "This endpoint requires authentication."}`))),
//				Request:    r,
//			}, nil
//		}),
//	})
//
//	err := aar.FetchAll(context.TODO(), graphql.NewClient("https://fake.github", graphql.WithHTTPClient(cl)), "access token", ts)
//
//	assert.Error(t, err)
//	assert.True(t, sperrors.IsInvalidAccessToken(err.(*url.Error).Err))
//}
