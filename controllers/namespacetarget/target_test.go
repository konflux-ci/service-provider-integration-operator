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

package namespacetarget

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceTarget_GetActualSecretName(t *testing.T) {
	bt := getTestNamespaceTarget()
	assert.Equal(t, "kachny-asdf", bt.GetActualSecretName())
}

func TestNamespaceTarget_GetActualServiceAccountNames(t *testing.T) {
	bt := getTestNamespaceTarget()

	assert.Equal(t, []string{"a", "b"}, bt.GetActualServiceAccountNames())
}

func TestNamespaceTarget_GetClient(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	bt := getTestNamespaceTarget()
	bt.Client = cl

	assert.Same(t, cl, bt.GetClient())
}

func TestNamespaceTarget_GetTargetObjectKey(t *testing.T) {
	bt := getTestNamespaceTarget()
	assert.Equal(t, client.ObjectKey{Name: "remotesecret", Namespace: "ns"}, bt.GetTargetObjectKey())
}

func TestNamespaceTarget_GetSpec(t *testing.T) {
	bt := getTestNamespaceTarget()

	assert.Equal(t, api.LinkableSecretSpec{
		GenerateName: "kachny-",
	}, bt.GetSpec())
}

func TestNamespaceTarget_GetTargetNamespace(t *testing.T) {
	bt := getTestNamespaceTarget()
	assert.Equal(t, "target-ns", bt.GetTargetNamespace())
}

func TestNamespaceTarget_GetType(t *testing.T) {
	assert.Equal(t, "Namespace", (&NamespaceTarget{}).GetType())
}

func getTestNamespaceTarget() NamespaceTarget {
	rs := getTestRemoteSecret()
	return NamespaceTarget{
		Client:       nil,
		TargetKey:    client.ObjectKeyFromObject(rs),
		SecretSpec:   &rs.Spec.Secret,
		TargetSpec:   &rs.Spec.Targets[0],
		TargetStatus: &rs.Status.Targets[0],
	}
}

func getTestRemoteSecret() *api.RemoteSecret {
	return &api.RemoteSecret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "remotesecret",
			Namespace: "ns",
		},
		Spec: api.RemoteSecretSpec{
			Secret: api.LinkableSecretSpec{
				GenerateName: "kachny-",
			},
			Targets: []api.RemoteSecretTarget{
				{
					Namespace: "target-ns",
				},
			},
		},
		Status: api.RemoteSecretStatus{
			Targets: []api.TargetStatus{
				{
					Namespace:           "target-ns",
					SecretName:          "kachny-asdf",
					ServiceAccountNames: []string{"a", "b"},
				},
			},
		},
	}
}
