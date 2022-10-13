package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/go-retryablehttp"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type metadataProvider struct {
	tokenStorage    tokenstorage.TokenStorage
	httpClient      *http.Client
	glClientBuilder gitlabClientBuilder
	baseUrl         string
}

func (p metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	lg := log.FromContext(ctx, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		lg.Error(err, "failed to get the token metadata")
		return nil, fmt.Errorf("failed to get the token metadata: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	state := &TokenState{}

	glClient, err := p.glClientBuilder.createAuthenticatedGlClient(ctx, token, p.baseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated GitHub client: %w", err)
	}

	//if err := (&AllAccessibleRepos{}).FetchAll(ctx, ghClient, data.AccessToken, state); err != nil {
	//	return nil, err
	//}

	username, userId, scopes, err := p.fetchUserAndScopes(ctx, glClient)
	if err != nil {
		return nil, err
	}

	js, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("error marshalling the state: %w", err)
	}

	metadata := &api.TokenMetadata{}

	metadata.UserId = userId
	metadata.Username = username
	metadata.Scopes = scopes
	metadata.ServiceProviderState = js

	return metadata, nil
}

// fetchUserAndScopes fetches the scopes and the details of the user associated with the context
func (p metadataProvider) fetchUserAndScopes(ctx context.Context, gitlabClient *gitlab.Client) (userName string, userId string, scopes []string, err error) {
	lg := log.FromContext(ctx)
	usr, resp, err := gitlabClient.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		lg.Error(err, "error during fetching user metadata from GitLab")
		err = fmt.Errorf("failed to fetch user info: %w", err)
		return
	}
	if resp.StatusCode != 200 {
		lg.Error(err, "error during fetching user metadata from GitLab", "status", resp.StatusCode)
		return "", "", nil, nil
	}

	type tokenResponseBody struct {
		scope []string
	}
	responseBody := tokenResponseBody{}

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", p.baseUrl+"/oauth/token/info", nil)
	if err != nil {
		return "", "", nil, err
	}
	tokenResp, err := gitlabClient.Do(req, &responseBody)
	if err != nil {
		return "", "", nil, err
	}
	if tokenResp.StatusCode != 200 {
		lg.Error(err, "error during fetching user metadata from GitLab", "status", resp.StatusCode)
		return "", "", nil, nil
	}

	scopes = responseBody.scope
	userId = strconv.FormatInt(int64(usr.ID), 10)
	userName = usr.Username
	lg.V(logs.DebugLevel).Info("Fetched user metadata from GitLab", "login", userName, "userid", userId, "scopes", scopes)
	return
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
