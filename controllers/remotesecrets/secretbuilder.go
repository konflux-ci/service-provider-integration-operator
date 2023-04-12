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

package remotesecrets

import (
	"context"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	dependents "github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type SecretBuilder struct {
	Binding         *api.SPIAccessTokenBinding
	TokenStorage    tokenstorage.TokenStorage
	ServiceProvider serviceprovider.ServiceProvider
}

// GetData implements dependents.SecretBuilder
func (sb *SecretBuilder) GetData(ctx context.Context, tokenObject *api.SPIAccessToken) (map[string][]byte, string, error) {
	token, err := sb.TokenStorage.Get(ctx, tokenObject)
	if err != nil {
		return nil, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), fmt.Errorf("failed to get the token data from token storage: %w", err)
	}

	if token == nil {
		return nil, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), bindings.AccessTokenDataNotFoundError
	}

	at, err := sb.ServiceProvider.MapToken(ctx, sb.Binding, tokenObject, token)
	if err != nil {
		return nil, string(api.SPIAccessTokenBindingErrorReasonTokenAnalysis), fmt.Errorf("failed to analyze the token to produce the mapping to the secret: %w", err)
	}

	stringData, err := at.ToSecretType(sb.Binding.Spec.Secret.Type, &sb.Binding.Spec.Secret.Fields)
	if err != nil {
		return nil, string(api.SPIAccessTokenBindingErrorReasonTokenAnalysis), fmt.Errorf("failed to create data to be injected into the secret: %w", err)
	}

	// copy the string data into the byte-array data so that sync works reliably. If we didn't sync, we could have just
	// used the Secret.StringData, but Sync gives us other goodies.
	// So let's bite the bullet and convert manually here.
	data := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		data[k] = []byte(v)
	}

	return data, string(api.SPIAccessTokenBindingErrorReasonNoError), nil
}

var _ dependents.SecretBuilder[*api.SPIAccessToken] = (*SecretBuilder)(nil)