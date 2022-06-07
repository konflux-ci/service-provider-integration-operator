package snyk

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

var httpCl = &http.Client{
	Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL == snykUserApiEndpoint && strings.HasPrefix(r.Header.Get("Authorization"), "token ") {
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`{"id": "123abc", "username": "test_user"}`))),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 404,
			}, nil
		}
	}),
}

var ts = tokenstorage.TestTokenStorage{
	GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{
			AccessToken:  "access",
			TokenType:    "fake",
			RefreshToken: "refresh",
			Expiry:       0,
		}, nil
	},
}

func TestMetadataProvider_FetchUserIdAndName(t *testing.T) {

	mp := metadataProvider{
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "123abc", data.UserId)
	assert.Equal(t, "test_user", data.Username)
}
