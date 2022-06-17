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
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

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
		mock.WithRequestMatchPages(
			mock.GetUserRepos,
			[]github.Repository{
				{
					Name:    github.String("RHQ-old"),
					HTMLURL: github.String("https://github.com/jdoe/RHQ-old"),
					Permissions: map[string]bool{
						"admin": true,
					},
				},
				{
					Name:    github.String("openshift.io"),
					HTMLURL: github.String("https://github.com/openshiftio/openshift.io"),
					Permissions: map[string]bool{
						"push": true,
					},
				},
				{
					Name:    github.String("booster-parent"),
					HTMLURL: github.String("https://github.com/openshiftio/booster-parent"),
					Permissions: map[string]bool{
						"pull": true,
					},
				},
			},
			[]github.Repository{
				{
					Name:    github.String("opencompose-old"),
					HTMLURL: github.String("https://github.com/redhat-developer/opencompose-old"),
					Permissions: map[string]bool{
						"pull": true,
					},
				},
				{
					Name:    github.String("far2go"),
					HTMLURL: github.String("https://github.com/jdoe/far2go"),
					Permissions: map[string]bool{
						"admin": true,
					},
				},
			},
		),
	)
	githubClient := github.NewClient(mockedHTTPClient)
	err := aar.FetchAll(httptransport.WithBearerToken(context.TODO(), "access token"), githubClient, "access token", ts)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(ts.AccessibleRepos))
}

func TestAllAccessibleRepos_failFetchAll(t *testing.T) {
	aar := &AllAccessibleRepos{}

	ts := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}

	cl := serviceprovider.AuthenticatingHttpClient(&http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 401,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`{"message": "This endpoint requires authentication."}`))),
				Request:    r,
			}, nil
		}),
	})

	githubClient := github.NewClient(cl)
	err := aar.FetchAll(httptransport.WithBearerToken(context.TODO(), "access token"), githubClient, "access token", ts)

	assert.Error(t, err)
	assert.True(t, sperrors.IsServiceProviderHttpInvalidAccessToken(err))
}
