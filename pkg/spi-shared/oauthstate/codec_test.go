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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
)

func TestAnonymous(t *testing.T) {
	codec := getCodec(t)

	t.Run("valid", func(t *testing.T) {
		encoded, err := codec.Encode(&AnonymousOAuthState{
			TokenName:           "token-name",
			TokenNamespace:      "default",
			IssuedAt:            0,
			Scopes:              []string{"a", "b", "c"},
			ServiceProviderType: "sp type",
			ServiceProviderUrl:  "https://sp",
		})
		assert.NoError(t, err)

		decoded, err := codec.ParseAnonymous(encoded)
		assert.NoError(t, err)

		assert.Equal(t, "token-name", decoded.TokenName)
		assert.Equal(t, "default", decoded.TokenNamespace)
		assert.Equal(t, int64(0), decoded.IssuedAt)
		assert.Equal(t, []string{"a", "b", "c"}, decoded.Scopes)
		assert.Equal(t, config.ServiceProviderType("sp type"), decoded.ServiceProviderType)
		assert.Equal(t, "https://sp", decoded.ServiceProviderUrl)
	})

	t.Run("invalid", func(t *testing.T) {
		encoded, err := codec.Encode(&AnonymousOAuthState{
			TokenName:           "token-name",
			TokenNamespace:      "default",
			IssuedAt:            time.Now().Add(1 * time.Hour).Unix(),
			Scopes:              nil,
			ServiceProviderType: "sp type",
			ServiceProviderUrl:  "https://sp",
		})
		assert.NoError(t, err)

		_, err = codec.ParseAnonymous(encoded)
		assert.Error(t, err)
	})
}

func TestCustom(t *testing.T) {
	codec := getCodec(t)

	type Custom struct {
		Data string
	}

	type Invalid struct {
		NotData string
	}

	t.Run("valid", func(t *testing.T) {
		encoded, err := codec.Encode(&Custom{
			Data: "42",
		})
		assert.NoError(t, err)

		decoded := &Custom{}
		err = codec.ParseInto(encoded, decoded)
		assert.NoError(t, err)

		assert.Equal(t, "42", decoded.Data)
	})

	t.Run("wrong type", func(t *testing.T) {
		encoded, err := codec.Encode(&Invalid{})
		assert.NoError(t, err)

		decoded := &Custom{}
		err = codec.ParseInto(encoded, decoded)
		assert.NoError(t, err)
		assert.Empty(t, decoded.Data)
	})
}

func getCodec(t *testing.T) Codec {
	ret, err := NewCodec([]byte("secret"))
	assert.NoError(t, err)
	return ret
}
