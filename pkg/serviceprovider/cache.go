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
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MetadataCache acts like a cache of metadata of tokens.
// On top of just CRUDing the token metadata, this struct handles the refreshes of the data when it is determined
// stale.
type MetadataCache struct {
	// Ttl limits how long the data stays in the cache
	Ttl time.Duration

	client client.Client
}

// NewMetadataCache creates a new cache instance with the provided configuration.
func NewMetadataCache(ttl time.Duration, client client.Client) MetadataCache {
	return MetadataCache{
		Ttl:    ttl,
		client: client,
	}
}

// Persist assigns the last refresh time of the token metadata and updates the token
func (c *MetadataCache) Persist(ctx context.Context, token *api.SPIAccessToken) error {
	if token.Status.TokenMetadata != nil {
		token.Status.TokenMetadata.LastRefreshTime = time.Now().Unix()
	}

	return c.client.Status().Update(ctx, token)
}

// Refresh checks if the token's metadata is still valid. If it is stale, the metadata is cleared
func (c *MetadataCache) Refresh(token *api.SPIAccessToken) {
	metadata := token.Status.TokenMetadata
	if metadata == nil {
		return
	}

	if time.Now().After(time.Unix(metadata.LastRefreshTime, 0).Add(c.Ttl)) {
		token.Status.TokenMetadata = nil
	}
}

// Ensure combines Refresh and Persist. Makes sure that the metadata of the token is either still valid or has been
// refreshed using the MetadataProvider.
func (c *MetadataCache) Ensure(ctx context.Context, token *api.SPIAccessToken, ser MetadataProvider) error {
	wasPresent := token.Status.TokenMetadata != nil

	c.Refresh(token)

	if token.Status.TokenMetadata == nil {
		data, err := ser.Fetch(ctx, token)
		if err != nil {
			return err
		}

		// we persist in 3 cases:
		// 1) there was metadata but is not there anymore (wasPresent == true, metadata == nil)
		// 2) the metadata was (potentially) changed (wasPresent == true, metadata = ?)
		// 3) the metadata is newly available (wasPresent == false, metadata != nil)
		token.Status.TokenMetadata = data
		if wasPresent || token.Status.TokenMetadata != nil {
			if err := c.Persist(ctx, token); err != nil {
				return err
			}
		}
	}

	return nil
}
