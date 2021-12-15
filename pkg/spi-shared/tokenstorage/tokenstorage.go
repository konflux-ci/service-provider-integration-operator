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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TokenStorage is a simple interface on top of Kubernetes client to perform CRUD operations on the tokens. This is done
// so that we can provide either secret-based or Vault-based implementation.
type TokenStorage interface {
	Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error)
	Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error)
	GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error)
	Delete(ctx context.Context, owner *api.SPIAccessToken) error
}

// NewFromConfig creates a new `TokenStorage` instance based on the provided configuration.
func NewFromConfig(cfg *config.Configuration) (TokenStorage, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	cl, err := cfg.KubernetesClient(client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return New(cl)
}

// New creates a new `TokenStorage` instance using the provided Kubernetes client.
func New(cl client.Client) (TokenStorage, error) {
	return &tokenStorage{}, nil
}

type tokenStorage struct {
	// let's fake it for now
	_s map[client.ObjectKey]api.Token
}

var _ TokenStorage = (*tokenStorage)(nil)

func (v *tokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error) {
	v.storage()[client.ObjectKeyFromObject(owner)] = *token
	return v.GetDataLocation(ctx, owner)
}

func (v *tokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	key := client.ObjectKeyFromObject(owner)
	val, ok := v.storage()[key]
	if !ok {
		return nil, nil
	}
	return val.DeepCopy(), nil
}

func (v *tokenStorage) GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error) {
	return "/spi/" + owner.GetNamespace() + "/" + owner.GetName(), nil
}

func (v *tokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	delete(v.storage(), client.ObjectKeyFromObject(owner))
	return nil
}

func (v *tokenStorage) storage() map[client.ObjectKey]api.Token {
	if v._s == nil {
		v._s = map[client.ObjectKey]api.Token{}
	}

	return v._s
}
