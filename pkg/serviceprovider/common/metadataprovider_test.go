package common

import (
	"context"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

var ts = tokenstorage.TestTokenStorage{
	GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{
			Username:     "test_user",
			AccessToken:  "access",
			TokenType:    "fake",
			RefreshToken: "refresh",
			Expiry:       0,
		}, nil
	},
}

func TestMetadataProvider_FetchUserIdAndName(t *testing.T) {

	mp := metadataProvider{
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "test_user", data.Username)
}
