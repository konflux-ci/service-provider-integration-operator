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

	"golang.org/x/oauth2"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type RefreshTokenNotSupportedError struct {
}

func (f RefreshTokenNotSupportedError) Error() string {
	return "service provider does not support token refreshing"
}

// RefreshTokenCapability indicates an ability of given SCM provider to refresh issued OAuth access tokens.
type RefreshTokenCapability interface {
	// RefreshToken requests new access token from the service provider using refresh token as authorization.
	// This invalidates the old access token and refresh token
	RefreshToken(ctx context.Context, token *api.Token, config *oauth2.Config) (*api.Token, error)
}
