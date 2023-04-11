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

package oauth

import (
	"context"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	oauthstate2 "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testClientId     = "test_client_id_123"
	testClientSecret = "test_client_secret_123"
	testAuthUrl      = "test_auth_url_123"
	testTokenUrl     = "test_token_url_123"
)

func TestObtainOauthConfig(t *testing.T) {
	t.Run("no secret use default oauth config", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "eh?",
								ClientSecret: "bleh?",
								Endpoint:     github.Endpoint,
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "http://bleh.eh",
						},
					},
					BaseUrl: "baseurl",
				},
			},
		}

		oauthInfo := &oauthstate2.OAuthInfo{
			ServiceProviderUrl: "http://bleh.eh",
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthInfo)

		assert.NoError(t, err)
		assert.NotNil(t, oauthCfg)
		assert.Equal(t, "eh?", oauthCfg.ClientID)
		assert.Equal(t, "bleh?", oauthCfg.ClientSecret)
		assert.Equal(t, github.Endpoint, oauthCfg.Endpoint)
		assert.Contains(t, oauthCfg.RedirectURL, "baseurl")
	})

	t.Run("error if no secret and no oauth in default", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "http://bleh.eh",
						},
					},
					BaseUrl: "baseurl",
				},
			},
		}

		oauthInfo := &oauthstate2.OAuthInfo{
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
			ServiceProviderUrl:  "http://bleh.eh",
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthInfo)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("use oauth config from secret", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub.Name),
							v1beta1.ServiceProviderHostLabel: "bleh.eh",
						},
					},
					Data: map[string][]byte{
						"clientId":     []byte("testclientid"),
						"clientSecret": []byte("testclientsecret"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "eh?",
								ClientSecret: "bleh?",
								Endpoint:     github.Endpoint,
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "http://bleh.eh",
						},
					},
					BaseUrl: "baseurl",
				},
			},
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
			ServiceProviderUrl:  "http://bleh.eh",
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)
		assert.NoError(t, err)
		assert.NotNil(t, oauthCfg)
		assert.Equal(t, "testclientid", oauthCfg.ClientID)
		assert.Equal(t, "testclientsecret", oauthCfg.ClientSecret)
		assert.Equal(t, config.ServiceProviderTypeGitHub.DefaultOAuthEndpoint, oauthCfg.Endpoint)
		assert.Contains(t, oauthCfg.RedirectURL, "baseurl")
	})

	t.Run("found invalid oauth config secret", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub.Name),
						},
					},
					Data: map[string][]byte{
						"clientId": []byte("testclientid"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "eh?",
								ClientSecret: "bleh?",
								Endpoint:     github.Endpoint,
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "http://bleh.eh",
						},
					},
					BaseUrl: "baseurl",
				},
			},
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("fail when no oauth in config secret", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub.Name),
						},
					},
					Data: map[string][]byte{},
				},
			},
		}).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "eh?",
								ClientSecret: "bleh?",
								Endpoint:     github.Endpoint,
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: config.ServiceProviderTypeGitHub.DefaultBaseUrl,
						},
					},
					BaseUrl: "baseurl",
				},
			},
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
			ServiceProviderUrl:  config.ServiceProviderTypeGitHub.DefaultBaseUrl,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("error when failed kube request", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()

		ctrl := commonController{
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("invalid url fails", func(t *testing.T) {
		ctrl := commonController{}

		oauthInfo := &oauthstate2.OAuthInfo{
			ServiceProviderUrl: ":::",
		}

		oauthCfg, err := ctrl.obtainOauthConfig(context.TODO(), oauthInfo)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("found nothing returns error", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl := commonController{
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{},
					BaseUrl:          "baseurl",
				},
			},
		}

		oauthInfo := &oauthstate2.OAuthInfo{
			ServiceProviderUrl:  "http://bleh.eh",
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthInfo)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("no auth in list secret request context", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))

		ctx := clientfactory.WithAuthIntoContext("token", context.TODO())

		secretNamespace := "test-secretConfigNamespace"
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()
		testC := newTestClient(cl)
		testC.inspectList = func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
			assert.Empty(t, ctx.Value(httptransport.AuthenticatingRoundTripperContextKey))
		}

		ctrl := commonController{
			ClientFactory:       kubernetesclient.SingleInstanceClientFactory{Client: cl},
			InClusterK8sClient:  cl,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})
}

type testClient struct {
	client.Reader
	client.Writer
	client.StatusClient

	client client.Client

	inspectList func(ctx context.Context, list client.ObjectList, opts ...client.ListOption)
}

func (c *testClient) SubResource(subResource string) client.SubResourceClient {
	return c.client.SubResource(subResource)
}

func newTestClient(cl client.Client) *testClient {
	return &testClient{client: cl}
}

func (c *testClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *testClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.inspectList(ctx, list, opts...)
	return c.client.List(ctx, list, opts...)
}

func (c *testClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *testClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
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
