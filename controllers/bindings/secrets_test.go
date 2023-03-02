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

package bindings

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/memorystorage"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSync(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	clBld := fake.NewClientBuilder().WithScheme(scheme)

	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "default",
		},
	}

	binding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
		Spec: api.SPIAccessTokenBindingSpec{
			Secret: api.SecretSpec{},
		},
	}

	h := secretHandler{
		Binding: binding,
		// we initialize a new empty client in each subtest so that they remain independent
		Client:       nil,
		TokenStorage: &memorystorage.MemoryTokenStorage{},
	}

	assert.NoError(t, h.TokenStorage.Store(context.TODO(), token, &api.Token{
		Username:    "alois",
		AccessToken: "token",
	}))

	sp := serviceprovider.TestServiceProvider{}
	sp.MapTokenImpl = func(ctx context.Context, stb *api.SPIAccessTokenBinding, st *api.SPIAccessToken, t *api.Token) (serviceprovider.AccessTokenMapper, error) {
		return serviceprovider.DefaultMapToken(st, t), nil
	}

	t.Run("empty-cluster", func(t *testing.T) {
		t.Run("service-account-token secret type", func(t *testing.T) {
			binding.Spec.Secret.Type = corev1.SecretTypeServiceAccountToken
			h.Client = clBld.Build()

			secret, reason, err := h.Sync(context.TODO(), token, &sp)
			assert.Equal(t, api.SPIAccessTokenBindingErrorReasonNoError, reason)
			assert.NoError(t, err)

			assert.NotNil(t, secret)
			assert.Contains(t, secret.Data, "extra")
			assert.Equal(t, secret.Data["extra"], []byte("token"))
		})

		t.Run("other secret types", func(t *testing.T) {
			binding.Spec.Secret.Type = corev1.SecretTypeOpaque
			h.Client = clBld.Build()

			secret, reason, err := h.Sync(context.TODO(), token, &sp)
			assert.Equal(t, api.SPIAccessTokenBindingErrorReasonNoError, reason)
			assert.NoError(t, err)

			assert.NotNil(t, secret)
			assert.Contains(t, secret.Data, "token")
			assert.Equal(t, secret.Data["token"], []byte("token"))
		})
	})

	t.Run("secret-in-cluster", func(t *testing.T) {
		t.Run("service-account-token secret type", func(t *testing.T) {
			binding.Spec.Secret.Type = corev1.SecretTypeServiceAccountToken
		})

		t.Run("other secret types", func(t *testing.T) {
			binding.Spec.Secret.Type = corev1.SecretTypeOpaque
		})
	})
}

func TestList(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "not-our-binding",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "b",
					Namespace: "default",
					Labels: map[string]string{
						"not-labeled": "correctly",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "c",
					Namespace: "different-one",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shes-the-one",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "binding",
					},
				},
			},
		).
		Build()

	binding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
		Spec: api.SPIAccessTokenBindingSpec{
			Secret: api.SecretSpec{},
		},
	}

	h := secretHandler{
		Binding:      binding,
		Client:       cl,
		TokenStorage: &memorystorage.MemoryTokenStorage{},
	}

	scs, err := h.List(context.TODO())
	assert.NoError(t, err)

	assert.Len(t, scs, 1)
	assert.Equal(t, scs[0].Name, "shes-the-one")
}
