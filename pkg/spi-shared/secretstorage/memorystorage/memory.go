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
	"fmt"
	"sync"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
)

type MemoryStorage struct {
	// Data is the map of stored data.
	Data map[secretstorage.SecretID][]byte
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

// Delete implements secretstorage.SecretStorage
func (m *MemoryStorage) Delete(ctx context.Context, id secretstorage.SecretID) error {
	if m.ErrorOnDelete != nil {
		return m.ErrorOnDelete
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	delete(m.Data, id)
	return nil
}

// Get implements secretstorage.SecretStorage
func (m *MemoryStorage) Get(ctx context.Context, id secretstorage.SecretID) ([]byte, error) {
	if m.ErrorOnGet != nil {
		return nil, m.ErrorOnGet
	}

	m.lock.RLock()
	defer m.lock.RUnlock()

	m.ensureTokens()

	data, ok := m.Data[id]
	if !ok {
		return nil, fmt.Errorf("%w", secretstorage.NotFoundError)
	}

	return data, nil
}

// Initialize implements secretstorage.SecretStorage
func (m *MemoryStorage) Initialize(ctx context.Context) error {
	if m.ErrorOnInitialize != nil {
		return m.ErrorOnInitialize
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.Data = map[secretstorage.SecretID][]byte{}
	return nil
}

// Store implements secretstorage.SecretStorage
func (m *MemoryStorage) Store(ctx context.Context, id secretstorage.SecretID, data []byte) error {
	if m.ErrorOnStore != nil {
		return m.ErrorOnStore
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	m.Data[id] = data
	return nil
}

func (m *MemoryStorage) ensureTokens() {
	if m.Data == nil {
		m.Data = map[secretstorage.SecretID][]byte{}
	}
}

var _ secretstorage.SecretStorage = (*MemoryStorage)(nil)
