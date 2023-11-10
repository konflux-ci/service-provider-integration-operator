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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenericLookup_Lookup(t *testing.T) {
	matchingToken := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken1 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching1",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken2 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching2",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}

	cl := mockK8sClient(matchingToken, nonMatchingToken1, nonMatchingToken2)

	cache := MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
		CacheServiceProviderState: true,
	}
	gl := GenericLookup{
		ServiceProviderType: "test",
		TokenFilter: TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return token.Name == "matching", nil
		}),
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}),
		MetadataCache: &cache,
		RepoUrlParser: RepoUrlFromString,
	}

	tkns, err := gl.Lookup(context.TODO(), cl, &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "https://fake.sp",
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(tkns))
	assert.Equal(t, "matching", tkns[0].Name)
}

func TestGenericLookup_PersistMetadata(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{},
	}

	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(token).WithStatusSubresource(token).Build()

	cache := MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
		CacheServiceProviderState: true,
	}

	fetchCalled := false
	gl := GenericLookup{
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			fetchCalled = true
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}),
		MetadataCache: &cache,
		RepoUrlParser: RepoUrlFromString,
	}

	assert.NoError(t, gl.PersistMetadata(context.TODO(), token))
	assert.True(t, fetchCalled)

	assert.NoError(t, cl.Get(context.TODO(), client.ObjectKeyFromObject(token), token))
	assert.NotNil(t, token.Status.TokenMetadata)
	assert.Equal(t, "42", token.Status.TokenMetadata.UserId)
	assert.True(t, token.Status.TokenMetadata.LastRefreshTime > 0)
}

func TestGenericLookup_LookupRemoteSecrets(t *testing.T) {
	matchingLabelAndName := &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-name",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
	}
	matchingJustLabel := &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-label",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
	}
	nonMatching := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "diffrent.host",
			},
		},
	}

	check := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "default",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: "https://test",
		},
	}

	cl := mockK8sClient(matchingLabelAndName, matchingJustLabel, nonMatching)

	gl := GenericLookup{
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return remoteSecret.Name == "matching-name"
		}),
		RepoUrlParser: RepoUrlFromSchemalessString,
	}

	remoteSecrets, err := gl.lookupRemoteSecrets(context.TODO(), cl, &check)
	assert.NoError(t, err)
	assert.Len(t, remoteSecrets, 1)
	assert.Equal(t, "matching-name", remoteSecrets[0].Name)
}

func TestGenericLookup_LookupRemoteSecretSecret(t *testing.T) {
	remoteSecrets := []v1beta1.RemoteSecret{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-rs",
			Namespace: "default",
			Annotations: map[string]string{
				api.RSServiceProviderRepositoryAnnotation: "test/repo,diff/repo",
			},
		},
		Status: v1beta1.RemoteSecretStatus{
			Targets: []v1beta1.TargetStatus{{
				Namespace:  "default",
				SecretName: "rs-secret",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching",
			Namespace: "default",
			Annotations: map[string]string{
				api.RSServiceProviderRepositoryAnnotation: "diff/repo,anoth/repo",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching",
			Namespace: "default",
		},
	}}

	check := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "default",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: "https://test",
		},
	}

	remoteSecretSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-secret",
			Namespace: "default",
		},
	}

	cl := mockK8sClient(&remoteSecretSecret)
	gl := GenericLookup{
		RepoUrlParser: RepoUrlFromSchemalessString,
	}

	secret, err := gl.lookupRemoteSecretSecret(context.TODO(), cl, &check, remoteSecrets)
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "rs-secret", secret.Name)
}

func TestGenericLookup_LookupCredentials(t *testing.T) {
	remoteSecret := v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
		Status: v1beta1.RemoteSecretStatus{
			Targets: []v1beta1.TargetStatus{{
				Namespace:  "default",
				SecretName: "rs-secret",
			}},
		},
	}

	spiToken := api.SPIAccessToken{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "test",
			},
		},
		Spec: api.SPIAccessTokenSpec{},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}

	check := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "default",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: "https://test",
		},
	}

	remoteSecretSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("rs-username"),
			corev1.BasicAuthPasswordKey: []byte("rs-password"),
		},
	}

	cl := mockK8sClient(&remoteSecret, &spiToken, &remoteSecretSecret)
	gl := GenericLookup{
		ServiceProviderType: "test",
		TokenFilter: TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return token.Name == "matching", nil
		}),
		TokenStorage: tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{AccessToken: "spi-password", Username: "spi-username"}, nil
		}},
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return remoteSecret.Name == "matching"
		}),
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}), MetadataCache: &MetadataCache{
			Client:                    cl,
			ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
			CacheServiceProviderState: true,
		},
		RepoUrlParser: RepoUrlFromSchemalessString,
	}

	t.Run("credentials from SPIAccessToken", func(t *testing.T) {
		credentials, err := gl.LookupCredentials(context.TODO(), cl, &check)
		assert.NoError(t, err)
		assert.NotNil(t, credentials)
		assert.Equal(t, "spi-username", credentials.Username)
		assert.Equal(t, "spi-password", credentials.Token)
	})

	t.Run("credentials from RemoteSecret", func(t *testing.T) {
		gl.TokenFilter = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return false, nil
		})
		credentials, err := gl.LookupCredentials(context.TODO(), cl, &check)
		assert.NoError(t, err)
		assert.NotNil(t, credentials)
		assert.Equal(t, "rs-username", credentials.Username)
		assert.Equal(t, "rs-password", credentials.Token)
	})

	t.Run("no credentials", func(t *testing.T) {
		gl.TokenFilter = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return false, nil
		})
		gl.RemoteSecretFilter = RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return false
		})
		credentials, err := gl.LookupCredentials(context.TODO(), cl, &check)
		assert.NoError(t, err)
		assert.Nil(t, credentials)
	})

}

func TestGetLocalNamespaceTargetIndex(t *testing.T) {
	targets := []v1beta1.TargetStatus{
		{
			Namespace:  "ns",
			ApiUrl:     "url",
			SecretName: "sec0",
		},
		{
			Namespace:  "ns",
			SecretName: "sec1",
			Error:      "some error",
		},
		{
			Namespace:  "diff-ns",
			SecretName: "sec2",
		},
	}

	assert.Equal(t, -1, getLocalNamespaceTargetIndex(targets, "ns"))
	targets = append(targets, v1beta1.TargetStatus{
		Namespace:  "ns",
		SecretName: "sec3",
	})
	assert.Equal(t, 3, getLocalNamespaceTargetIndex(targets, "ns"))
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	utilruntime.Must(v1beta1.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).WithStatusSubresource(objects...).Build()
}
