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

package bindingtarget

import (
	"testing"

	rapi "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBindingNamespaceTarget_GetActualSecretName(t *testing.T) {
	bt := BindingNamespaceTarget{
		Binding: getTestBinding(),
	}

	assert.Equal(t, "kachny-asdf", bt.GetActualSecretName())
}

func TestBindingNamespaceTarget_GetActualServiceAccountNames(t *testing.T) {
	bt := BindingNamespaceTarget{
		Binding: getTestBinding(),
	}

	assert.Equal(t, []string{"a", "b"}, bt.GetActualServiceAccountNames())
}

func TestBindingNamespaceTarget_GetClient(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	bt := BindingNamespaceTarget{
		Client: cl,
	}

	assert.Same(t, cl, bt.GetClient())
}

func TestBindingNamespaceTarget_GetTargetObjectKey(t *testing.T) {
	bt := BindingNamespaceTarget{
		Binding: getTestBinding(),
	}

	assert.Equal(t, client.ObjectKey{Name: "binding", Namespace: "ns"}, bt.GetTargetObjectKey())
}

func TestBindingNamespaceTarget_GetSpec(t *testing.T) {
	bt := BindingNamespaceTarget{
		Binding: getTestBinding(),
	}

	assert.Equal(t, rapi.LinkableSecretSpec{
		GenerateName: "kachny-",
	}, bt.GetSpec())
}

func TestBindingNamespaceTarget_GetTargetNamespace(t *testing.T) {
	bt := BindingNamespaceTarget{
		Binding: getTestBinding(),
	}

	assert.Equal(t, "ns", bt.GetTargetNamespace())
}

func TestBindingNamespaceTarget_GetType(t *testing.T) {
	assert.Equal(t, "Binding", (&BindingNamespaceTarget{}).GetType())
}

func getTestBinding() *api.SPIAccessTokenBinding {
	return &api.SPIAccessTokenBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "binding",
			Namespace: "ns",
		},
		Spec: api.SPIAccessTokenBindingSpec{
			Secret: api.SecretSpec{
				LinkableSecretSpec: rapi.LinkableSecretSpec{
					GenerateName: "kachny-",
				},
			},
		},
		Status: api.SPIAccessTokenBindingStatus{
			ServiceAccountNames: []string{"a", "b"},
			SyncedObjectRef: api.TargetObjectRef{
				Name: "kachny-asdf",
			},
		},
	}
}
