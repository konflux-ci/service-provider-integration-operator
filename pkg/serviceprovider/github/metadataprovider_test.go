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
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/machinebox/graphql"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

func TestMetadataProvider_Fetch(t *testing.T) {
	httpCl := &http.Client{
		Transport: serviceprovider.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL == githubUserApiEndpoint {
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
					Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(allRepositoriesFakeResponse))),
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

	mp := metadataProvider{
		graphqlClient: graphql.NewClient("", graphql.WithHTTPClient(httpCl)),
		httpClient:    httpCl,
		tokenStorage:  &ts,
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
}
