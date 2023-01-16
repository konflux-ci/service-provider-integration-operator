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
)

const (
	quaySaasHost    = "quay.io"
	quaySaasBaseUrl = "https://" + quaySaasHost
)

var ServiceProviderTypeQuay ServiceProviderType = ServiceProviderType{
	Name: "Quay",
	DefaultOAuthEndpoint: oauth2.Endpoint{
		AuthURL:  quaySaasBaseUrl + "/oauth/authorize",
		TokenURL: quaySaasBaseUrl + "/oauth/access_token",
	},
	DefaultHost:    quaySaasHost,
	DefaultBaseUrl: quaySaasBaseUrl,
}
