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
)

// MetadataProvider is a function that converts a fills in the metadata in the token's status with
// service-provider-specific information used for token matching.
type MetadataProvider interface {
	// Fetch tries to fetch the token metadata and assign it in the token. Note that the metadata of the token may or
	// may not be nil and this method shouldn't change it unless there is data to assign.
	// Implementors should make sure to return some errors.ServiceProviderError if the failure to fetch the metadata is
	// caused by the token or service provider itself and not other environmental reasons.
	//
	// The `includeState` governs the initialization of the `api.TokenMetadata.ServiceProviderState` field.
	// If it is false, the provider can leave it out, leading to reduced time needed to initialize the metadata.
	// This field is assumed to not be needed if the token match policy is set to "any" (which is currently the default).
	Fetch(ctx context.Context, token *api.SPIAccessToken, includeState bool) (*api.TokenMetadata, error)
}

type MetadataProviderFunc func(ctx context.Context, token *api.SPIAccessToken, includeState bool) (*api.TokenMetadata, error)

var _ MetadataProvider = (MetadataProviderFunc)(nil)

func (f MetadataProviderFunc) Fetch(ctx context.Context, token *api.SPIAccessToken, includeState bool) (*api.TokenMetadata, error) {
	return f(ctx, token, includeState)
}
