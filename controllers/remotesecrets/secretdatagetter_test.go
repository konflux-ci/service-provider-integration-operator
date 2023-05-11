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

package remotesecrets

import (
	"context"
	"errors"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/remotesecretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretDataGetter_GetData(t *testing.T) {
	new := func() (*secretstorage.TestSecretStorage, remotesecretstorage.RemoteSecretStorage) {
		ss := &secretstorage.TestSecretStorage{}
		st := remotesecretstorage.NewJSONSerializingRemoteSecretStorage(ss)
		return ss, st
	}
	t.Run("not found error", func(t *testing.T) {
		ss, st := new()

		ss.GetImpl = func(ctx context.Context, key secretstorage.SecretID) ([]byte, error) {
			return nil, secretstorage.NotFoundError
		}

		sdg := SecretDataGetter{
			Storage: st,
		}

		data, reason, err := sdg.GetData(context.TODO(), &api.RemoteSecret{
			ObjectMeta: v1.ObjectMeta{
				Name: "kachny",
			},
		})
		assert.Empty(t, data)
		assert.Equal(t, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), reason)
		assert.Error(t, err)
		assert.ErrorIs(t, err, bindings.SecretDataNotFoundError)
	})

	t.Run("unkown error", func(t *testing.T) {
		ss, st := new()

		ss.GetImpl = func(ctx context.Context, key secretstorage.SecretID) ([]byte, error) {
			return nil, errors.New("teh error")
		}

		sdg := SecretDataGetter{
			Storage: st,
		}

		data, reason, err := sdg.GetData(context.TODO(), &api.RemoteSecret{
			ObjectMeta: v1.ObjectMeta{
				Name: "kachny",
			},
		})
		assert.Empty(t, data)
		assert.Equal(t, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), reason)
		assert.Error(t, err)
		assert.False(t, errors.Is(err, bindings.SecretDataNotFoundError))
	})

	t.Run("get the data", func(t *testing.T) {
		ss, st := new()

		ss.GetImpl = func(ctx context.Context, key secretstorage.SecretID) ([]byte, error) {
			return []byte("{\"a\": \"Yg==\"}"), nil
		}

		sdg := SecretDataGetter{
			Storage: st,
		}

		data, reason, err := sdg.GetData(context.TODO(), &api.RemoteSecret{
			ObjectMeta: v1.ObjectMeta{
				Name: "kachny",
			},
		})
		assert.NotEmpty(t, data)
		assert.Equal(t, []byte("b"), data["a"])
		assert.Empty(t, reason)
		assert.NoError(t, err)
	})
}
