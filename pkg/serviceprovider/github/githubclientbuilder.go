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
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"github.com/google/go-github/v45/github"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
)

type githubClientBuilder struct {
	httpClient   *http.Client
	tokenStorage tokenstorage.TokenStorage
}

func (g githubClientBuilder) CreateAuthenticatedClient(ctx context.Context, credentials serviceprovider.Credentials) (*github.Client, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: credentials.Token})
	// We need the githubBaseUrl in the githubClientBuilder struct to decide whether to create
	// regular GitHub client or an Enterprise client through the GitHub library's NewEnterpriseClient() function.
	// However, the function takes 2 URLs as parameters: one for the API URL and one for the upload URL where files can be uploaded.
	// As we currently do not allow uploading files to GitHub, we might omit the uploadURL.
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}

var _ serviceprovider.AuthenticatedClientBuilder[github.Client] = (*githubClientBuilder)(nil)
