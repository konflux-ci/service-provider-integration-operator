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

//go:build !release

package secretstorage

import (
	"context"
)

type TestSecretStorage struct {
	InitializeImpl func(context.Context) error
	StoreImpl      func(ctx context.Context, key SecretID, data []byte) error
	GetImpl        func(ctx context.Context, key SecretID) ([]byte, error)
	DeleteImpl     func(ctx context.Context, key SecretID) error
}

func (t TestSecretStorage) Initialize(ctx context.Context) error {
	if t.InitializeImpl == nil {
		return nil
	}

	return t.InitializeImpl(ctx)
}

func (t TestSecretStorage) Store(ctx context.Context, key SecretID, data []byte) error {
	if t.StoreImpl == nil {
		return nil
	}

	return t.StoreImpl(ctx, key, data)
}

func (t TestSecretStorage) Get(ctx context.Context, key SecretID) ([]byte, error) {
	if t.GetImpl == nil {
		return nil, nil
	}

	return t.GetImpl(ctx, key)
}

func (t TestSecretStorage) Delete(ctx context.Context, key SecretID) error {
	if t.DeleteImpl == nil {
		return nil
	}

	return t.DeleteImpl(ctx, key)
}

var _ SecretStorage = (*TestSecretStorage)(nil)
