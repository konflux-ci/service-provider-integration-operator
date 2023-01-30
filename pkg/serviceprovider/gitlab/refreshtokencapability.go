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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type refreshTokenCapability struct {
	httpClient          *http.Client
	gitlabBaseUrl       string
	oauthServiceBaseUrl string
}

var nonOkResponseError = errors.New("GitLab responded with non-ok status")

var _ serviceprovider.RefreshTokenCapability = (*refreshTokenCapability)(nil)

func (r refreshTokenCapability) RefreshToken(ctx context.Context, token *api.Token, config *oauth2.Config) (*api.Token, error) {
	lg := log.FromContext(ctx)

	v := url.Values{}
	v.Set("client_id", config.ClientID)
	v.Set("client_secret", config.ClientSecret)
	v.Set("refresh_token", token.RefreshToken)
	v.Set("grant_type", "refresh_token")
	v.Set("redirect_uri", r.oauthServiceBaseUrl+oauth.CallBackRoutePath)
	requestUrl := fmt.Sprintf("%s/oauth/token?%s", r.gitlabBaseUrl, v.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a request to refresh a token: %w", err)
	}

	req.Header.Add("content-type", "application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request a refresh token: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "unable to close body of refresh token response")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		lg.Error(nonOkResponseError, "cannot refresh token", "status code", resp.StatusCode)
		return nil, nonOkResponseError
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body of refresh token response: %w", err)
	}

	refreshResponse := struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		Expiry       uint64 `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		CreationTime uint64 `json:"created_at"`
	}{}

	err = json.Unmarshal(body, &refreshResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the refresh token response: %w", err)
	}

	return &api.Token{
		AccessToken:  refreshResponse.AccessToken,
		TokenType:    refreshResponse.TokenType,
		RefreshToken: refreshResponse.RefreshToken,
		Expiry:       refreshResponse.Expiry,
	}, nil
}
