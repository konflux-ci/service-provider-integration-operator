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
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type githubClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

var accessTokenNotFoundError = errors.New("token data is not found in token storage")

func (g *githubClientBuilder) createAuthenticatedGhClient(ctx context.Context, spiToken *api.SPIAccessToken) (*github.Client, error) {
	tokenData, tsErr := g.tokenStorage.Get(ctx, spiToken)
	lg := log.FromContext(ctx)
	if tsErr != nil {

		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w", spiToken.Namespace, spiToken.Name, tsErr)
	}
	if tokenData == nil {
		lg.Error(accessTokenNotFoundError, "token data not found", "token-name", spiToken.Name)
		return nil, accessTokenNotFoundError
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokenData.AccessToken})
	lg.V(logs.DebugLevel).Info("Created new github client", "SPIAccessToken", spiToken)

	// We need the githubBaseUrl in the githubClientBuilder struct to decide whether to create
	// regular GitHub client or an Enterprise client through the GitHub library's NewEnterpriseClient() function.
	// However, the function takes 2 URLs as parameters: one for the API URL and one for the upload URL where files can be uploaded.
	// As we currently do not allow uploading files to GitHub, we might omit the uploadURL.
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}
