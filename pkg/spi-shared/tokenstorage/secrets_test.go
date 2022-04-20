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

package tokenstorage

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()

	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(api.AddToScheme(scheme))
}

func TestSecretsTokenStorage_Delete(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "default",
		},
		Spec: api.SPIAccessTokenSpec{
			Permissions:        api.Permissions{},
			ServiceProviderUrl: "https://sp",
		},
	}

	storage := newStorage(token, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spi-storage-token",
			Namespace: "default",
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	})

	assert.NoError(t, storage.Delete(context.TODO(), token))

	err := storage.Client.Get(context.TODO(), client.ObjectKey{Name: "secret", Namespace: "default"}, &corev1.Secret{})
	assert.True(t, errors.IsNotFound(err))
}

func TestSecretsTokenStorage_Get(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "default",
		},
		Spec: api.SPIAccessTokenSpec{
			Permissions:        api.Permissions{},
			ServiceProviderUrl: "https://sp",
		},
	}

	t.Run("with expiry", func(t *testing.T) {
		storage := newStorage(token, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "spi-storage-token",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"access_token":  []byte("access"),
				"refresh_token": []byte("refresh"),
				"token_type":    []byte("awesome"),
				"expiry":        []byte("15"),
			},
			Type: "Opaque",
		})

		data, err := storage.Get(context.TODO(), token)
		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Equal(t, "access", data.AccessToken)
		assert.Equal(t, "refresh", data.RefreshToken)
		assert.Equal(t, "awesome", data.TokenType)
		assert.Equal(t, uint64(15), data.Expiry)
	})

	t.Run("without expiry", func(t *testing.T) {
		storage := newStorage(token, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "spi-storage-token",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"access_token":  []byte("access"),
				"refresh_token": []byte("refresh"),
				"token_type":    []byte("awesome"),
			},
			Type: "Opaque",
		})

		data, err := storage.Get(context.TODO(), token)
		assert.NoError(t, err)
		assert.Equal(t, "access", data.AccessToken)
		assert.Equal(t, "refresh", data.RefreshToken)
		assert.Equal(t, "awesome", data.TokenType)
		assert.Equal(t, uint64(0), data.Expiry)
	})
}

func TestSecretsTokenStorage_Store(t *testing.T) {
	tokenSpec := api.SPIAccessTokenSpec{
		Permissions:        api.Permissions{},
		ServiceProviderUrl: "https://sp",
	}

	data := &api.Token{
		AccessToken:  "access",
		TokenType:    "happy",
		RefreshToken: "refresh",
		Expiry:       42,
	}

	testSecret := func(t *testing.T, storage *secretsTokenStorage, token *api.SPIAccessToken) {
		key := getSecretKey(token)

		secret := &corev1.Secret{}
		assert.NoError(t, storage.Client.Get(context.TODO(), key, secret))
		assert.Equal(t, "access", string(secret.Data["access_token"]))
		assert.Equal(t, "happy", string(secret.Data["token_type"]))
		assert.Equal(t, "refresh", string(secret.Data["refresh_token"]))
		assert.Equal(t, "42", string(secret.Data["expiry"]))
		assert.Equal(t, 1, len(secret.OwnerReferences))
		assert.Equal(t, "42", string(secret.OwnerReferences[0].UID))
	}

	t.Run("token with name", func(t *testing.T) {
		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
				UID:       "42",
			},
			Spec: tokenSpec,
		}

		storage := newStorage(token)

		err := storage.Store(context.TODO(), token, data)
		assert.NoError(t, err)
		testSecret(t, storage, token)
	})

	t.Run("with pre-existing secret", func(t *testing.T) {
		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
				UID:       "42",
			},
			Spec: tokenSpec,
		}

		preexistingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
				UID:       "56",
			},
			Data: map[string][]byte{
				"my": []byte("data"),
			},
			Type: "Opaque",
		}

		storage := newStorage(token, preexistingSecret)

		err := storage.Store(context.TODO(), token, data)
		assert.NoError(t, err)
		testSecret(t, storage, token)
	})
}

func newStorage(objs ...client.Object) *secretsTokenStorage {
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	return &secretsTokenStorage{
		Client: cl,
		syncer: sync.New(cl),
	}
}
