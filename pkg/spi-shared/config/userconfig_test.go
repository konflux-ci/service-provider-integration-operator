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
	"fmt"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testClientId     = "test_client_id_123"
	testClientSecret = "test_client_secret_123"
	testAuthUrl      = "test_auth_url_123"
	testTokenUrl     = "test_token_url_123"
)

func TestCreateOauthConfigFromSecret(t *testing.T) {
	t.Run("all fields set ok", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId:     []byte(testClientId),
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
				oauthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := initializeOAuthConfigFromSecret(secret, ServiceProviderTypeGitHub)

		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, testAuthUrl, oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, testTokenUrl, oauthCfg.Endpoint.TokenURL)
	})

	t.Run("error if missing client id", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
				oauthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := initializeOAuthConfigFromSecret(secret, ServiceProviderTypeGitHub)

		assert.Nil(t, oauthCfg)
	})

	t.Run("error if missing client secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId: []byte(testClientId),
				oauthCfgSecretFieldAuthUrl:  []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl: []byte(testTokenUrl),
			},
		}

		oauthCfg := initializeOAuthConfigFromSecret(secret, ServiceProviderTypeGitHub)

		assert.Nil(t, oauthCfg)
	})

	t.Run("ok with just client id and secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId:     []byte(testClientId),
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
			},
		}

		oauthCfg := initializeOAuthConfigFromSecret(secret, ServiceProviderTypeGitHub)

		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, "", oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, "", oauthCfg.Endpoint.TokenURL)
	})
}

func TestFindOauthConfigSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	ctx := context.TODO()

	secretNamespace := "test-secretConfigNamespace"

	t.Run("no secrets", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret found", func(t *testing.T) {
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

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.True(t, found)
		assert.NotNil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret for different sp", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeQuay.Name),
						},
					},
				},
			},
		}).Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret in different namespace", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: "different-namespace",
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						},
					},
				},
			},
		}).Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("no permission for secrets", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewForbidden(schema.GroupResource{
					Group:    "test-group",
					Resource: "test-resource",
				}, "nenene", fmt.Errorf("test err"))
			}}).Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("error from kube", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, ServiceProviderTypeGitHub.DefaultHost)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.Error(t, err)
	})
}

func TestMultipleProviders(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	ctx := context.TODO()

	secretNamespace := "test-secretConfigNamespace"

	test := func(t *testing.T, oauthConfigSecrets []v1.Secret, shouldFind bool, findSecretName string, shouldError bool) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: oauthConfigSecrets,
		}).Build()

		found, secret, err := findUserServiceProviderConfigSecret(ctx, cl, secretNamespace, ServiceProviderTypeGitHub, "blabol.eh")
		assert.Equal(t, shouldFind, found, "should find the secret")
		if shouldError {
			assert.Error(t, err, "should error")
		} else {
			assert.NoError(t, err, "should not error")
		}
		if shouldFind {
			assert.NotNil(t, secret)
			assert.Equal(t, findSecretName, secret.Name)
		}
	}

	t.Run("find secret with host", func(t *testing.T) {
		test(t, []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret-hosted",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "blabol.eh",
					},
				},
			},
		}, true, "oauth-config-secret-hosted", false)
	})

	t.Run("if no secret host match, use default", func(t *testing.T) {
		test(t, []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret-hosted",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "different-host.eh",
					},
				},
			},
		}, true, "oauth-config-secret", false)
	})

	t.Run("if no secret host match or default, not-found", func(t *testing.T) {
		test(t, []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "different-host.eh",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret-hosted",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "another-different-host.eh",
					},
				},
			},
		}, false, "", false)
	})

	t.Run("multiple secrets with same host should error", func(t *testing.T) {
		test(t, []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "blabol.eh",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret-hosted",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
						v1beta1.ServiceProviderHostLabel: "blabol.eh",
					},
				},
			},
		}, false, "", true)
	})

	t.Run("multiple secrets defaults without host should fail", func(t *testing.T) {
		test(t, []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oauth-config-secret-hosted",
					Namespace: secretNamespace,
					Labels: map[string]string{
						v1beta1.ServiceProviderTypeLabel: string(ServiceProviderTypeGitHub.Name),
					},
				},
			},
		}, false, "", true)
	})
}

type mockTracker struct {
	listImpl func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error)
}

func (t *mockTracker) Add(obj runtime.Object) error {
	panic("not needed for now")
}

func (t *mockTracker) Get(gvr schema.GroupVersionResource, ns, name string) (runtime.Object, error) {
	panic("not needed for now")
}

func (t *mockTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	panic("not needed for now")
}

func (t *mockTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	panic("not needed for now")
}

func (t *mockTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
	return t.listImpl(gvr, gvk, ns)
}

func (t *mockTracker) Delete(gvr schema.GroupVersionResource, ns, name string) error {
	panic("not needed for now")
}

func (t *mockTracker) Watch(gvr schema.GroupVersionResource, ns string) (watch.Interface, error) {
	panic("not needed for now")
}
