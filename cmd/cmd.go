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
		return nil, err
	}

	return tokenStorage, nil
}

func createVaultStorage(ctx context.Context, args *CommonCliArgs) (tokenstorage.TokenStorage, error) {
	vaultConfig := vaultstorage.VaultStorageConfigFromCliArgs(&args.VaultCliArgs)
	// use the same metrics registry as the controller-runtime
	vaultConfig.MetricsRegisterer = metrics.Registry
	strg, err := vaultstorage.NewVaultStorage(vaultConfig)
	if err != nil {
		return nil, err
	}
	return strg, nil
}
