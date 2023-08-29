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

package serviceprovider

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSPIAccessTokenLookup(t *testing.T) {
	t.Run("token found", func(t *testing.T) {
		lookup := GenericLookup{
			SPICredentialsSource: TestSPISource{},
		}
		tokens, err := lookup.SPIAccessTokenLookup(context.TODO(), nil, &api.SPIAccessCheck{})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(tokens))
	})

	t.Run("token not found", func(t *testing.T) {
		lookup := GenericLookup{
			SPICredentialsSource: TestSPISource{LookupCredentialsSourceImpl: func(ctx context.Context, c client.Client, matchable Matchable) (*api.SPIAccessToken, error) {
				return nil, nil
			}},
		}
		tokens, err := lookup.SPIAccessTokenLookup(context.TODO(), nil, &api.SPIAccessCheck{})
		assert.NoError(t, err)
		assert.Empty(t, tokens)
	})

}

func TestCredentialsLookup(t *testing.T) {
	t.Run("SPIAccessToken found", func(t *testing.T) {
		lookup := GenericLookup{
			SPICredentialsSource: TestSPISource{},
		}
		cred, err := lookup.CredentialsLookup(context.TODO(), nil, &api.SPIAccessCheck{})
		assert.NoError(t, err)
		assert.Equal(t, "spitoken", cred.SourceObjectName)
	})

	t.Run("SPIAccessToken not found, RS found", func(t *testing.T) {
		lookup := GenericLookup{
			SPICredentialsSource: TestSPISource{LookupCredentialsImpl: func(ctx context.Context, c client.Client, matchable Matchable) (*Credentials, error) {
				return nil, nil
			}},
			RemoteSecretCredentialsSource: TestRSSource{},
		}
		cred, err := lookup.CredentialsLookup(context.TODO(), nil, &api.SPIAccessCheck{})
		assert.NoError(t, err)
		assert.Equal(t, "remotesecret", cred.SourceObjectName)
	})
	t.Run("SPIAccessToken not found, RS not found", func(t *testing.T) {
		lookup := GenericLookup{
			SPICredentialsSource: TestSPISource{LookupCredentialsImpl: func(ctx context.Context, c client.Client, matchable Matchable) (*Credentials, error) {
				return nil, nil
			}},
			RemoteSecretCredentialsSource: TestRSSource{LookupCredentialsImpl: func(ctx context.Context, c client.Client, matchable Matchable) (*Credentials, error) {
				return nil, nil
			}},
		}
		cred, err := lookup.CredentialsLookup(context.TODO(), nil, &api.SPIAccessCheck{})
		assert.NoError(t, err)
		assert.Nil(t, cred)
	})
}
