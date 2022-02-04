package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/machinebox/graphql"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	graphqlClient *graphql.Client
	httpClient *http.Client
	tokenStorage  tokenstorage.TokenStorage
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
var githubUserApiEndpoint *url.URL

func init() {
	url, err := url.Parse("https://api.github.com/user")
	if err != nil {
		panic(err)
	}
	githubUserApiEndpoint = url
}

func (s metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) error {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	state := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	if err := (&AllAccessibleRepos{}).FetchAll(ctx, s.graphqlClient, data.AccessToken, state); err != nil {
		return err
	}

	userName, userId, scopes, err := s.fetchUserAndScopes(data.AccessToken)
	if err != nil {
		return err
	}

	js, err := json.Marshal(state)
	if err != nil {
		return err
	}

	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	metadata.UserId = userId
	metadata.UserName = userName
	metadata.Scopes = scopes
	metadata.ServiceProviderState = js

	return nil
}

func (s metadataProvider) fetchUserAndScopes(accessToken string) (userName string, userId string, scopes []string, err error) {
	var res *http.Response
	res, err = s.httpClient.Do(&http.Request{
		Method:           "GET",
		URL:              githubUserApiEndpoint,
		Header: map[string][]string{
			"Authorization": {"Bearer " + accessToken},
		},
	})
	if err != nil {
		return
	}

	scopesString := res.Header.Get("x-oauth-scopes")

	untrimmedScopes := strings.Split(scopesString, ",")

	for _, s := range untrimmedScopes {
		scopes = append(scopes, strings.TrimSpace(s))
	}

	content := map[string]interface{}{}
	if err = json.NewDecoder(res.Body).Decode(&content); err != nil {
		return
	}

	userId = strconv.FormatFloat(content["id"].(float64), 'f', -1, 64)
	userName = content["login"].(string)

	return
}
