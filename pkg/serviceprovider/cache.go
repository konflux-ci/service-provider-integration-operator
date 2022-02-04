package serviceprovider

import (
	"context"
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MetadataCache acts like a cache of metadata of tokens.
// On top of just CRUDing the token metadata, this struct handles the refreshes of the data when it is determined
// stale.
type MetadataCache struct {
	// Ttl limits how long the data stays in the cache
	Ttl time.Duration

	client client.Client
	sync   sync.Syncer
}

// NewMetadataCache creates a new cache instance with the provided configuration.
func NewMetadataCache(ttl time.Duration, client client.Client) MetadataCache {
	return MetadataCache{
		Ttl:    ttl,
		client: client,
		sync:   sync.New(client),
	}
}

// Persist assigns the last refresh time of the token metadata and updates the token
func (c *MetadataCache) Persist(ctx context.Context, token *api.SPIAccessToken) error {
	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata := &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	metadata.LastRefreshTime = time.Now().Unix()

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
	c.Refresh(token)

	if token.Status.TokenMetadata == nil {
		if err := ser.Fetch(ctx, token); err != nil {
			return err
		}

		if token.Status.TokenMetadata != nil {
			if err := c.Persist(ctx, token); err != nil {
				return err
			}
		}
	}

	return nil
}
