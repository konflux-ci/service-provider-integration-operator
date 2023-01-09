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

package gitlab

import (
	"context"
	"errors"
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

var accessTokenNotFoundError = errors.New("token data is not found in token storage")

func (builder *gitlabClientBuilder) createGitlabAuthClient(ctx context.Context, spiAccessToken *api.SPIAccessToken, baseUrl string) (*gitlab.Client, error) {
	lg := log.FromContext(ctx)
	tokenData, err := builder.tokenStorage.Get(ctx, spiAccessToken)
	if err != nil {
		lg.Error(err, "failed to get token from storage for", "token", spiAccessToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w",
			spiAccessToken.Namespace, spiAccessToken.Name, err)
	}

	if tokenData == nil {
		lg.Error(accessTokenNotFoundError, "token data not found", "token-name", spiAccessToken.Name)
		return nil, accessTokenNotFoundError
	}
	client, err := gitlab.NewOAuthClient(tokenData.AccessToken, gitlab.WithHTTPClient(builder.httpClient), gitlab.WithBaseURL(baseUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to created new authenticated gitlab client for SPIAccessToken %s/%s: %w",
			spiAccessToken.Namespace, spiAccessToken.Name, err)
	}
	lg.V(logs.DebugLevel).Info("new authenticated gitlab client successfully created", "SPIAccessToken", spiAccessToken)
	return client, nil
}
