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

package awsstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/logging"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type secretManagerTokenStorage struct {
	client *secretsmanager.Client
}

const awsDataPathFormat = "%s/data/%s/%s"

var _ tokenstorage.TokenStorage = (*secretManagerTokenStorage)(nil)

type AWSCliArgs struct {
}

func configFromCliArgs(ctx context.Context, args *AWSCliArgs) (*aws.Config, error) {
	lg := log.FromContext(ctx)
	lg.Info("creating aws config")

	awsLogger := logging.NewStandardLogger(os.Stdout)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigFiles([]string{"/Users/mvala/.aws/config"}),
		config.WithSharedCredentialsFiles([]string{"/Users/mvala/.aws/credentials"}),
		config.WithLogConfigurationWarnings(true),
		config.WithLogger(awsLogger))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to create aws token storage")
		return nil, err
	}
	return &cfg, nil
}

// NewAwsTokenStorage creates a new `TokenStorage` instance using ....
func NewAwsTokenStorage(ctx context.Context, args *AWSCliArgs) (tokenstorage.TokenStorage, error) {
	lg := log.FromContext(ctx)
	cfg, err := configFromCliArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	lg.Info("creating aws client")
	client := secretsmanager.NewFromConfig(*cfg)
	return &secretManagerTokenStorage{client: client}, nil
}

func (s *secretManagerTokenStorage) Initialize(ctx context.Context) error {
	log.FromContext(ctx).Info("AWS storage has nothing to initialize")
	s.client.
	return nil
}

func (s *secretManagerTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	log.FromContext(ctx).Info("AWS ==========> Store", "owner", owner, "token", token)
	secretName := fmt.Sprintf(awsDataPathFormat, "spi", owner.Namespace, owner.Name)
	tokenJson, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("error marshalling the state: %w", err)
	}

	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretBinary: tokenJson,
	}
	_, err = s.client.CreateSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("error saving secret: %w", err)
	}
	return nil

}

func (s *secretManagerTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	lg := log.FromContext(ctx)
	lg.Info("AWS ==========> Get")

	secretName := fmt.Sprintf(awsDataPathFormat, "spi", owner.Namespace, owner.Name)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := s.client.GetSecretValue(ctx, input)
	if err != nil {
		var awsErr smithy.APIError
		if errors.As(err, &awsErr) {
			notFoundErr := types.ResourceNotFoundException{}
			if awsErr.ErrorCode() == notFoundErr.ErrorCode() {
				lg.Info("token not found in aws storage")
				return nil, nil
			} else {
				return nil, fmt.Errorf("not able to get secret from the aws storage: %w", awsErr)
			}
		}

		return nil, fmt.Errorf("not able to get secret from the aws storage for some strange reason: %w", err)
	}

	token := api.Token{}
	if err := json.Unmarshal(result.SecretBinary, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}
	return &token, nil
}

func (s *secretManagerTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	log.FromContext(ctx).Info("AWS ==========> Delete", "owner", owner)

	secretName := fmt.Sprintf(awsDataPathFormat, "spi", owner.Namespace, owner.Name)

	input := &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretName),
	}
	_, err := s.client.DeleteSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("error deleting secret: %w", err)
	}
	return nil
}
