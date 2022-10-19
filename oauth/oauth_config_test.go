package oauth

import (
	"context"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	"testing"
)

const (
	testClientId     = "test_client_id_123"
	testClientSecret = "test_client_secret_123"
	testAuthUrl      = "test_auth_url_123"
	testTokenUrl     = "test_token_url_123"
)

func TestCreateOauthConfigFromSecret(t *testing.T) {
	t.Run("all fields set ok", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				secretFieldClientId:     []byte(testClientId),
				secretFieldClientSecret: []byte(testClientSecret),
				secretFieldAuthUrl:      []byte(testAuthUrl),
				secretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := createConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, testAuthUrl, oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, testTokenUrl, oauthCfg.Endpoint.TokenURL)
	})

	t.Run("error if missing client id", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				secretFieldClientSecret: []byte(testClientSecret),
				secretFieldAuthUrl:      []byte(testAuthUrl),
				secretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := createConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("error if missing client secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				secretFieldClientId: []byte(testClientId),
				secretFieldAuthUrl:  []byte(testAuthUrl),
				secretFieldTokenUrl: []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := createConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("ok with just client id and secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				secretFieldClientId:     []byte(testClientId),
				secretFieldClientSecret: []byte(testClientSecret),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := createConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, "", oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, "", oauthCfg.Endpoint.TokenURL)
	})
}

func TestFindOauthConfigSecret(t *testing.T) {
	ctrl := commonController{
		Config:           config.ServiceProviderConfiguration{},
		K8sClient:        IT.Client,
		TokenStorage:     nil,
		Endpoint:         oauth2.Endpoint{},
		BaseUrl:          "",
		RedirectTemplate: nil,
		Authenticator:    nil,
		StateStorage:     nil,
	}

	ctx := context.TODO()
	oauthState := &oauthstate.OAuthInfo{
		TokenName:           "",
		TokenNamespace:      "",
		TokenKcpWorkspace:   "",
		Scopes:              nil,
		ServiceProviderType: "",
		ServiceProviderUrl:  "",
	}

	found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)

	assert.False(t, found)
	assert.Nil(t, secret)
	assert.Error(t, err)
}
