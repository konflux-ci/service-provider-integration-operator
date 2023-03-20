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

package tokenstorage

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type TestTokenStorage struct {
	InitializeImpl func(context.Context) error
	StoreImpl      func(context.Context, *api.SPIAccessToken, *api.Token) error
	GetImpl        func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error)
	DeleteImpl     func(context.Context, *api.SPIAccessToken) error
}

func (t TestTokenStorage) Initialize(ctx context.Context) error {
	if t.InitializeImpl == nil {
		return nil
	}

	return t.InitializeImpl(ctx)
}

func (t TestTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if t.StoreImpl == nil {
		return nil
	}

	return t.StoreImpl(ctx, owner, token)
}

func (t TestTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	if t.GetImpl == nil {
		return nil, nil
	}

	return t.GetImpl(ctx, owner)
}

func (t TestTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if t.DeleteImpl == nil {
		return nil
	}

	return t.DeleteImpl(ctx, owner)
}

var _ TokenStorage = (*TestTokenStorage)(nil)
