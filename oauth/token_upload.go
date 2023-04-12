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

package oauth

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TokenUploader is used to permanently persist credentials for the given token.
type TokenUploader interface {
	Upload(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error
}

// UploadFunc used to provide anonymous implementation of TokenUploader.
// Example:
//
//	 uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
//			return fmt.Errorf("failed to store the token data into storage")
//		})
type UploadFunc func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error

func (u UploadFunc) Upload(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
	return u(ctx, tokenObjectName, tokenObjectNamespace, data)
}

// This variable is a guard to ensure that UploadFunc actually satisfies the TokenUploader interface
var _ TokenUploader = (UploadFunc)(nil)

type SpiTokenUploader struct {
	ClientFactory kubernetesclient.K8sClientFactory
	Storage       tokenstorage.TokenStorage
}

func (u *SpiTokenUploader) Upload(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
	AuditLogWithTokenInfo(ctx, "manual token upload initiated", tokenObjectNamespace, tokenObjectName, "action", "UPDATE")
	token := &api.SPIAccessToken{}
	ctx = clientfactory.NamespaceIntoContext(ctx, tokenObjectNamespace)
	cl, err := u.ClientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create K8S client for namespace %s: %w", tokenObjectNamespace, err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Name: tokenObjectName, Namespace: tokenObjectNamespace}, token); err != nil {
		return fmt.Errorf("failed to get SPIAccessToken object %s/%s: %w", tokenObjectNamespace, tokenObjectName, err)
	}

	if err := u.Storage.Store(ctx, token, data); err != nil {
		return fmt.Errorf("failed to store the token data into storage: %w", err)
	}
	AuditLogWithTokenInfo(ctx, "manual token upload done", tokenObjectNamespace, tokenObjectName)
	return nil
}
