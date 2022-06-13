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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/machinebox/graphql"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	graphqlClient *graphql.Client
	httpClient    *http.Client
	tokenStorage  tokenstorage.TokenStorage
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)
var githubUserApiEndpoint *url.URL
var unhandledStatusCode = errors.New("unhandled response from the service provider with status code")

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
		return nil, fmt.Errorf("error while getting token data: %w", err)
	}

	if data == nil {
		return nil, nil
	}

	state := &TokenState{
		AccessibleRepos: map[RepositoryUrl]RepositoryRecord{},
	}
	if err := (&AllAccessibleRepos{}).FetchAll(ctx, s.graphqlClient, data.AccessToken, state); err != nil {
		return nil, fmt.Errorf("error fetching repo metadata: %w", err)
	}

	username, userId, scopes, err := s.fetchUserAndScopes(data.AccessToken)
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

// fetchUserAndScopes fetches the scopes and the details of the user associated with the token
func (s metadataProvider) fetchUserAndScopes(accessToken string) (userName string, userId string, scopes []string, err error) {
	var res *http.Response
	res, err = s.httpClient.Do(&http.Request{
		Method: "GET",
		URL:    githubUserApiEndpoint,
		Header: map[string][]string{
			"Authorization": {"Bearer " + accessToken},
		},
	})
	if err != nil {
		err = fmt.Errorf("error fetching user and scope info: %w", err)
		return
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			ctrl.Log.Error(err, "failed to close the response")
		}
	}()

	if res.StatusCode != 200 {
		// this should never happen because our http client should already handle the errors so we return a hard
		// error that will cause the whole fetch to fail
		err = fmt.Errorf("%w: %d", unhandledStatusCode, res.StatusCode)
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
		err = fmt.Errorf("error parsing the response from github: %w", err)
		return
	}

	userId = strconv.FormatFloat(content["id"].(float64), 'f', -1, 64)
	userName = content["login"].(string)

	return
}
