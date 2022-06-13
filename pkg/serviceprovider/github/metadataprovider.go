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

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	httpClient      *http.Client
	tokenStorage    tokenstorage.TokenStorage
	ghClientBuilder githubClientBuilder
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
func (s metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	state := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	authenticatedContext := httptransport.WithBearerToken(ctx, data.AccessToken)
	ghClient, err := s.ghClientBuilder.createAuthenticatedGhClient(authenticatedContext, token)
	if err != nil {
		return nil, err
	}

	if err := (&AllAccessibleRepos{}).FetchAll(authenticatedContext, ghClient, data.AccessToken, state); err != nil {
		return nil, err
	}

	username, userId, scopes, err := s.fetchUserAndScopes(authenticatedContext)
	if err != nil {
		return nil, err
	}

	js, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	metadata := &api.TokenMetadata{}

	metadata.UserId = userId
	metadata.Username = username
	metadata.Scopes = scopes
	metadata.ServiceProviderState = js

	return metadata, nil
}

// fetchUserAndScopes fetches the scopes and the details of the user associated with the context
func (s metadataProvider) fetchUserAndScopes(ctx context.Context) (userName string, userId string, scopes []string, err error) {
	var res *http.Response
	request := http.Request{
		Method: "GET",
		URL:    githubUserApiEndpoint,
	}
	res, err = s.httpClient.Do(request.WithContext(ctx))
	if err != nil {
		return
	}

	if res.StatusCode != 200 {
		// this should never happen because our http client should already handle the errors so we return a hard
		// error that will cause the whole fetch to fail
		err = fmt.Errorf("unhandled response from the service provider. status code: %d", res.StatusCode)
		return
	}

	// https://docs.github.com/en/developers/apps/building-oauth-apps/scopes-for-oauth-apps
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
