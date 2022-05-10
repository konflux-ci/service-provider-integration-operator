package quay

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestDockerLogin(t *testing.T) {
	cl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			assert.Contains(t, r.Header, "Authorization")
			// the value is base64 encoded "alois:password"
			if r.Header.Get("Authorization") == "Basic YWxvaXM6cGFzc3dvcmQ=" {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(`{"token": "token"}`)),
				}, nil
			} else {
				return &http.Response{
					StatusCode: 403,
				}, nil
			}
		}),
	}

	t.Run("successful login", func(t *testing.T) {
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "password")
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})

	t.Run("unsuccessful login", func(t *testing.T) {
		token, err := DockerLogin(context.TODO(), cl, "acme/foo", "alois", "passwort")
		assert.Empty(t, token)
		assert.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "login did not succeed"))
	})
}

func TestAnalyzeLoginToken(t *testing.T) {
	// This would just test that we can parse a JWT token using a library.
}
