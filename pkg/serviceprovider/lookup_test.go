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

func TestGenericLookup_Lookup(t *testing.T) {
	matchingToken := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken1 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching1",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken2 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching2",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}

	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(matchingToken, nonMatchingToken1, nonMatchingToken2).Build()

	cache := MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
		CacheServiceProviderState: true,
	}
	gl := GenericLookup{
		ServiceProviderType: "test",
		TokenFilter: TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return token.Name == "matching", nil
		}),
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}),
		MetadataCache:  &cache,
		RepoHostParser: RepoHostFromUrl,
	}

	tkns, err := gl.Lookup(context.TODO(), cl, &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "https://fake.sp",
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(tkns))
	assert.Equal(t, "matching", tkns[0].Name)
}

func TestGenericLookup_PersistMetadata(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{},
	}

	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(token).Build()

	cache := MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
		CacheServiceProviderState: true,
	}

	fetchCalled := false
	gl := GenericLookup{
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			fetchCalled = true
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}),
		MetadataCache:  &cache,
		RepoHostParser: RepoHostFromUrl,
	}

	assert.NoError(t, gl.PersistMetadata(context.TODO(), token))
	assert.True(t, fetchCalled)

	assert.NoError(t, cl.Get(context.TODO(), client.ObjectKeyFromObject(token), token))
	assert.NotNil(t, token.Status.TokenMetadata)
	assert.Equal(t, "42", token.Status.TokenMetadata.UserId)
	assert.True(t, token.Status.TokenMetadata.LastRefreshTime > 0)
}
