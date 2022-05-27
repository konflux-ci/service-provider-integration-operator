package snyk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
var snykUserApiEndpoint *url.URL

func init() {
	qUrl, err := url.Parse("https://snyk.io/api/v1/user/me")
	if err != nil {
		panic(err)
	}
	snykUserApiEndpoint = qUrl
}

func (s metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, err
	}

	username, err := s.fetchUser(data.AccessToken)
	if err != nil {
		return nil, err
	}

	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}
	metadata.Username = username
	return metadata, nil
}

func (s metadataProvider) fetchUser(accessToken string) (userName string, err error) {
	var res *http.Response
	res, err = s.httpClient.Do(&http.Request{
		Method: "GET",
		URL:    snykUserApiEndpoint,
		Header: map[string][]string{
			"Authorization": {"token " + accessToken},
		},
	})
	if err != nil {
		return
	}

	content := map[string]interface{}{}
	if err = json.NewDecoder(res.Body).Decode(&content); err != nil {
		return
	}

	userName = content["username"].(string)

	return
}
