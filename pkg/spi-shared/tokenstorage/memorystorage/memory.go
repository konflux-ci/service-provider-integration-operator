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

package memorystorage

import (
	"context"
	"sync"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MemoryTokenStorage is an in-memory implementation of the TokenStorage interface intended to be used in tests.
type MemoryTokenStorage struct {
	// Tokens is the map of stored Tokens. The keys are object keys of the SPIAccessToken objects.
	Tokens map[client.ObjectKey]api.Token
	// ErrorOnInitialize if not nil, the error is thrown when the Initialize method is called.
	ErrorOnInitialize error
	// ErrorOnStore if not nil, the error is thrown when the Store method is called.
	ErrorOnStore error
	// ErrorOnGet if not nil, the error is thrown when the Get method is called.
	ErrorOnGet error
	// ErrorOnDelete if not nil, the error is thrown when the Delete method is called.
	ErrorOnDelete error

	lock sync.RWMutex
}

var _ tokenstorage.TokenStorage = (*MemoryTokenStorage)(nil)

func (m *MemoryTokenStorage) Initialize(_ context.Context) error {
	if m.ErrorOnInitialize != nil {
		return m.ErrorOnInitialize
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.Tokens = map[client.ObjectKey]api.Token{}
	return nil
}

func (m *MemoryTokenStorage) Store(_ context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if m.ErrorOnStore != nil {
		return m.ErrorOnStore
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	m.Tokens[client.ObjectKeyFromObject(owner)] = *token
	return nil
}

func (m *MemoryTokenStorage) Get(_ context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	if m.ErrorOnGet != nil {
		return nil, m.ErrorOnGet
	}

	m.lock.RLock()
	defer m.lock.RUnlock()

	m.ensureTokens()

	token, ok := m.Tokens[client.ObjectKeyFromObject(owner)]
	if !ok {
		return nil, nil
	}

	return &token, nil
}

func (m *MemoryTokenStorage) Delete(_ context.Context, owner *api.SPIAccessToken) error {
	if m.ErrorOnDelete != nil {
		return m.ErrorOnDelete
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	delete(m.Tokens, client.ObjectKeyFromObject(owner))
	return nil
}

func (m *MemoryTokenStorage) ensureTokens() {
	if m.Tokens == nil {
		m.Tokens = map[client.ObjectKey]api.Token{}
	}
}
