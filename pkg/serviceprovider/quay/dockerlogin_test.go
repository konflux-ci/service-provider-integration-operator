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
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestDockerLogin(t *testing.T) {
	var throwError bool
	var responseBody string

	setup := func() {
		throwError = false
		responseBody = `{"token": "token"}`
	}

	cl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if throwError {
				return nil, errors.New("intentional HTTP error")
			}

			assert.Contains(t, r.Header, "Authorization")
			// the value is base64 encoded "alois:password"
			if r.Header.Get("Authorization") == "Basic YWxvaXM6cGFzc3dvcmQ=" {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(responseBody)),
				}, nil
			} else {
				return &http.Response{
					StatusCode: 403,
				}, nil
			}
		}),
	}

	t.Run("successful login", func(t *testing.T) {
		setup()
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "password")
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})

	t.Run("unsuccessful login", func(t *testing.T) {
		setup()
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "passwort")
		assert.NoError(t, err)
		assert.Empty(t, token)
	})

	t.Run("error during login", func(t *testing.T) {
		setup()
		throwError = true
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "password")
		assert.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("invalid response from quay as error", func(t *testing.T) {
		setup()
		responseBody = "invalid response body"
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "password")
		assert.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestAnalyzeLoginToken(t *testing.T) {
	// a fake token with the payload in the same format as quay uses
	loginToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJxdWF5IiwiYXVkIjoicXVheS5pbyIsIm5iZiI6MTY1MjEwNTQ1OCwiaWF0IjoxNjUyMTA1NDU4LCJleHAiOjE2NTIxMDkwNTgsInN1YiI6InRlc3QrdGVzdCIsImFjY2VzcyI6W3sidHlwZSI6InJlcG9zaXRvcnkiLCJuYW1lIjoidGVzdG9yZy9yZXBvIiwiYWN0aW9ucyI6WyJwdXNoIiwicHVsbCJdfV0sImNvbnRleHQiOnsidmVyc2lvbiI6MiwiZW50aXR5X2tpbmQiOiJyb2JvdCIsImVudGl0eV9yZWZlcmVuY2UiOiJ0ZXN0K3Rlc3QiLCJraW5kIjoidXNlciIsInVzZXIiOiJ0ZXN0K3Rlc3QiLCJjb20uYXBvc3RpbGxlLnJvb3RzIjp7InVuaG9vay91bmhvb2stdHVubmVsIjoiJGRpc2FibGVkIn0sImNvbS5hcG9zdGlsbGUucm9vdCI6IiRkaXNhYmxlZCJ9fQ.W5juwSf4l2YQM7SGiuUpok5ZJBEF1hry01cQ5rObrP8"

	info, err := AnalyzeLoginToken(loginToken)
	assert.NoError(t, err)

	assert.Equal(t, "test+test", info.Username)

	assert.Contains(t, info.Repositories, "testorg/repo")
	assert.True(t, info.Repositories["testorg/repo"].Pullable)
	assert.True(t, info.Repositories["testorg/repo"].Pushable)
}
