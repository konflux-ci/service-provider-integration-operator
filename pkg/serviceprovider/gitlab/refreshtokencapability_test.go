package gitlab

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func mockRefreshTokenCapability(returnCode int, body string, responseError error) *refreshTokenCapability {
	httpClientMock := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: returnCode,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBuffer([]byte(body))),
				Request:    r,
			}, responseError
		}),
	}

	return &refreshTokenCapability{
		httpClient:          httpClientMock,
		gitlabBaseUrl:       "",
		oauthServiceBaseUrl: "",
	}
}

func TestRefreshToken(t *testing.T) {
	refreshCapability := mockRefreshTokenCapability(http.StatusOK, `{"access_token": "42"}`, nil)

	token, err := refreshCapability.RefreshToken(context.TODO(), &api.Token{}, "hello", "world")

	assert.NoError(t, err)
	assert.Equal(t, "42", token.AccessToken)
}

func TestRefreshTokenError(t *testing.T) {
	test := func(refreshCapabilityInstance refreshTokenCapability, expectedErrorSubstring string) {
		t.Run(fmt.Sprintf("should return error containing: %s", expectedErrorSubstring), func(t *testing.T) {
			_, err := refreshCapabilityInstance.RefreshToken(context.TODO(), &api.Token{}, "hello", "world")
			assert.ErrorContains(t, err, expectedErrorSubstring)
		})
	}

	test(*mockRefreshTokenCapability(http.StatusOK, `{"access_token": 42}`, nil), "failed to unmarshal")
	test(*mockRefreshTokenCapability(http.StatusUnauthorized, "", nil), "non-ok status")
	test(*mockRefreshTokenCapability(http.StatusOK, "", errors.New("request error")), "failed to request")
}
