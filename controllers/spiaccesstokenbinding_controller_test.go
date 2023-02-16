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
	"context"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateServiceProviderUrl(t *testing.T) {
	assert.ErrorIs(t, validateServiceProviderUrl("https://foo."), invalidServiceProviderHostError)
	assert.ErrorIs(t, validateServiceProviderUrl("https://Ca$$h.com"), invalidServiceProviderHostError)

	assert.ErrorContains(t, validateServiceProviderUrl("://invalid"), "not parsable")
	assert.ErrorContains(t, validateServiceProviderUrl("https://rick:mory"), "not parsable")

	assert.NoError(t, validateServiceProviderUrl(config.ServiceProviderTypeGitHub.DefaultBaseUrl))
	assert.NoError(t, validateServiceProviderUrl(config.ServiceProviderTypeQuay.DefaultBaseUrl))
	assert.NoError(t, validateServiceProviderUrl("http://random.ogre"))
}

func TestAssureProperRepoUrl(t *testing.T) {
	t.Run("keeps scheme", func(t *testing.T) {
		oldUrl := "scheme://url"
		newUrl, err := assureProperRepoUrl(oldUrl)
		assert.NoError(t, err)
		assert.Equal(t, oldUrl, newUrl)
	})

	t.Run("adds https", func(t *testing.T) {
		oldUrl := "url.without.scheme"
		newUrl, err := assureProperRepoUrl(oldUrl)
		assert.NoError(t, err)
		assert.Equal(t, "https://"+oldUrl, newUrl)
	})

	t.Run("bad url", func(t *testing.T) {
		oldUrl := "::/bad.url"
		_, err := assureProperRepoUrl(oldUrl)
		assert.Error(t, err)
	})
}

func TestAssureProperValuesInBinding(t *testing.T) {
	binding := api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
	}
	r := SPIAccessTokenBindingReconciler{
		Client: mockK8sClient(&binding),
	}

	t.Run("url changed", func(t *testing.T) {
		binding.Spec.RepoUrl = "url.without.scheme"
		changed, result, err := r.assureProperValuesInBinding(context.TODO(), &binding)

		assert.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, ctrl.Result{Requeue: true}, result)

		assert.Equal(t, "https://url.without.scheme", binding.Spec.RepoUrl)
		assert.Empty(t, binding.Status.ErrorReason)
		assert.Empty(t, binding.Status.ErrorMessage)
	})

	t.Run("no change", func(t *testing.T) {
		binding.Spec.RepoUrl = "http://url.with.scheme"
		changed, result, err := r.assureProperValuesInBinding(context.TODO(), &binding)

		assert.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, ctrl.Result{}, result)

		assert.Equal(t, "http://url.with.scheme", binding.Spec.RepoUrl)
		assert.Empty(t, binding.Status.ErrorReason)
		assert.Empty(t, binding.Status.ErrorMessage)
	})

	t.Run("error", func(t *testing.T) {
		binding.Spec.RepoUrl = ":://bad.url"
		changed, result, err := r.assureProperValuesInBinding(context.TODO(), &binding)

		assert.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, ctrl.Result{}, result)

		assert.NotEmpty(t, binding.Status.ErrorReason)
		assert.NotEmpty(t, binding.Status.ErrorMessage)
	})
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
