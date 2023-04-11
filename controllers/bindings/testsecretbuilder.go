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

package bindings

import "context"

type TestSecretBuilder[K any] struct {
	GetDataImpl func(context.Context, K) (map[string][]byte, string, error)
}

var _ SecretBuilder[bool] = (*TestSecretBuilder[bool])(nil)

// GetData implements SecretBuilder
func (b *TestSecretBuilder[K]) GetData(ctx context.Context, secretDataKey K) (data map[string][]byte, errorReason string, err error) {
	if b.GetDataImpl != nil {
		return b.GetDataImpl(ctx, secretDataKey)
	}

	return map[string][]byte{}, "", nil
}
