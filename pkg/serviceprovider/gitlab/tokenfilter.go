package gitlab

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

type tokenFilter struct {
	metadataProvider *metadataProvider
}

func (t tokenFilter) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	//TODO implement me
	panic("implement me")
}
