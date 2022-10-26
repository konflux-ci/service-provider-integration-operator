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

package gitlab

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

var tokenStorageGet = func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
	return &api.Token{
		AccessToken:  "token",
		TokenType:    "Grizzly Bear-er",
		RefreshToken: "refresh-token",
		Expiry:       0,
	}, nil
}

func TestFetch(t *testing.T) {
	httpClient := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {

			if strings.Contains(r.URL.String(), "/user") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "user42"}`))),
				}, nil
			}
			if strings.Contains(r.URL.String(), "/token/info") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"scope": ["write_repository", "read_registry"]}`))),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
			}, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}

	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpClient,
			tokenStorage: ts,
		},
		httpClient:   httpClient,
		tokenStorage: &ts,
	}

	token := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &token)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "42", data.UserId)
	assert.Equal(t, "user42", data.Username)
	assert.Equal(t, []string{"write_repository", "read_registry"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)
}

func TestFetch_Fail(t *testing.T) {
	expectedError := errors.New("math: square root of negative number")
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/user") {
				return nil, expectedError
			} else {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "user42"}`))),
				}, nil
			}
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}

	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpCl,
			tokenStorage: ts,
		},
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.Error(t, err, expectedError)
	assert.Nil(t, data)
}
