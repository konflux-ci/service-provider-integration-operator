package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/machinebox/graphql"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type stateSerializer struct {
	client       *graphql.Client
	tokenStorage tokenstorage.TokenStorage
}

var _ serviceprovider.StateSerializer = (*stateSerializer)(nil)

func (s stateSerializer) Serialize(ctx context.Context, token *api.SPIAccessToken) ([]byte, error) {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("token data is missing")
	}

	repos, err := AllAccessibleRepos{}.FetchAll(ctx, s.client, data.AccessToken)
	if err != nil {
		return nil, err
	}

	js, err := json.Marshal(repos)
	if err != nil {
		return nil, err
	}

	return js, nil
}
