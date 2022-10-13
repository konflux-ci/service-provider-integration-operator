package gitlab

import (
	"context"
	"fmt"
	"net/http"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type gitlabClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

func (g *gitlabClientBuilder) createAuthenticatedGlClient(ctx context.Context, spiAccessToken *api.SPIAccessToken, baseUrl string) (*gitlab.Client, error) {
	lg := log.FromContext(ctx)
	tokenData, err := g.tokenStorage.Get(ctx, spiAccessToken)
	if err != nil {
		lg.Error(err, "failed to get token from storage for", "token", spiAccessToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w", spiAccessToken.Namespace, spiAccessToken.Name, err)
	}
	lg.V(logs.DebugLevel).Info("Created new github client", "SPIAccessToken", spiAccessToken)
	return gitlab.NewOAuthClient(tokenData.AccessToken, gitlab.WithHTTPClient(g.httpClient), gitlab.WithBaseURL(baseUrl))

}
