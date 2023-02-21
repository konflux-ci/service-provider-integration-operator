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
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var testToken = &v1beta1.Token{
	Username:     "testUsername",
	AccessToken:  "testAccessToken",
	TokenType:    "testTokenType",
	RefreshToken: "testRefreshToken",
	Expiry:       123,
}

var testSpiAccessToken = &v1beta1.SPIAccessToken{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "testSpiAccessToken",
		Namespace: "testNamespace",
	},
}

func TestInitialize(t *testing.T) {
	ctx := context.TODO()
	awsConfig, _ := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigFiles([]string{"nothing"}),
		config.WithSharedCredentialsFiles([]string{"nothing"}))
	strg := AwsTokenStorage{
		Config: &awsConfig,
	}

	errInit := strg.Initialize(ctx)

	assert.Error(t, errInit)
}

func TestGenerateSecretName(t *testing.T) {
	secretName := generateAwsSecretName(&v1beta1.SPIAccessToken{ObjectMeta: metav1.ObjectMeta{Namespace: "tokennamespace", Name: "tokenname"}})

	assert.NotNil(t, secretName)
	assert.Contains(t, *secretName, "tokennamespace")
	assert.Contains(t, *secretName, "tokenname")
}

func TestCheckCredentials(t *testing.T) {
	ctx := context.TODO()
	t.Run("ok check", func(t *testing.T) {
		cl := &mockAwsClient{
			listFn: func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
				return nil, nil
			},
		}
		strg := AwsTokenStorage{
			client: cl,
			lg:     log.FromContext(ctx),
		}
		assert.NoError(t, strg.checkCredentials(ctx))
	})

	t.Run("failed check", func(t *testing.T) {
		ctx := context.TODO()
		cl := &mockAwsClient{
			listFn: func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
				return nil, fmt.Errorf("fail")
			},
		}
		strg := AwsTokenStorage{
			client: cl,
			lg:     log.FromContext(ctx),
		}
		assert.Error(t, strg.checkCredentials(ctx))
		assert.True(t, cl.listCalled)
	})
}

func TestStore(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := context.TODO()
		cl := &mockAwsClient{
			createFn: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     log.FromContext(ctx),
		}

		errStore := strg.Store(ctx, testSpiAccessToken, testToken)
		assert.NoError(t, errStore)
		assert.True(t, cl.createCalled)
		assert.False(t, cl.updateCalled)
	})

	t.Run("fail create", func(t *testing.T) {
		ctx := context.TODO()
		cl := &mockAwsClient{
			createFn: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, fmt.Errorf("failed to create")
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     log.FromContext(ctx),
		}

		errStore := strg.Store(ctx, testSpiAccessToken, testToken)
		assert.Error(t, errStore)
		assert.True(t, cl.createCalled)
		assert.False(t, cl.updateCalled)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			createFn: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, &types.ResourceExistsException{}
			},
			updateFn: func(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				return nil, nil
			},
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{ARN: aws.String("awssecretid")}, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		errStore := strg.Store(ctx, testSpiAccessToken, testToken)
		assert.NoError(t, errStore)
		assert.True(t, cl.createCalled)
		assert.True(t, cl.updateCalled)
		assert.True(t, cl.getCalled)
	})

	t.Run("fail", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			createFn: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, &types.ResourceExistsException{}
			},
			updateFn: func(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
				return nil, fmt.Errorf("update failed")
			},
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{ARN: aws.String("awssecretid")}, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		errStore := strg.Store(ctx, testSpiAccessToken, testToken)
		assert.Error(t, errStore)
		assert.True(t, cl.createCalled)
		assert.True(t, cl.updateCalled)
		assert.True(t, cl.getCalled)
	})

	t.Run("fail get to update", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			createFn: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return nil, &types.ResourceExistsException{}
			},
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, fmt.Errorf("fail to get")
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		errStore := strg.Store(ctx, testSpiAccessToken, testToken)
		assert.Error(t, errStore)
		assert.True(t, cl.createCalled)
		assert.False(t, cl.updateCalled)
		assert.True(t, cl.getCalled)
	})
}

func TestGet(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				tokenData, _ := json.Marshal(testToken)
				return &secretsmanager.GetSecretValueOutput{ARN: aws.String("awssecretid"), SecretBinary: tokenData}, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.NoError(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Equal(t, testToken, token)
	})

	t.Run("corrupted data", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{ARN: aws.String("awssecretid"), SecretBinary: []byte("blabol")}, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.Error(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})

	t.Run("got nil secret", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.Error(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})

	t.Run("fail to get", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, fmt.Errorf("fail to get")
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.Error(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})

	t.Run("token not found", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, &types.ResourceNotFoundException{}
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.NoError(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})

	t.Run("secret is deleting", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, &types.InvalidRequestException{Message: aws.String("token is scheduled for deletion")}
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.NoError(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})

	t.Run("fail invalid request", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			getFn: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, &types.InvalidRequestException{Message: aws.String("some failure")}
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		token, errStore := strg.Get(ctx, testSpiAccessToken)
		assert.Error(t, errStore)
		assert.True(t, cl.getCalled)
		assert.Nil(t, token)
	})
}

func TestDelete(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			deleteFn: func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
				return nil, nil
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		errDelete := strg.Delete(ctx, testSpiAccessToken)
		assert.NoError(t, errDelete)
		assert.True(t, cl.deleteCalled)
	})

	t.Run("fail", func(t *testing.T) {
		ctx := context.TODO()
		lg := log.FromContext(ctx)

		cl := &mockAwsClient{
			deleteFn: func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
				return nil, fmt.Errorf("failed to delete")
			},
		}

		strg := AwsTokenStorage{
			client: cl,
			lg:     lg,
		}

		errDelete := strg.Delete(ctx, testSpiAccessToken)
		assert.Error(t, errDelete)
		assert.True(t, cl.deleteCalled)
	})
}

type mockAwsClient struct {
	createFn     func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	createCalled bool

	getFn     func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	getCalled bool

	listFn     func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	listCalled bool

	updateFn     func(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error)
	updateCalled bool

	deleteFn     func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
	deleteCalled bool
}

func (c *mockAwsClient) CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	c.createCalled = true
	return c.createFn(ctx, params, optFns...)
}
func (c *mockAwsClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	c.getCalled = true
	return c.getFn(ctx, params, optFns...)
}
func (c *mockAwsClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	c.listCalled = true
	return c.listFn(ctx, params, optFns...)
}
func (c *mockAwsClient) UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	c.updateCalled = true
	return c.updateFn(ctx, params, optFns...)
}
func (c *mockAwsClient) DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	c.deleteCalled = true
	return c.deleteFn(ctx, params, optFns...)
}
