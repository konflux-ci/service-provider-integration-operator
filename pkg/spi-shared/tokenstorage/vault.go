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

package tokenstorage

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/vaultstorage/vaultcli"
)

func NewVaultStorage(ctx context.Context, args *vaultcli.VaultCliArgs) (TokenStorage, error) {
	secretStorage, err := vaultcli.CreateVaultStorage(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to construct the secret storage: %w", err)
	}
	return NewJSONSerializingTokenStorage(secretStorage), nil
}
