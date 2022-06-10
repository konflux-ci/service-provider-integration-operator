package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type githubClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

func (g *githubClientBuilder) createAuthenticatedGhClient(ctx context.Context, spiToken *api.SPIAccessToken) (*github.Client, error) {
	token, tsErr := g.tokenStorage.Get(ctx, spiToken)
	lg := log.FromContext(ctx)
	if tsErr != nil {

		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, tsErr
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.AccessToken})
	lg.V(1).Info("Created new github client", "SPIAccessToken", spiToken)
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}
