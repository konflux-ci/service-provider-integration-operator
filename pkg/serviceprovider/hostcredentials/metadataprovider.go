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

package hostcredentials

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	tokenStorage tokenstorage.TokenStorage
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

func (p metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
	lg := log.FromContext(ctx, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		lg.Error(err, "failed to get the token metadata")
		return nil, fmt.Errorf("failed to get the token metadata: %w", err)
	}

	if data == nil {
		return nil, nil
	}
	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	if len(data.Username) > 0 {
		metadata.Username = data.Username
	}
	return metadata, nil
}
