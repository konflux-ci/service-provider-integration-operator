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
	corev1 "k8s.io/api/core/v1"
)

func TestValidate(t *testing.T) {
	validate := func(secretType corev1.SecretType) SPIAccessTokenBindingValidation {
		binding := SPIAccessTokenBinding{
			Spec: SPIAccessTokenBindingSpec{
				Secret: SecretSpec{
					Type: secretType,
					LinkedTo: []SecretLink{
						{
							ServiceAccount: ServiceAccountLink{
								As: ServiceAccountLinkTypeImagePullSecret,
							},
						},
					},
				},
			},
		}

		return binding.Validate()
	}

	t.Run(string(corev1.SecretTypeBasicAuth), func(t *testing.T) {
		res := validate(corev1.SecretTypeBasicAuth)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeBootstrapToken), func(t *testing.T) {
		res := validate(corev1.SecretTypeBootstrapToken)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeDockercfg), func(t *testing.T) {
		res := validate(corev1.SecretTypeDockercfg)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeDockerConfigJson), func(t *testing.T) {
		res := validate(corev1.SecretTypeDockerConfigJson)
		assert.Empty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeOpaque), func(t *testing.T) {
		res := validate(corev1.SecretTypeOpaque)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeServiceAccountToken), func(t *testing.T) {
		res := validate(corev1.SecretTypeServiceAccountToken)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeSSHAuth), func(t *testing.T) {
		res := validate(corev1.SecretTypeSSHAuth)
		assert.NotEmpty(t, res.Consistency)
	})
	t.Run(string(corev1.SecretTypeTLS), func(t *testing.T) {
		res := validate(corev1.SecretTypeTLS)
		assert.NotEmpty(t, res.Consistency)
	})
}
