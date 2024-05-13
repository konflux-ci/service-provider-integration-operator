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
	"fmt"
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
)

type gitlabClientBuilder struct {
	httpClient    *http.Client
	tokenStorage  tokenstorage.TokenStorage
	gitlabBaseUrl string
}

var _ serviceprovider.AuthenticatedClientBuilder[gitlab.Client] = (*gitlabClientBuilder)(nil)

func (builder gitlabClientBuilder) CreateAuthenticatedClient(ctx context.Context, credentials serviceprovider.Credentials) (*gitlab.Client, error) {
	var client *gitlab.Client
	var err error
	if credentials.Username == "" {
		client, err = gitlab.NewClient(credentials.Token, gitlab.WithHTTPClient(builder.httpClient), gitlab.WithBaseURL(builder.gitlabBaseUrl))
	} else {
		client, err = gitlab.NewBasicAuthClient(credentials.Username, credentials.Token, gitlab.WithHTTPClient(builder.httpClient), gitlab.WithBaseURL(builder.gitlabBaseUrl))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to created new authenticated GitLab client: %w", err)
	}
	return client, nil
}
