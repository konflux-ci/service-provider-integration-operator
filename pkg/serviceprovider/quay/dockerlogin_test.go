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

package quay

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestDockerLogin(t *testing.T) {
	cl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			assert.Contains(t, r.Header, "Authorization")
			// the value is base64 encoded "alois:password"
			if r.Header.Get("Authorization") == "Basic YWxvaXM6cGFzc3dvcmQ=" {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(`{"token": "token"}`)),
				}, nil
			} else {
				return &http.Response{
					StatusCode: 403,
				}, nil
			}
		}),
	}

	t.Run("successful login", func(t *testing.T) {
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "password")
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})

	t.Run("unsuccessful login", func(t *testing.T) {
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "passwort")
		assert.Empty(t, token)
		assert.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "login did not succeed"))
	})
}

func TestAnalyzeLoginToken(t *testing.T) {
	// This would just test that we can parse a JWT token using a library.
}
