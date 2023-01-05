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
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMetadataCache_Refresh(t *testing.T) {
	t.Run("ignores nil metadata", func(t *testing.T) {
		tkn := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{},
		}

		mc := MetadataCache{
			Client:                    nil,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Second},
			CacheServiceProviderState: true,
		}
		mc.refresh(tkn)

		assert.Nil(t, tkn.Status.TokenMetadata)
	})

	t.Run("leaves valid metadata in place", func(t *testing.T) {
		lastRefresh := time.Now()

		tkn := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					LastRefreshTime: lastRefresh.Unix(),
				},
			},
		}

		mc := MetadataCache{
			Client:                    nil,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		mc.refresh(tkn)
		// assert that we ran sufficiently quickly for the test to have a chance to pass
		assert.True(t, time.Now().Before(lastRefresh.Add(1*time.Hour)), "the test ran too slow")

		assert.NotNil(t, tkn.Status.TokenMetadata)
		assert.Equal(t, lastRefresh.Unix(), tkn.Status.TokenMetadata.LastRefreshTime)
	})

	t.Run("clears stale metadata", func(t *testing.T) {
		lastRefresh := time.Now().Add(-5 * time.Hour).Unix()

		tkn := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					LastRefreshTime: lastRefresh,
				},
			},
		}

		mc := MetadataCache{
			Client:                    nil,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		mc.refresh(tkn)

		assert.Nil(t, tkn.Status.TokenMetadata)
	})
}

func TestMetadataCache_Persist(t *testing.T) {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{},
	}).Build()

	t.Run("works with nil metadata", func(t *testing.T) {
		token := &api.SPIAccessToken{}
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "test-token", Namespace: "default"}, token))

		mc := MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		assert.NoError(t, mc.Persist(context.TODO(), token))
		assert.Nil(t, token.Status.TokenMetadata)
	})

	t.Run("sets last refresh time", func(t *testing.T) {
		lastRefresh := time.Now().Add(-5 * time.Hour).Unix()
		token := &api.SPIAccessToken{}
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "test-token", Namespace: "default"}, token))
		token.Status.TokenMetadata = &api.TokenMetadata{
			LastRefreshTime: lastRefresh,
		}

		mc := MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		assert.NoError(t, mc.Persist(context.TODO(), token))

		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "test-token", Namespace: "default"}, token))
		assert.NotNil(t, token.Status.TokenMetadata)
		assert.True(t, token.Status.TokenMetadata.LastRefreshTime > lastRefresh)
	})
}

func TestMetadataCache_Ensure(t *testing.T) {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	cl := fake.NewClientBuilder().WithScheme(sch).Build()

	t.Run("no re-fetch when valid", func(t *testing.T) {
		lastRefresh := time.Now()
		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token-valid",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					LastRefreshTime: lastRefresh.Unix(),
				},
			},
		}
		assert.NoError(t, cl.Create(context.TODO(), token))

		mc := MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		err := mc.Ensure(context.TODO(), token, MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}))
		assert.NoError(t, err)
		// assert that we ran sufficiently quickly for the test to have a chance to pass
		assert.True(t, time.Now().Before(lastRefresh.Add(1*time.Hour)), "the test ran too slow")

		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "test-token-valid", Namespace: "default"}, token))
		assert.NotNil(t, token.Status.TokenMetadata)
		assert.Empty(t, token.Status.TokenMetadata.UserId) // no fetch happened, otherwise we'd have 42 here
		assert.Equal(t, lastRefresh.Unix(), token.Status.TokenMetadata.LastRefreshTime)
	})

	t.Run("refresh when ttl elapsed", func(t *testing.T) {
		lastRefresh := time.Now().Add(-5 * time.Hour)
		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token-stale",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					LastRefreshTime: lastRefresh.Unix(),
				},
			},
		}
		assert.NoError(t, cl.Create(context.TODO(), token))

		mc := MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		err := mc.Ensure(context.TODO(), token, MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}))
		assert.NoError(t, err)

		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "test-token-stale", Namespace: "default"}, token))
		assert.NotNil(t, token.Status.TokenMetadata)
		assert.Equal(t, "42", token.Status.TokenMetadata.UserId)
		assert.True(t, lastRefresh.Unix() < token.Status.TokenMetadata.LastRefreshTime)
	})

	t.Run("clears metadata when re-fetch yields nothing", func(t *testing.T) {
		// This happens when there is no metadata or the metadata is stale and the attempt to fetch fresh metadata
		// returns no data (but does not fail). I.e. when the token data disappears for some reason
		lastRefresh := time.Now().Add(-5 * time.Hour)
		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token-clear-metadata-",
				Namespace:    "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					LastRefreshTime: lastRefresh.Unix(),
				},
			},
		}
		assert.NoError(t, cl.Create(context.TODO(), token))

		mc := MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		}
		err := mc.Ensure(context.TODO(), token, MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return nil, nil
		}))
		assert.NoError(t, err)

		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKeyFromObject(token), token))
		assert.Nil(t, token.Status.TokenMetadata)
	})
}
