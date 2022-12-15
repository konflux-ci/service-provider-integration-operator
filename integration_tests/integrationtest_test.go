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

package integrationtests

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

type A struct {
	A int
	B string
}

func TestToPointerArray(t *testing.T) {
	arr := []A{{}, {A: 1, B: "2"}}

	ptrs := toPointerArray(arr)

	assert.Equal(t, 0, ptrs[0].A)
	assert.Equal(t, 1, ptrs[1].A)
	assert.Equal(t, "", ptrs[0].B)
	assert.Equal(t, "2", ptrs[1].B)
}

func TestFromPointerArray(t *testing.T) {
	ptrs := []*A{{}, {A: 1, B: "2"}}

	arr := fromPointerArray(ptrs)

	assert.Equal(t, 0, arr[0].A)
	assert.Equal(t, 1, arr[1].A)
	assert.Equal(t, "", arr[0].B)
	assert.Equal(t, "2", arr[1].B)
}

func TestFindDifferences(t *testing.T) {
	ptrs1 := []*api.SPIAccessToken{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a",
				Namespace: "ns",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "b",
				Namespace: "ns",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "u",
			},
		},
	}
	ptrs2 := []*api.SPIAccessToken{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "b",
				Namespace: "ns",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a",
				Namespace: "ns",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "u",
			},
		},
	}

	diff := findDifferences(ptrs1, ptrs2)

	assert.NotEmpty(t, diff)
}
