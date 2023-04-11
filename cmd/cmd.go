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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

var (
	errUnsupportedTokenStorage = errors.New("unsupported token storage type")
	errNilTokenStorage         = errors.New("nil token storage")
)

func InitTokenStorage(ctx context.Context, args *CommonCliArgs) (tokenstorage.TokenStorage, error) {
	var tokenStorage tokenstorage.TokenStorage
	var errTokenStorage error

	switch args.TokenStorage {
	case VaultTokenStorage:
		tokenStorage, errTokenStorage = tokenstorage.NewVaultStorage(ctx, &args.VaultCliArgs)
	case AWSTokenStorage:
		tokenStorage, errTokenStorage = tokenstorage.NewAwsTokenStorage(ctx, &args.AWSCliArgs)
	default:
		return nil, fmt.Errorf("%w '%s'", errUnsupportedTokenStorage, args.TokenStorage)
	}

	if errTokenStorage != nil {
		return nil, fmt.Errorf("failed to create the token storage '%s': %w", args.TokenStorage, errTokenStorage)
	}

	if tokenStorage == nil {
		return nil, fmt.Errorf("%w: '%s'", errNilTokenStorage, args.TokenStorage)
	}

	if err := tokenStorage.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize token storage: %w", err)
	}

	return tokenStorage, nil
}
