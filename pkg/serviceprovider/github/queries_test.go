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

	"github.com/machinebox/graphql"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/stretchr/testify/assert"
)

const fakeResponse = `
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
		Transport: serviceprovider.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(fakeResponse))),
				Request:    r,
			}, nil
		}),
	})), "access token", ts)

	assert.NoError(t, err)

	assert.Equal(t, 3, len(ts.AccessibleRepos))
}
