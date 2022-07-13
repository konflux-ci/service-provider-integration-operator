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
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type metadataProvider struct {
	httpClient      *http.Client
	tokenStorage    tokenstorage.TokenStorage
	ghClientBuilder githubClientBuilder
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

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

	ghClient, err := s.ghClientBuilder.createAuthenticatedGhClient(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated GitHub client: %w", err)
	}

	if err := (&AllAccessibleRepos{}).FetchAll(ctx, ghClient, data.AccessToken, state); err != nil {
		return nil, err
	}

	username, userId, scopes, err := s.fetchUserAndScopes(ctx, ghClient)
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
func (s metadataProvider) fetchUserAndScopes(ctx context.Context, githubClient *github.Client) (userName string, userId string, scopes []string, err error) {
	lg := log.FromContext(ctx)
	defer logs.TimeTrack(lg, time.Now(), "fetch user and scopes")
	usr, resp, err := githubClient.Users.Get(ctx, "")
	if err != nil {
		lg.Error(err, "Error during fetching user metadata from Github")
		err = fmt.Errorf("failed to fetch user info: %w", err)
		return
	}
	if resp.StatusCode != 200 {
		lg.Error(err, "Error during fetching user metadata from Github", "status", resp.StatusCode)
		return "", "", nil, nil
	}

	// https://docs.github.com/en/developers/apps/building-oauth-apps/scopes-for-oauth-apps
	scopesString := resp.Header.Get("x-oauth-scopes")

	untrimmedScopes := strings.Split(scopesString, ",")

	for _, s := range untrimmedScopes {
		scopes = append(scopes, strings.TrimSpace(s))
	}

	userId = strconv.FormatInt(*usr.ID, 10)
	userName = *usr.Login
	lg.V(logs.DebugLevel).Info("Fetched user metadata from Github", "login", userName, "userid", userId, "scopes", scopes)
	return
}
