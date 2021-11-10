package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ ServiceProvider = (*Quay)(nil)

type Quay struct {
	Client client.Client
}

func (g *Quay) LookupToken(ctx context.Context, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	// TODO implement
	return nil, nil
}

func (g *Quay) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return getHostWithScheme(repoUrl)
}
