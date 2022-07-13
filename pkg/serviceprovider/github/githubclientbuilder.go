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
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type githubClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

func (g *githubClientBuilder) createAuthenticatedGhClient(ctx context.Context, spiToken *api.SPIAccessToken) (*github.Client, error) {
	tokenData, tsErr := g.tokenStorage.Get(ctx, spiToken)
	lg := log.FromContext(ctx)
	if tsErr != nil {

		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w", spiToken.Namespace, spiToken.Name, tsErr)
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokenData.AccessToken})
	lg.V(logs.DebugLevel).Info("Created new github client", "SPIAccessToken", spiToken)
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}
