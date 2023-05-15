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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteSecretValidate(t *testing.T) {
	t.Run("accepts single target", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
				},
			},
		}

		assert.NoError(t, rs.Validate())
	})

	t.Run("accepts multiple targets in differents namespaces", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
					{
						Namespace: "ns-2",
					},
					{
						Namespace: "ns-3",
					},
				},
			},
		}

		assert.NoError(t, rs.Validate())
	})

	t.Run("rejects multiple targets in single namespace", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
					{
						Namespace: "ns-1",
					},
				},
			},
		}

		err := rs.Validate()

		assert.Error(t, err)
		assert.Equal(t, "multiple targets referencing the same namespace is not allowed: targets on indices 0 and 1 point to the same namespace ns-1", err.Error())
	})

}
