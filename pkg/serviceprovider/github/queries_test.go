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
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/mshaposhnik/service-provider-integration-operator/pkg/spi-shared/util"

	sperrors "github.com/mshaposhnik/service-provider-integration-operator/pkg/errors"

	"github.com/machinebox/graphql"
	"github.com/mshaposhnik/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/stretchr/testify/assert"
)

const allRepositoriesFakeResponse = `
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
            "url": "https://github.com/metlos/RHQ-old"
          },
          {
            "viewerPermission": "WRITE",
            "url": "https://github.com/openshiftio/openshift.io"
          },
          {
            "viewerPermission": "READ",
            "url": "https://github.com/openshiftio/booster-parent"
          }
        ]
      }
    }
  }
}
`

func TestAllAccessibleRepos(t *testing.T) {
	aar := &AllAccessibleRepos{}

	ts := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	err := aar.FetchAll(context.TODO(), graphql.NewClient("https://fake.github", graphql.WithHTTPClient(&http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(allRepositoriesFakeResponse))),
				Request:    r,
			}, nil
		}),
	})), "access token", ts)

	assert.NoError(t, err)

	assert.Equal(t, 3, len(ts.AccessibleRepos))
}

func TestAllAccessibleRepos_fail(t *testing.T) {
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

	err := aar.FetchAll(context.TODO(), graphql.NewClient("https://fake.github", graphql.WithHTTPClient(cl)), "access token", ts)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, sperrors.InvalidAccessToken))
}
