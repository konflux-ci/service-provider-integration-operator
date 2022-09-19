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

package oauthstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnonymous(t *testing.T) {
	encoded, err := Encode(&OAuthInfo{
		TokenName:      "token-name",
		TokenNamespace: "default",
		Scopes:         []string{"a", "b", "c"},
	})
	assert.NoError(t, err)

	decoded, err := ParseOAuthInfo(encoded)
	assert.NoError(t, err)

	assert.Equal(t, "token-name", decoded.TokenName)
	assert.Equal(t, "default", decoded.TokenNamespace)
	assert.Equal(t, []string{"a", "b", "c"}, decoded.Scopes)
}

func TestCustom(t *testing.T) {
	type Custom struct {
		Data string
	}

	type Invalid struct {
		NotData string
	}

	t.Run("valid", func(t *testing.T) {
		encoded, err := Encode(&Custom{
			Data: "42",
		})
		assert.NoError(t, err)

		decoded := &Custom{}
		err = ParseInto(encoded, decoded)
		assert.NoError(t, err)

		assert.Equal(t, "42", decoded.Data)
	})

	t.Run("wrong type", func(t *testing.T) {
		encoded, err := Encode(&Invalid{})
		assert.NoError(t, err)

		decoded := &Custom{}
		err = ParseInto(encoded, decoded)
		assert.NoError(t, err)
		assert.Empty(t, decoded.Data)
	})
}
