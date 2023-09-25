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

package controllers

import (
	"testing"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRemoteSecretApplicableForEnvironment(t *testing.T) {
	rs := v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{EnvironmentLabelAndAnnotationName: "test"},
			Annotations: map[string]string{EnvironmentLabelAndAnnotationName: "prod,stage"},
		},
	}
	t.Run("has label", func(t *testing.T) {
		rs := rs
		rs.Annotations = nil
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "test"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "prod"))
	})
	t.Run("has annotation", func(t *testing.T) {
		rs := rs
		rs.Labels = nil
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "prod"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "test"))
	})
	t.Run("has both", func(t *testing.T) {
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "prod"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "test"))
	})
	t.Run("has none", func(t *testing.T) {
		assert.True(t, remoteSecretApplicableForEnvironment(v1beta1.RemoteSecret{}, "any"))
	})
}
