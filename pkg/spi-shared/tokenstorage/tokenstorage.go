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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// TokenStorage is a simple interface on top of Kubernetes client to perform CRUD operations on the tokens. This is done
// so that we can provide either secret-based or Vault-based implementation.
type TokenStorage interface {
	Initialize(ctx context.Context) error
	Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error
	Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error)
	Delete(ctx context.Context, owner *api.SPIAccessToken) error
}
