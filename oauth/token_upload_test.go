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
	"fmt"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTokenUploader_ShouldUploadWithNoError(t *testing.T) {
	//given
	scheme := runtime.NewScheme()
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	cntx := clientfactory.NamespaceIntoContext(context.TODO(), "ns-1")
	tokenData := &api.Token{AccessToken: "2345-2345-2345-234-46456", Username: "jdoe"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1beta1.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token-123",
				Namespace: "ns-1",
			},
		},
	).Build()

	strg := tokenstorage.NotifyingTokenStorage{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		TokenStorage: tokenstorage.TestTokenStorage{
			StoreImpl: func(ctx context.Context, token *v1beta1.SPIAccessToken, data *v1beta1.Token) error {
				nsFromBaseCntx, _ := clientfactory.NamespaceFromContext(cntx)
				nsFromInvokedCtx, _ := clientfactory.NamespaceFromContext(ctx)
				assert.Equal(t, nsFromBaseCntx, nsFromInvokedCtx)
				assert.Equal(t, "jdoe", data.Username)
				assert.Equal(t, "2345-2345-2345-234-46456", data.AccessToken)
				assert.Equal(t, "ns-1", token.Namespace)
				assert.Equal(t, "token-123", token.Name)
				return nil
			},
		},
	}

	uploader := SpiTokenUploader{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		Storage:       strg,
	}

	//when
	err := uploader.Upload(cntx, "token-123", "ns-1", tokenData)

	//then
	if err != nil {
		t.Fatal("error should not happen", err)
	}
}

func TestTokenUploader_ShouldFailTokenNotFound(t *testing.T) {
	//given
	scheme := runtime.NewScheme()
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	cntx := context.TODO()
	tokenData := &api.Token{AccessToken: "2345-2345-2345-234-46456", Username: "jdoe"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1beta1.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token-324",
				Namespace: "ns-1",
			},
		},
	).Build()

	strg := tokenstorage.NotifyingTokenStorage{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		TokenStorage: tokenstorage.TestTokenStorage{
			StoreImpl: func(ctx context.Context, token *v1beta1.SPIAccessToken, data *v1beta1.Token) error {
				assert.Fail(t, "This line should not be reached")
				return nil
			},
		},
	}

	uploader := SpiTokenUploader{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		Storage:       strg,
	}
	//when
	err := uploader.Upload(cntx, "token-123", "ns-1", tokenData)

	//then
	var expectedErrorMsg = "failed to get SPIAccessToken object ns-1/token-123: spiaccesstokens.appstudio.redhat.com \"token-123\" not found"
	assert.EqualErrorf(t, err, expectedErrorMsg, "Error should be: %v, got: %v", expectedErrorMsg, err)

}

func TestTokenUploader_ShouldFailOnStorage(t *testing.T) {
	//given
	scheme := runtime.NewScheme()
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	cntx := context.TODO()
	tokenData := &api.Token{AccessToken: "2345-2345-2345-234-46456", Username: "jdoe"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1beta1.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token-123",
				Namespace: "ns-1",
			},
		},
	).Build()

	strg := tokenstorage.NotifyingTokenStorage{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		TokenStorage: tokenstorage.TestTokenStorage{
			StoreImpl: func(ctx context.Context, token *v1beta1.SPIAccessToken, data *v1beta1.Token) error {
				return fmt.Errorf("storage disconnected")
			},
		},
	}

	uploader := SpiTokenUploader{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: cl},
		Storage:       strg,
	}
	//when
	err := uploader.Upload(cntx, "token-123", "ns-1", tokenData)

	//then
	var expectedErrorMsg = "failed to store the token data into storage: wrapped storage error: storage disconnected"
	assert.EqualErrorf(t, err, expectedErrorMsg, "Error should be: %v, got: %v", expectedErrorMsg, err)
}
