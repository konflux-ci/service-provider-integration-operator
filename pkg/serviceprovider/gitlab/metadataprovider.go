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
	"net/http"
	"strconv"

	k8sMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type metadataProvider struct {
	tokenStorage    tokenstorage.TokenStorage
	httpClient      *http.Client
	glClientBuilder gitlabClientBuilder
	baseUrl         string
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

const gitlabOAuthTokenInfoPath = "oauth/token/info" //#nosec G101 -- no security risk, just an API endpoint
const gitlabPatInfoPath = "personal_access_tokens/self"

var gitlabNonOkError = errors.New("GitLab responded with non-ok status code")

var metadataFetchMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: config.MetricsNamespace,
	Subsystem: config.MetricsSubsystem,
	Name:      "gitlab_token_metadata_fetch_seconds",
	Help:      "The overall time to fetch the metadata for a single repository",
}, []string{"failure"})

// pre-create the individual metrics for each label value for perf reasons
var metadataFetchSuccessMetric = metadataFetchMetric.WithLabelValues("false")
var metadataFetchFailureMetric = metadataFetchMetric.WithLabelValues("true")

func init() {
	k8sMetrics.Registry.MustRegister(metadataFetchMetric)
}

func metadataFetchTimer() metrics.ValueTimer2[*api.TokenMetadata, error] {
	return metrics.NewValueTimer2[*api.TokenMetadata, error](metrics.ValueObserverFunc2[*api.TokenMetadata, error](func(m *api.TokenMetadata, err error, metric float64) {
		if err == nil {
			if m != nil {
				// only collect the success if there was any metadata actually fetched. If there was no error and no
				// metadata fetched, there must have been no token therefore it makes no sense to even talk about
				// metadata fetching.
				metadataFetchSuccessMetric.Observe(metric)
			}
		} else {
			metadataFetchFailureMetric.Observe(metric)
		}
	}))
}

func (p metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken, includeState bool) (*api.TokenMetadata, error) {
	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get the token metadata: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	timer := metadataFetchTimer()
	return timer.ObserveValuesAndDuration(p.doFetch(ctx, token, includeState))
}

func (p metadataProvider) doFetch(ctx context.Context, token *api.SPIAccessToken, includeState bool) (*api.TokenMetadata, error) {
	lg := log.FromContext(ctx, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	state := &TokenState{}

	glClient, err := p.glClientBuilder.createGitlabAuthClient(ctx, token, p.baseUrl)
	if err != nil {
		return nil, err
	}

	username, userId, err := p.fetchUser(ctx, glClient)
	if err != nil {
		return nil, err
	}

	scopes, err := p.fetchOAuthScopes(ctx, glClient)
	if err != nil {
		return nil, err
	}

	if scopes == nil {
		lg.Info("could not obtain token scopes from OAuth API, proceeding to PAT API")
		scopes, err = p.fetchPATScopes(ctx, glClient)
		if err != nil {
			return nil, err
		}
	}

	// TODO: In the future we can figure out scopes by making request for different resources similarly to how we do it with Quay.
	lg.V(logs.DebugLevel).Info("fetched user metadata from GitLab", "login", username, "userid", userId, "scopes", scopes)

	metadata := &api.TokenMetadata{
		Username: username,
		UserId:   userId,
		Scopes:   scopes,
	}

	if !includeState {
		return metadata, nil
	}

	// Service provider state is currently expected to be empty json.
	metadata.ServiceProviderState, err = json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("error marshalling the state: %w", err)
	}

	return metadata, nil
}

func (p metadataProvider) fetchUser(ctx context.Context, gitlabClient *gitlab.Client) (userName string, userId string, err error) {
	lg := log.FromContext(ctx)
	usr, resp, err := gitlabClient.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch user metadata from GitLab: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body when fetching user metadata from GitLab")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to fetch user metadata due to %d status code: %w",
			resp.StatusCode, gitlabNonOkError)
	}

	return usr.Username, strconv.FormatInt(int64(usr.ID), 10), nil
}

func (p metadataProvider) fetchOAuthScopes(ctx context.Context, gitlabClient *gitlab.Client) ([]string, error) {
	lg := log.FromContext(ctx)
	tokenInfoResponse := struct {
		Scopes []string `json:"scope"`
	}{}

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, p.baseUrl+"/"+gitlabOAuthTokenInfoPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request to fetch oauth token scopes: %w", err)
	}

	res, err := gitlabClient.Do(req, &tokenInfoResponse)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusUnauthorized {
			// GitLab client returns an error in case the response is 401, but we would like to try
			// PAT API to fetch the scopes, so we do not return the error in this case.
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch oauth token scopes: %w", err)
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body when fetching oauth token scopes from GitLab")
		}
	}()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch oauth token scopes due to %d status code: %w",
			res.StatusCode, gitlabNonOkError)
	}

	return tokenInfoResponse.Scopes, nil
}

func (p metadataProvider) fetchPATScopes(ctx context.Context, gitlabClient *gitlab.Client) ([]string, error) {
	lg := log.FromContext(ctx)
	patInfoResponse := struct {
		Scopes []string `json:"scopes"`
	}{}

	req, err := gitlabClient.NewRequest(http.MethodGet, gitlabPatInfoPath, nil, []gitlab.RequestOptionFunc{gitlab.WithContext(ctx)})
	if err != nil {
		return nil, fmt.Errorf("failed to construct request to fetch PAT token scopes: %w", err)
	}

	res, err := gitlabClient.Do(req, &patInfoResponse)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusNotFound {
			// Endpoint for fetching PAT scopes not found in this GitLab, just return empty slice.
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to fetch PAT token scopes: %w", err)
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			lg.Error(err, "failed to close response body when fetching PAT token scopes from GitLab")
		}
	}()

	return patInfoResponse.Scopes, nil
}
