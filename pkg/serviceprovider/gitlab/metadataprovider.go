package gitlab

import (
	"context"
	"net/http"
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type metadataProvider struct {
	tokenStorage     tokenstorage.TokenStorage
	httpClient       *http.Client
	kubernetesClient client.Client
	ttl              time.Duration
}

func (m metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	//TODO implement me
	panic("implement me")
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
