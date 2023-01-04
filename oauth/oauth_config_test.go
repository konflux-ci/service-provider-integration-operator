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

func TestCreateOauthConfigFromSecret(t *testing.T) {
	t.Run("all fields set ok", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				config.OAuthCfgSecretFieldClientId:     []byte(testClientId),
				config.OAuthCfgSecretFieldClientSecret: []byte(testClientSecret),
				config.OAuthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				config.OAuthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, testAuthUrl, oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, testTokenUrl, oauthCfg.Endpoint.TokenURL)
	})

	t.Run("error if missing client id", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				config.OAuthCfgSecretFieldClientSecret: []byte(testClientSecret),
				config.OAuthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				config.OAuthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("error if missing client secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				config.OAuthCfgSecretFieldClientId: []byte(testClientId),
				config.OAuthCfgSecretFieldAuthUrl:  []byte(testAuthUrl),
				config.OAuthCfgSecretFieldTokenUrl: []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("ok with just client id and secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				config.OAuthCfgSecretFieldClientId:     []byte(testClientId),
				config.OAuthCfgSecretFieldClientSecret: []byte(testClientSecret),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, "", oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, "", oauthCfg.Endpoint.TokenURL)
	})
}

func TestObtainOauthConfig(t *testing.T) {
	t.Run("no secret use default oauth config", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl := commonController{
			ServiceProviderInstance: map[string]oauthConfiguration{
				"bleh.eh": {
					Config: config.ServiceProviderConfiguration{
						Oauth2Config: &oauth2.Config{
							ClientID:     "eh?",
							ClientSecret: "bleh?",
						},
						ServiceProviderType:    config.ServiceProviderTypeGitHub,
						ServiceProviderBaseUrl: "http://bleh.eh",
					},
					Endpoint: github.Endpoint,
				},
			},
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			K8sClient:           cl,
			BaseUrl:             "baseurl",
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
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
					Data: map[string][]byte{
						config.OAuthCfgSecretFieldClientId:     []byte("testclientid"),
						config.OAuthCfgSecretFieldClientSecret: []byte("testclientsecret"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			ServiceProviderInstance: map[string]oauthConfiguration{
				config.GithubSaasHost: {
					Config: config.ServiceProviderConfiguration{
						Oauth2Config: &oauth2.Config{
							ClientID:     "eh?",
							ClientSecret: "bleh?",
						},
						ServiceProviderType:    config.ServiceProviderTypeGitHub,
						ServiceProviderBaseUrl: "http://bleh.eh",
					},
					Endpoint: github.Endpoint,
				},
			},
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			K8sClient:           cl,
			BaseUrl:             "baseurl",
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			ServiceProviderUrl:  "http://bleh.eh",
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)
		expectedEndpoint := createDefaultEndpoint("http://bleh.eh")

		assert.NoError(t, err)
		assert.NotNil(t, oauthCfg)
		assert.Equal(t, oauthCfg.ClientID, "testclientid")
		assert.Equal(t, oauthCfg.ClientSecret, "testclientsecret")
		assert.Equal(t, expectedEndpoint, oauthCfg.Endpoint)
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
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
					Data: map[string][]byte{
						config.OAuthCfgSecretFieldClientId: []byte("testclientid"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			ServiceProviderInstance: map[string]oauthConfiguration{
				config.GithubSaasHost: {
					Config: config.ServiceProviderConfiguration{
						Oauth2Config: &oauth2.Config{
							ClientID:     "eh?",
							ClientSecret: "bleh?",
						},
						ServiceProviderType:    config.ServiceProviderTypeGitHub,
						ServiceProviderBaseUrl: "http://bleh.eh",
					},
					Endpoint: github.Endpoint,
				},
			},
			ServiceProviderType: config.ServiceProviderTypeGitHub,
			K8sClient:           cl,
			BaseUrl:             "baseurl",
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
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
			K8sClient:           cl,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
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
			ServiceProviderInstance: map[string]oauthConfiguration{},
			ServiceProviderType:     config.ServiceProviderTypeGitHub,
			K8sClient:               cl,
			BaseUrl:                 "baseurl",
		}

		oauthInfo := &oauthstate2.OAuthInfo{
			ServiceProviderUrl:  "http://bleh.eh",
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthInfo)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("no auth in list secret request context", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))

		ctx := WithAuthIntoContext("token", context.TODO())

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
			K8sClient:           testC,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
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
