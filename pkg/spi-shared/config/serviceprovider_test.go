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

package config

import (
	"context"
	"net/url"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSpConfigFromUserSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	ctx := context.TODO()

	secretNamespace := "test-secretConfigNamespace"

	t.Run("find valid secret", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						},
					},
				},
			},
		}).Build()

		repoUrl, _ := url.Parse(ServiceProviderTypeGitHub.DefaultBaseUrl + "/neco/cosi")
		spConfig, err := SpConfigFromUserSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, repoUrl)

		assert.NotNil(t, spConfig)
		assert.NoError(t, err)
	})

	t.Run("no secret returs nil, but no err", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{},
		}).Build()

		repoUrl, _ := url.Parse(ServiceProviderTypeGitHub.DefaultBaseUrl + "/neco/cosi")
		spConfig, err := SpConfigFromUserSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, repoUrl)

		assert.Nil(t, spConfig)
		assert.NoError(t, err)
	})

	t.Run("kube error results in error", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()

		repoUrl, _ := url.Parse(ServiceProviderTypeGitHub.DefaultBaseUrl + "/neco/cosi")
		spConfig, err := SpConfigFromUserSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, repoUrl)

		assert.Nil(t, spConfig)
		assert.Error(t, err)
	})

}

func TestSpConfigFromGlobalConfig(t *testing.T) {
	t.Run("finds configuration when set and host matches", func(t *testing.T) {
		globalConfig := &SharedConfiguration{
			ServiceProviders: []ServiceProviderConfiguration{
				{
					ServiceProviderType:    ServiceProviderTypeQuay,
					ServiceProviderBaseUrl: ServiceProviderTypeQuay.DefaultBaseUrl,
				},
				{
					ServiceProviderType:    ServiceProviderTypeGitHub,
					ServiceProviderBaseUrl: "blab.ol",
				},
			},
		}

		spConfig := SpConfigFromGlobalConfig(globalConfig, ServiceProviderTypeGitHub, "blab.ol")

		assert.NotNil(t, spConfig)
		assert.Equal(t, "blab.ol", spConfig.ServiceProviderBaseUrl)
		assert.Equal(t, ServiceProviderTypeGitHub.Name, spConfig.ServiceProviderType.Name)
	})

	t.Run("returns nil when same type but different host", func(t *testing.T) {
		globalConfig := &SharedConfiguration{
			ServiceProviders: []ServiceProviderConfiguration{
				{
					ServiceProviderType:    ServiceProviderTypeQuay,
					ServiceProviderBaseUrl: ServiceProviderTypeQuay.DefaultBaseUrl,
				},
				{
					ServiceProviderType:    ServiceProviderTypeGitHub,
					ServiceProviderBaseUrl: "blab.ol",
				},
			},
		}

		spConfig := SpConfigFromGlobalConfig(globalConfig, ServiceProviderTypeGitHub, "ne.ne")

		assert.Nil(t, spConfig)
	})

	t.Run("if no config matches but find default url then return default", func(t *testing.T) {
		globalConfig := &SharedConfiguration{
			ServiceProviders: []ServiceProviderConfiguration{
				{
					ServiceProviderType:    ServiceProviderTypeQuay,
					ServiceProviderBaseUrl: ServiceProviderTypeQuay.DefaultBaseUrl,
				},
				{
					ServiceProviderType:    ServiceProviderTypeGitHub,
					ServiceProviderBaseUrl: "blab.ol",
				},
			},
		}

		spConfig := SpConfigFromGlobalConfig(globalConfig, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultBaseUrl)

		assert.NotNil(t, spConfig)
		assert.Equal(t, ServiceProviderTypeGitHub.Name, spConfig.ServiceProviderType.Name)
		assert.Equal(t, ServiceProviderTypeGitHub.DefaultBaseUrl, spConfig.ServiceProviderBaseUrl)
	})

	t.Run("if empty config but find default url then return default", func(t *testing.T) {
		globalConfig := &SharedConfiguration{}

		spConfig := SpConfigFromGlobalConfig(globalConfig, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultBaseUrl)

		assert.NotNil(t, spConfig)
		assert.Equal(t, ServiceProviderTypeGitHub.Name, spConfig.ServiceProviderType.Name)
		assert.Equal(t, ServiceProviderTypeGitHub.DefaultBaseUrl, spConfig.ServiceProviderBaseUrl)
	})

	t.Run("if empty config and find non-default url then return nil", func(t *testing.T) {
		globalConfig := &SharedConfiguration{}

		spConfig := SpConfigFromGlobalConfig(globalConfig, ServiceProviderTypeGitHub, "bla.bla")

		assert.Nil(t, spConfig)
	})
}
