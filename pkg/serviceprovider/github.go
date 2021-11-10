package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ ServiceProvider = (*Github)(nil)

type Github struct {
	Client client.Client
}

func (g *Github) LookupToken(ctx context.Context, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	// TODO implement
	return nil, nil
}

func (g *Github) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return getHostWithScheme(repoUrl)
}
