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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"
)

type OAuthCapability interface {
	// GetOAuthEndpoint returns the URL of the OAuth initiation. This must point to the SPI oauth service, NOT
	//the service provider itself.
	GetOAuthEndpoint() string

	// OAuthScopesFor translates all the permissions into a list of service-provider-specific scopes. This method
	// is used to compose the OAuth flow URL. There is a generic helper, GetAllScopes, that can be used if all that is
	// needed is just a translation of permissions into scopes.
	OAuthScopesFor(permissions *api.Permissions) []string
}

type DefaultOAuthCapability struct {
	BaseUrl string
}

func (o *DefaultOAuthCapability) GetOAuthEndpoint() string {
	return o.BaseUrl + oauth.AuthenticateRoutePath
}
