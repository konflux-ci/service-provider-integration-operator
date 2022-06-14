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

package hostcredentials

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

var ts = tokenstorage.TestTokenStorage{
	GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{
			Username:     "test_user",
			AccessToken:  "access",
			TokenType:    "fake",
			RefreshToken: "refresh",
			Expiry:       0,
		}, nil
	},
}

func TestMetadataProvider_FetchUserIdAndName(t *testing.T) {

	mp := metadataProvider{
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "test_user", data.Username)
}
