package github

import (
	"bytes"
	"context"
	"github.com/machinebox/graphql"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
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

	ts, err := aar.FetchAll(context.TODO(), graphql.NewClient("https://fake.github", graphql.WithHTTPClient(&http.Client{
		Transport: serviceprovider.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(fakeResponse))),
				Request:    r,
			}, nil
		}),
	})), "access token")

	assert.NoError(t, err)

	assert.Equal(t, 3, len(ts))
}
