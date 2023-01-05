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

package config

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// ServiceProviderDefaults configuration containing default values used to initialize supported service providers
type ServiceProviderDefaults struct {
	SpType   ServiceProviderType
	Endpoint oauth2.Endpoint
	UrlHost  string // default host of service provider. ex.: `github.com`
	BaseUrl  string // default base url of service provider, typically scheme+host. ex: `https://github.com`
}

// all servise provider types we support, including default values
var SupportedServiceProvidersDefaults []ServiceProviderDefaults = []ServiceProviderDefaults{
	{
		SpType:   ServiceProviderTypeGitHub,
		Endpoint: github.Endpoint,
		UrlHost:  GithubSaasHost,
		BaseUrl:  GithubSaasBaseUrl,
	},
	{
		SpType:   ServiceProviderTypeQuay,
		Endpoint: QuayEndpoint,
		UrlHost:  QuaySaasHost,
		BaseUrl:  QuaySaasBaseUrl,
	},
	{
		SpType:   ServiceProviderTypeGitLab,
		Endpoint: GitlabEndpoint,
		UrlHost:  GitlabSaasHost,
		BaseUrl:  GitlabSaasBaseUrl,
	},
}
