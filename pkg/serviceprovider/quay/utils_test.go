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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"

	"github.com/stretchr/testify/assert"
)

func TestDoQuayRequest(t *testing.T) {
	test := func(url, token, method, body, header string) {
		httpClient := http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == url {
					if token != "" {
						assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
					} else {
						assert.Empty(t, r.Header.Get("Authorization"))
					}
					assert.Equal(t, header, r.Header.Get("Content-Type"))
					assert.Equal(t, method, r.Method)
					return &http.Response{StatusCode: 200}, nil
				}
				assert.Fail(t, "unexpected request", "url", r.URL)
				return nil, nil
			}),
		}
		resp, err := doQuayRequest(context.TODO(), &httpClient, url, token, method, strings.NewReader(body), header)
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	}

	t.Run("empty header", func(t *testing.T) {
		test("http://test.com", "token", "GET", "body", "")
	})
	t.Run("application/json header", func(t *testing.T) {
		test("http://test.com/123", "token123", "POST", "body123", "application/json ")
	})
	t.Run("empty token", func(t *testing.T) {
		test("http://test.com", "", "Get", "body", "")
	})
}

func TestSplitQuayUrl(t *testing.T) {
	test := func(url, expectedOwner, expectedRepo, expectedVersion string) {
		t.Run(url, func(t *testing.T) {
			owner, repo, version := splitToOrganizationAndRepositoryAndVersion(url)
			assert.Equal(t, expectedOwner, owner)
			assert.Equal(t, expectedRepo, repo)
			assert.Equal(t, expectedVersion, version)
		})
	}

	test("https://quay.io/repository/redhat-appstudio/service-provider-integration-operator", "redhat-appstudio", "service-provider-integration-operator", "")
	test("quay.io/repository/redhat-appstudio/service-provider-integration-operator", "redhat-appstudio", "service-provider-integration-operator", "")
	test("https://quay.io/redhat-appstudio/service-provider-integration-operator", "redhat-appstudio", "service-provider-integration-operator", "")
	test("quay.io/redhat-appstudio/service-provider-integration-operator", "redhat-appstudio", "service-provider-integration-operator", "")
	test("github.com/redhat-appstudio/service-provider-integration-operator", "", "", "")
	test("https://github.com/redhat-appstudio/service-provider-integration-operator", "", "", "")
	test("quay.io/redhat-appstudio", "", "", "")
	test("https://quay.io/redhat-appstudio", "", "", "")
	test("quay.io", "", "", "")
	test("quay.io/", "", "", "")
	test("https://quay.io/repository/redhat-appstudio/service-provider-integration-operator:blabol", "redhat-appstudio", "service-provider-integration-operator", "blabol")
	test("quay.io/repository/redhat-appstudio/service-provider-integration-operator:blabol", "redhat-appstudio", "service-provider-integration-operator", "blabol")
	test("https://quay.io/redhat-appstudio/service-provider-integration-operator:blabol", "redhat-appstudio", "service-provider-integration-operator", "blabol")
	test("quay.io/redhat-appstudio/service-provider-integration-operator:blabol", "redhat-appstudio", "service-provider-integration-operator", "blabol")
}

func TestReadResponseBodyToJsonMap(t *testing.T) {
	test := func(name string, resp *http.Response, expected map[string]interface{}, expectedError bool) {
		t.Run(name, func(t *testing.T) {
			json, err := readResponseBodyToJsonMap(resp)
			assert.Equal(t, expected, json)
			if expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	test("no body", &http.Response{}, nil, true)
	test("not json body", &http.Response{Body: io.NopCloser(strings.NewReader("this is not a json"))}, nil, true)
	test("json body", &http.Response{Body: io.NopCloser(strings.NewReader("{\"json\": \"yes\"}"))}, map[string]interface{}{"json": "yes"}, false)
}
