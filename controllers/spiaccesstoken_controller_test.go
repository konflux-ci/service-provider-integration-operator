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

package controllers

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/memorystorage"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureLabels(t *testing.T) {
	t.Run("sets the predefined", func(t *testing.T) {
		at := api.SPIAccessToken{
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, ensureLabels(&at, "sp_type"))
		assert.Equal(t, "sp_type", at.Labels[api.ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[api.ServiceProviderHostLabel])
	})

	t.Run("doesn't overwrite existing", func(t *testing.T) {
		at := api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"a":                          "av",
					"b":                          "bv",
					api.ServiceProviderHostLabel: "orig-host",
				},
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, ensureLabels(&at, "sp_type"))
		assert.Equal(t, "sp_type", at.Labels[api.ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[api.ServiceProviderHostLabel])
		assert.Equal(t, "av", at.Labels["a"])
		assert.Equal(t, "bv", at.Labels["b"])
	})
}

func TestReconcileData(t *testing.T) {
	t.Run("no data found then nil the metadata", func(t *testing.T) {
		at := &api.SPIAccessToken{Status: api.SPIAccessTokenStatus{Phase: api.SPIAccessTokenPhaseReady, TokenMetadata: &api.TokenMetadata{}}}
		reconciler := SPIAccessTokenReconciler{
			TokenStorage: &memorystorage.MemoryTokenStorage{},
		}

		err := reconciler.reconcileTokenData(context.TODO(), at)

		assert.NoError(t, err)
		assert.Nil(t, at.Status.TokenMetadata)
	})

	t.Run("data found, don't touch the SPIAccessToken", func(t *testing.T) {
		at := &api.SPIAccessToken{Status: api.SPIAccessTokenStatus{Phase: api.SPIAccessTokenPhaseReady, TokenMetadata: &api.TokenMetadata{}}}
		ts := &memorystorage.MemoryTokenStorage{}
		ts.Store(context.TODO(), at, &api.Token{Username: "blabol", AccessToken: "kockopes"})
		reconciler := SPIAccessTokenReconciler{
			TokenStorage: ts,
		}

		err := reconciler.reconcileTokenData(context.TODO(), at)

		assert.NoError(t, err)
		assert.NotNil(t, at.Status.TokenMetadata)
	})
}
