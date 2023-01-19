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

package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBaseUrl(t *testing.T) {
	assert.Equal(t, "ht://bla.bol", GetBaseUrl(&url.URL{Scheme: "ht", Host: "bla.bol"}))
	assert.Equal(t, "ht://bla.bol", GetBaseUrl(&url.URL{Scheme: "ht", Host: "bla.bol", Path: "/hello/there"}))
	assert.Equal(t, "", GetBaseUrl(&url.URL{Host: "blabol.sh"}))
	assert.Equal(t, "", GetBaseUrl(&url.URL{Scheme: "bla"}))
	assert.Equal(t, "", GetBaseUrl(nil))
}

func TestGetHostWithScheme(t *testing.T) {
	test := func(name string, inputUrl string, shouldError bool, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			u, err := GetHostWithScheme(inputUrl)
			if shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, expectedOutput, u)
		})
	}

	test("ok url with path", "https://blabol.com/hello/world", false, "https://blabol.com")
	test("ok url without path", "https://blabol.com", false, "https://blabol.com")
	test("url without scheme", "blab.ol", false, "")
	test("scheme without host", "https://", false, "")
	test("invalid url", ":::", true, "")
}
