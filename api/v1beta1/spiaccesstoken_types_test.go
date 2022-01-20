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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissions(t *testing.T) {
	var pt PermissionType

	pt = PermissionTypeRead
	assert.True(t, pt.IsRead())
	assert.False(t, pt.IsWrite())

	pt = PermissionTypeReadWrite
	assert.True(t, pt.IsRead())
	assert.True(t, pt.IsWrite())

	pt = PermissionTypeWrite
	assert.False(t, pt.IsRead())
	assert.True(t, pt.IsWrite())
}

func TestEnsureLabels(t *testing.T) {
	t.Run("sets the predefined", func (t *testing.T) {
		at := SPIAccessToken{
			Spec: SPIAccessTokenSpec{
				ServiceProviderType: ServiceProviderType("sp_type"),
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, at.EnsureLabels())
		assert.Equal(t, "sp_type", at.Labels[ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[ServiceProviderHostLabel])
	})

	t.Run("doesn't overwrite existing", func (t *testing.T) {
		at := SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"a": "av",
					"b": "bv",
					ServiceProviderHostLabel: "orig-host",
				},
			},
			Spec: SPIAccessTokenSpec{
				ServiceProviderType: ServiceProviderType("sp_type"),
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, at.EnsureLabels())
		assert.Equal(t, "sp_type", at.Labels[ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[ServiceProviderHostLabel])
		assert.Equal(t, "av", at.Labels["a"])
		assert.Equal(t, "bv", at.Labels["b"])
	})
}
