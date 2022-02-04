package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// MetadataProvider is a function that converts a fills in the metadata in the token's status with
// service-provider-specific information used for token matching.
type MetadataProvider interface {
	// Fetch tries to fetch the token metadata and assign it in the token. Note that the metadata of the token may or
	// may not be nil and this method shouldn't change it unless there is data to assign.
	Fetch(ctx context.Context, token *api.SPIAccessToken) error
}

type MetadataProviderFunc func(ctx context.Context, token *api.SPIAccessToken) error

var _ MetadataProvider = (MetadataProviderFunc)(nil)

func (f MetadataProviderFunc) Fetch(ctx context.Context, token *api.SPIAccessToken) error {
	return f(ctx, token)
}
