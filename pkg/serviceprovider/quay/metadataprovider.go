//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quay

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
	quay         Quay
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
var quayUserApiEndpoint *url.URL

func init() {
	qUrl, err := url.Parse("https://quay.io/api/v1/user")
	if err != nil {
		panic(err)
	}
	quayUserApiEndpoint = qUrl
}

func (s metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, err
	}

	state := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	//TODO: verify access

	username, err := s.fetchUser(data.AccessToken)
	if err != nil {
		return nil, err
	}

	js, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	scopes := make([]string, 0)
	for _, p := range token.Spec.Permissions.Required {
		scopes = append(scopes, s.quay.TranslateToScopes(p)...)
	}

	metadata.Username = username
	metadata.Scopes = scopes
	metadata.ServiceProviderState = js

	return metadata, nil
}

// fetchUserAndScopes fetches the scopes and the details of the user associated with the token
func (s metadataProvider) fetchUser(accessToken string) (userName string, err error) {
	var res *http.Response
	res, err = s.httpClient.Do(&http.Request{
		Method: "GET",
		URL:    quayUserApiEndpoint,
		Header: map[string][]string{
			"Authorization": {"Bearer " + accessToken},
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
