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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AwsTokenStorage struct {
	Config *aws.Config

	client *secretsmanager.Client
	lg     logr.Logger
}

const awsDataPathFormat = "%s/%s"

var _ tokenstorage.TokenStorage = (*AwsTokenStorage)(nil)

// NewAwsTokenStorage creates a new `TokenStorage` instance using ....

func (s *AwsTokenStorage) Initialize(ctx context.Context) error {
	s.lg = log.FromContext(ctx, "tokenstorage", "AWS")
	s.lg.Info("initializing AWS token storage")

	//TODO: fail if something missing here
	s.client = secretsmanager.NewFromConfig(*s.Config)

	return nil
}

func (s *AwsTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	s.lg.V(logs.DebugLevel).Info("storing the token", "SPIAccessToken", owner)

	secretName := fmt.Sprintf(awsDataPathFormat, owner.Namespace, owner.Name)
	tokenData, errMarshal := json.Marshal(token)
	if errMarshal != nil {
		return fmt.Errorf("error marshalling the state: %w", errMarshal)
	}

	errCreate := s.createOrUpdate(ctx, aws.String(secretName), tokenData)
	if errCreate != nil {
		return fmt.Errorf("failed to create the secret: %w", errCreate)
	}
	return nil
}

func (s *AwsTokenStorage) createOrUpdate(ctx context.Context, name *string, data []byte) error {
	s.lg.V(logs.DebugLevel).Info("creating the AWS secret", "secretname", name)

	createInput := &secretsmanager.CreateSecretInput{
		Name:         name,
		SecretBinary: data,
	}

	_, errCreate := s.client.CreateSecret(ctx, createInput)
	if errCreate != nil {
		var awsError smithy.APIError
		if errors.As(errCreate, &awsError) {
			// if secret with same name already exists in AWS, we try to update it
			if errAlreadyExists, ok := awsError.(*types.ResourceExistsException); ok {
				s.lg.V(logs.DebugLevel).Info("AWS secret already exists, trying to update", "secretname", name)
				updateErr := s.update(ctx, createInput.Name, createInput.SecretBinary)
				if updateErr != nil {
					return fmt.Errorf("failed to update the secret: %w", errAlreadyExists)
				}
				return nil
			}
		}
		return fmt.Errorf("error creating the secret: %w", errCreate)
	}

	return nil
}

func (s *AwsTokenStorage) update(ctx context.Context, name *string, data []byte) error {
	s.lg.V(logs.DebugLevel).Info("updating the AWS secret", "secretname", name)

	awsSecret, errGet := s.getSecret(ctx, *name)
	if errGet != nil {
		return fmt.Errorf("failed to get the secret '%s' to update it in aws secretmanager: %w", *name, errGet)
	}

	updateInput := &secretsmanager.UpdateSecretInput{SecretId: awsSecret.ARN, SecretBinary: data}
	_, errUpdate := s.client.UpdateSecret(ctx, updateInput)
	if errUpdate != nil {
		return fmt.Errorf("failed to update the secret '%s' in aws secretmanager: %w", *name, errUpdate)
	}
	return nil
}

func (s *AwsTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	s.lg.V(logs.DebugLevel).Info("getting the token", "SPIAccessToken", owner)

	secretName := fmt.Sprintf(awsDataPathFormat, owner.Namespace, owner.Name)
	result, err := s.getSecret(ctx, secretName)

	if err != nil {
		var awsError smithy.APIError
		if errors.As(err, &awsError) {
			if notFoundErr, ok := awsError.(*types.ResourceNotFoundException); ok {
				s.lg.V(logs.DebugLevel).Info("token not found in aws storage", "err", notFoundErr.ErrorMessage())
				return nil, nil
			}

			if invalidRequestErr, ok := awsError.(*types.InvalidRequestException); ok {
				if strings.Contains(invalidRequestErr.ErrorMessage(), "deletion") {
					s.lg.V(logs.DebugLevel).Info("tried to get aws secret that is marked for deletion. This is not error on our side.")
					return nil, nil
				} else {
					return nil, fmt.Errorf("not able to get secret from the aws storage: [%s] %s", invalidRequestErr.ErrorCode(), invalidRequestErr.ErrorMessage())
				}
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

func (s *AwsTokenStorage) getSecret(ctx context.Context, secretName string) (*secretsmanager.GetSecretValueOutput, error) {
	s.lg.V(logs.DebugLevel).Info("getting AWS secret", "secretname", secretName)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	awsSecret, err := s.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get the secret '%s' from aws secretmanager: %w", secretName, err)
	}
	return awsSecret, nil
}

func (s *AwsTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	s.lg.V(logs.DebugLevel).Info("deleting the token", "SPIAccessToken", owner)

	secretName := fmt.Sprintf(awsDataPathFormat, owner.Namespace, owner.Name)
	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	}

	//TODO: check delete result
	_, err := s.client.DeleteSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("error deleting secret: %w", err)
	}
	return nil
}
