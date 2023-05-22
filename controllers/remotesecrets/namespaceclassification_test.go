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

package remotesecrets

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestClassifyWithNoPriorState(t *testing.T) {
	rs := &api.RemoteSecret{
		Spec: api.RemoteSecretSpec{
			Targets: []api.RemoteSecretTarget{
				{
					Namespace: "ns_a",
				},
				{
					Namespace: "ns_b",
				},
			},
		},
	}

	nc := ClassifyTargetNamespaces(rs)

	assert.Len(t, nc.Remove, 0)
	assert.Len(t, nc.Sync, 2)
	assert.Equal(t, StatusTargetIndex(-1), nc.Sync[SpecTargetIndex(0)])
	assert.Equal(t, StatusTargetIndex(-1), nc.Sync[SpecTargetIndex(1)])
}

func TestClassifyReordered(t *testing.T) {
	rs := &api.RemoteSecret{
		Spec: api.RemoteSecretSpec{
			Targets: []api.RemoteSecretTarget{
				{
					Namespace: "ns_a",
				},
				{
					Namespace: "ns_b",
				},
				{
					Namespace: "ns_c",
				},
			},
		},
		Status: api.RemoteSecretStatus{
			Targets: []api.TargetStatus{
				{
					Namespace:           "ns_b",
					SecretName:          "sec3",
					ServiceAccountNames: []string{},
				},
				{
					Namespace:           "ns_c",
					SecretName:          "sec2",
					ServiceAccountNames: []string{},
				},
				{
					Namespace:           "ns_a",
					SecretName:          "sec1",
					ServiceAccountNames: []string{},
				},
			},
		},
	}

	nc := ClassifyTargetNamespaces(rs)

	assert.Len(t, nc.Remove, 0)
	assert.Len(t, nc.Sync, 3)
	assert.Equal(t, StatusTargetIndex(2), nc.Sync[SpecTargetIndex(0)])
	assert.Equal(t, StatusTargetIndex(0), nc.Sync[SpecTargetIndex(1)])
	assert.Equal(t, StatusTargetIndex(1), nc.Sync[SpecTargetIndex(2)])
}

func TestClassifyWithSomeMissingFromStatus(t *testing.T) {
	rs := &api.RemoteSecret{
		Spec: api.RemoteSecretSpec{
			Targets: []api.RemoteSecretTarget{
				{
					Namespace: "ns_a",
				},
				{
					Namespace: "ns_b",
				},
			},
		},
		Status: api.RemoteSecretStatus{
			Targets: []api.TargetStatus{
				{
					Namespace:           "ns_b",
					SecretName:          "sec",
					ServiceAccountNames: []string{"sa_a", "sa_b"},
				},
			},
		},
	}

	nc := ClassifyTargetNamespaces(rs)

	assert.Len(t, nc.Remove, 0)
	assert.Len(t, nc.Sync, 2)
	assert.Equal(t, StatusTargetIndex(-1), nc.Sync[SpecTargetIndex(0)])
	assert.Equal(t, StatusTargetIndex(0), nc.Sync[SpecTargetIndex(1)])
}

func TestClassifyWithSomeMoreInStatus(t *testing.T) {
	rs := &api.RemoteSecret{
		Spec: api.RemoteSecretSpec{
			Targets: []api.RemoteSecretTarget{
				{
					Namespace: "ns_a",
				},
			},
		},
		Status: api.RemoteSecretStatus{
			Targets: []api.TargetStatus{
				{
					Namespace:           "ns_b",
					SecretName:          "sec",
					ServiceAccountNames: []string{"sa_a", "sa_b"},
				},
				{
					Namespace:           "ns_a",
					SecretName:          "sec",
					ServiceAccountNames: []string{"sa_a", "sa_b"},
				},
			},
		},
	}

	nc := ClassifyTargetNamespaces(rs)

	assert.Len(t, nc.Remove, 1)
	assert.Len(t, nc.Sync, 1)
	assert.Equal(t, StatusTargetIndex(1), nc.Sync[SpecTargetIndex(0)])
	assert.Equal(t, StatusTargetIndex(0), nc.Remove[0])
}
