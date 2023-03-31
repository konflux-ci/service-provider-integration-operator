package tokenstorage

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/vaultstorage/vaultcli"
)

func CreateVaultStorage(ctx context.Context, args *vaultcli.VaultCliArgs) (TokenStorage, error) {
	secretStorage, err := vaultcli.CreateVaultStorage(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to construct the secret storage: %w", err)
	}
	return NewJSONSerializingTokenStorage(secretStorage), nil
}

