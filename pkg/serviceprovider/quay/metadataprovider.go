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

package quay

import (
	"context"
	"encoding/json"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	tokenStorage tokenstorage.TokenStorage
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

func (s metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	data, err := s.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, err
	}

	// This method is called when we need to refresh (or obtain anew, after cache expiry) the metadata of the token.
	// Because we load all the state iteratively for Quay, this info is always empty when fresh.
	state := &TokenState{
		Repositories:  map[string]EntityRecord{},
		Organizations: map[string]EntityRecord{},
		Users:         map[string]EntityRecord{},
	}

	js, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	if len(data.Username) > 0 {
		metadata.Username = data.Username
		// TODO: The scopes are going to differ per-repo, so we're going to need an SP-specific secret sync
	} else {
		metadata.Username = "$oauthtoken"
		metadata.Scopes = serviceprovider.GetAllScopes(translateToQuayScopes, &token.Spec.Permissions)
	}

	metadata.ServiceProviderState = js

	return metadata, nil
}
