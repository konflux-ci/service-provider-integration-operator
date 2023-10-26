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

	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSPICheckIdentityHasAccess(t *testing.T) {
	// TODO: finish
}

func TestSPISyncTokenData(t *testing.T) {
	exchange := &exchangeResult{
		OAuthInfo: oauthstate.OAuthInfo{
			ObjectName:      "test-token",
			ObjectNamespace: "default",
		},
		result: 0,
		token: &oauth2.Token{
			AccessToken: "token",
		},
		authorizationHeader: "",
	}
	spiToken := &api.SPIAccessToken{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
	}
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	client := fake.NewClientBuilder().WithScheme(sch).WithObjects(spiToken).Build()

	ts := tokenstorage.TestTokenStorage{
		StoreImpl: func(ctx context.Context, spiToken *api.SPIAccessToken, token *api.Token) error {
			assert.Equal(t, "token", token.AccessToken)
			assert.Equal(t, "test-token", spiToken.Name)
			assert.Equal(t, "default", spiToken.Namespace)
			return nil
		},
	}
	strategy := SPIAccessTokenSyncStrategy{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: client},
		TokenStorage:  ts,
	}

	err := strategy.syncTokenData(context.TODO(), exchange)
	assert.NoError(t, err)
}
