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
	"fmt"
	"io"
	"net/http"
	"testing"

	"golang.org/x/oauth2"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func mockRefreshTokenCapability(returnCode int, body string, responseError error) *refreshTokenCapability {
	httpClientMock := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: returnCode,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBuffer([]byte(body))),
				Request:    r,
			}, responseError
		}),
	}

	return &refreshTokenCapability{
		httpClient:          httpClientMock,
		gitlabBaseUrl:       "",
		oauthServiceBaseUrl: "",
	}
}

func TestRefreshToken(t *testing.T) {
	refreshCapability := mockRefreshTokenCapability(http.StatusOK, `{"access_token": "42"}`, nil)

	token, err := refreshCapability.RefreshToken(context.TODO(), &api.Token{}, &oauth2.Config{ClientID: "hello", ClientSecret: "world"})

	assert.NoError(t, err)
	assert.Equal(t, "42", token.AccessToken)
}

func TestRefreshTokenError(t *testing.T) {
	test := func(refreshCapabilityInstance refreshTokenCapability, expectedErrorSubstring string) {
		t.Run(fmt.Sprintf("should return error containing: %s", expectedErrorSubstring), func(t *testing.T) {
			_, err := refreshCapabilityInstance.RefreshToken(context.TODO(), &api.Token{}, &oauth2.Config{ClientID: "hello", ClientSecret: "world"})
			assert.ErrorContains(t, err, expectedErrorSubstring)
		})
	}

	test(*mockRefreshTokenCapability(http.StatusOK, `{"access_token": 42}`, nil), "failed to unmarshal")
	test(*mockRefreshTokenCapability(http.StatusUnauthorized, "", nil), "non-ok status")
	test(*mockRefreshTokenCapability(http.StatusOK, "", errors.New("request error")), "failed to request")
}
