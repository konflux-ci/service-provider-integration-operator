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

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/metrics"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/vaultstorage"
)

var (
	errUnsupportedTokenStorage    = errors.New("unsupported token storage type")
	errFailedToCreateTokenStorage = errors.New("failed to create the token storage")
)

func InitTokenStorage(ctx context.Context, args *CommonCliArgs) (tokenstorage.TokenStorage, error) {
	var tokenStorage tokenstorage.TokenStorage
	var errTokenStorage error

	switch args.TokenStorage {
	case VaultTokenStorage:
		tokenStorage, errTokenStorage = createVaultStorage(ctx, args)
	default:
		return nil, fmt.Errorf("%w '%s'", errUnsupportedTokenStorage, args.TokenStorage)
	}

	if errTokenStorage != nil {
		return nil, errTokenStorage
	}

	if tokenStorage == nil {
		return nil, fmt.Errorf("%w '%s'", errFailedToCreateTokenStorage, args.TokenStorage)
	}

	if err := tokenStorage.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize token storage: %w", err)
	}

	return tokenStorage, nil
}

func createVaultStorage(ctx context.Context, args *CommonCliArgs) (tokenstorage.TokenStorage, error) {
	vaultConfig := vaultstorage.VaultStorageConfigFromCliArgs(&args.VaultCliArgs)
	// use the same metrics registry as the controller-runtime
	vaultConfig.MetricsRegisterer = metrics.Registry
	strg, err := vaultstorage.NewVaultStorage(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault token storage: %w", err)
	}
	return strg, nil
}
