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
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MetadataExpirationPolicy is responsible for the decision whether the metadata of a token should be refreshed or whether
// they are still considered valid.
type MetadataExpirationPolicy interface {
	// IsExpired returns true if the metadata of the supplied token should be refreshed, false otherwise.
	// The implementation can assume that `token.Status.TokenMetadata` is not nil.
	IsExpired(token *api.SPIAccessToken) bool
}

// MetadataExpirationPolicyFunc an adaptor for making a function an implementation of MetadataExpirationPolicy interface.
type MetadataExpirationPolicyFunc func(token *api.SPIAccessToken) bool

var _ MetadataExpirationPolicy = (MetadataExpirationPolicyFunc)(nil)

func (f MetadataExpirationPolicyFunc) IsExpired(token *api.SPIAccessToken) bool {
	return f(token)
}

// MetadataCache acts like a cache of metadata of tokens.
// On top of just CRUDing the token metadata, this struct handles the refreshes of the data when it is determined
// stale.
type MetadataCache struct {
	client           client.Client
	expirationPolicy MetadataExpirationPolicy
}

// TtlMetadataExpirationPolicy is a MetadataExpirationPolicy implementation that checks whether the metadata of the token is
// older than the configured TTL (time to live).
type TtlMetadataExpirationPolicy struct {
	Ttl time.Duration
}

var _ MetadataExpirationPolicy = (*TtlMetadataExpirationPolicy)(nil)

func (t TtlMetadataExpirationPolicy) IsExpired(token *api.SPIAccessToken) bool {
	return time.Now().After(time.Unix(token.Status.TokenMetadata.LastRefreshTime, 0).Add(t.Ttl))
}

// NeverMetadataExpirationPolicy is a MetadataExpirationPolicy that makes the metadata to never expire.
type NeverMetadataExpirationPolicy struct{}

var _ MetadataExpirationPolicy = (*NeverMetadataExpirationPolicy)(nil)

func (t NeverMetadataExpirationPolicy) IsExpired(_ *api.SPIAccessToken) bool {
	return false
}

// NewMetadataCache creates a new cache instance with the provided configuration.
func NewMetadataCache(client client.Client, expirationPolicy MetadataExpirationPolicy) MetadataCache {
	return MetadataCache{
		client:           client,
		expirationPolicy: expirationPolicy,
	}
}

// Persist assigns the last refresh time of the token metadata and updates the token
func (c *MetadataCache) Persist(ctx context.Context, token *api.SPIAccessToken) error {
	if token.Status.TokenMetadata != nil {
		token.Status.TokenMetadata.LastRefreshTime = time.Now().Unix()
	}

	if err := c.client.Status().Update(ctx, token); err != nil {
		return fmt.Errorf("metadata cache error: updating the status of token %s/%s: %w", token.Namespace, token.Name, err)
	}

	return nil
}

// refresh checks if the token's metadata is still valid. If it is stale, the metadata is cleared
func (c *MetadataCache) refresh(token *api.SPIAccessToken) {
	metadata := token.Status.TokenMetadata
	if metadata == nil {
		return
	}

	if c.expirationPolicy.IsExpired(token) {
		token.Status.TokenMetadata = nil
	}
}

// Ensure makes sure that the metadata of the token is either still valid or has been refreshed using
// the MetadataProvider. This method calls Persist if needed.
func (c *MetadataCache) Ensure(ctx context.Context, token *api.SPIAccessToken, ser MetadataProvider) error {
	wasPresent := token.Status.TokenMetadata != nil

	c.refresh(token)

	if token.Status.TokenMetadata == nil {
		auditLog := log.FromContext(ctx, "audit", "true", "namespace", token.Namespace, "token", token.Name)
		if token.Labels[api.ServiceProviderTypeLabel] != "" {
			auditLog = auditLog.WithValues("provider", token.Labels[api.ServiceProviderTypeLabel])
		}
		auditLog.Info("token metadata being fetched or refreshed")
		data, err := ser.Fetch(ctx, token)
		if err != nil {
			auditLog.Error(err, "error fetching token metadata")
			return fmt.Errorf("metadata cache error: fetching token data: %w", err)
		}

		// we persist in 3 cases:
		// 1) there was metadata but is not there anymore (wasPresent == true, metadata == nil)
		// 2) the metadata was (potentially) changed (wasPresent == true, metadata = ?)
		// 3) the metadata is newly available (wasPresent == false, metadata != nil)
		token.Status.TokenMetadata = data
		if wasPresent || token.Status.TokenMetadata != nil {
			if err := c.Persist(ctx, token); err != nil {
				auditLog.Error(err, "Error when storing token metadata")
				return err
			}
			auditLog.Info("token metadata fetched and stored successfully")
		} else {
			auditLog.Info("token metadata could not be read from the provider")
		}
	}

	return nil
}
