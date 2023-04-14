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
	"errors"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/remotesecretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
)

type SecretBuilder struct {
	RemoteSecret *api.RemoteSecret
	Storage      remotesecretstorage.RemoteSecretStorage
}

func (sb *SecretBuilder) GetData(ctx context.Context, obj *api.RemoteSecret) (map[string][]byte, string, error) {
	data, err := sb.Storage.Get(ctx, obj)
	if err != nil {
		if errors.Is(err, secretstorage.NotFoundError) {
			return map[string][]byte{}, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), fmt.Errorf("%w: %s", bindings.AccessTokenDataNotFoundError, err.Error())
		}
		return nil, string(api.SPIAccessTokenBindingErrorReasonTokenRetrieval), fmt.Errorf("failed to get the token data from token storage: %w", err)
	}

	return *data, string(api.SPIAccessTokenBindingErrorReasonNoError), nil
}

var _ bindings.SecretBuilder[*api.RemoteSecret] = (*SecretBuilder)(nil)
