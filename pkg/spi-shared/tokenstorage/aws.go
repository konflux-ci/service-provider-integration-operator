package tokenstorage

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/awsstorage/awscli"
)

func NewAwsTokenStorage(ctx context.Context, args *awscli.AWSCliArgs) (TokenStorage, error) {
	secretStorage, err := awscli.NewAwsSecretStorage(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to construct AWS secret storage: %w", err)
	}

	return NewJSONSerializingTokenStorage(secretStorage), nil
}
