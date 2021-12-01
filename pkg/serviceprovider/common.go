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

package serviceprovider

import (
	"context"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getHostWithScheme(repoUrl string) (string, error) {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

type TokenFilterFunction interface {
	Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error)
}

func TokenFilter(f func(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error)) TokenFilterFunction {
	return &customFilter{f: f}
}

var FirstToken = TokenFilter(func(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error) {
	return true, nil
})

func CommonLookup(sp ServiceProvider, ctx context.Context, binding *api.SPIAccessTokenBinding, filters ...TokenFilterFunction) (*api.SPIAccessToken, error) {
	repoUrl, err := url.Parse(binding.Spec.RepoUrl)
	if err != nil {
		return nil, err
	}

	repoHost := repoUrl.Host

	ats := &api.SPIAccessTokenList{}
	if err := sp.List(ctx, ats, client.InNamespace(binding.Namespace), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
		api.ServiceProviderHostLabel: repoHost,
	}); err != nil {
		return nil, err
	}

	if len(ats.Items) == 0 {
		return nil, nil
	}

	allFilters := []TokenFilterFunction{
		&serviceProviderUrlCheck{sp: sp},
		&scopeCheck{},
	}
	for _, f := range filters {
		allFilters = append(allFilters, f)
	}

Tokens:
	for _, t := range ats.Items {
		for _, f := range allFilters {
			res, err := f.Matches(binding, &t)
			if err != nil {
				return nil, err
			}
			if !res {
				continue Tokens
			}
		}

		// just return the first matching
		return &t, nil
	}

	return nil, nil
}

type customFilter struct {
	f func(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error)
}

func (cf *customFilter) Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error) {
	return cf.f(binding, token)
}

type serviceProviderUrlCheck struct {
	sp ServiceProvider
}

func (c *serviceProviderUrlCheck) Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error) {
	spUrl, err := c.sp.GetServiceProviderUrlForRepo(binding.Spec.RepoUrl)
	if err != nil {
		return false, err
	}

	return spUrl == token.Spec.ServiceProviderUrl, nil
}

type scopeCheck struct{}

func (c *scopeCheck) Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error) {
Required:
	for _, required := range binding.Spec.Scopes {
		for _, defined := range token.Spec.TokenMetadata.Scopes {
			if defined == required {
				continue Required
			}
		}

		return false, nil
	}

	return true, nil
}
