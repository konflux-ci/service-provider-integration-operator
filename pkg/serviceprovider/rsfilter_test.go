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

package serviceprovider

import (
	"context"
	"testing"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultRemoteSecretFilterFunc(t *testing.T) {
	remoteSecret := v1beta1.RemoteSecret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs",
			Namespace: "ns",
		},
		Spec: v1beta1.RemoteSecretSpec{
			Secret: v1beta1.LinkableSecretSpec{
				Name: "secret",
				Type: corev1.SecretTypeBasicAuth,
			},
			Targets: []v1beta1.RemoteSecretTarget{{
				Namespace: "ns",
			}},
		},
		Status: v1beta1.RemoteSecretStatus{
			Conditions: []metav1.Condition{{
				Type:   string(v1beta1.RemoteSecretConditionTypeDataObtained),
				Status: metav1.ConditionTrue,
			}},
			Targets: []v1beta1.TargetStatus{{
				Namespace:  "ns",
				SecretName: "secret",
			}},
		},
	}
	accessCheck := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "ns",
		},
	}

	t.Run("all conditions satisfied", func(t *testing.T) {
		assert.True(t, DefaultRemoteSecretFilterFunc(context.TODO(), &accessCheck, &remoteSecret))
	})

	t.Run("data not obtained", func(t *testing.T) {
		rs := remoteSecret.DeepCopy()
		rs.Status.Conditions[0].Status = metav1.ConditionFalse
		assert.False(t, DefaultRemoteSecretFilterFunc(context.TODO(), &accessCheck, rs))
	})

	t.Run("wrong secret type", func(t *testing.T) {
		rs := remoteSecret.DeepCopy()
		rs.Spec.Secret.Type = corev1.SecretTypeOpaque
		assert.False(t, DefaultRemoteSecretFilterFunc(context.TODO(), &accessCheck, rs))
	})

	t.Run("target in different namespace", func(t *testing.T) {
		rs := remoteSecret.DeepCopy()
		rs.Status.Targets[0].Namespace = "diff-ns"
		assert.False(t, DefaultRemoteSecretFilterFunc(context.TODO(), &accessCheck, rs))
	})
}
