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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"golang.org/x/oauth2"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
)

type gitlabClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
	baseUrl      string
}

var _ serviceprovider.AuthorizedClientBuilder[gitlab.Client] = (*gitlabClientBuilder)(nil)
var tokenNilError = errors.New("token used to construct authorized client is nil")

func (builder *gitlabClientBuilder) CreateAuthorizedClient(ctx context.Context, token *oauth2.Token) (*gitlab.Client, error) {
	if token == nil {
		return nil, tokenNilError
	}
	client, err := gitlab.NewOAuthClient(token.AccessToken, gitlab.WithHTTPClient(builder.httpClient), gitlab.WithBaseURL(builder.baseUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to created new authenticated gitlab client: %w", err)
	}
	return client, nil
}
