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
	"time"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRSCheckIdentityHasAccess(t *testing.T) {
	t.Skip("not testable until we use controller-runtime >= 0.15.x because we need to fake SubjectAccessReview using interceptors")
}

func TestRSSyncTokenData(t *testing.T) {
	exchange := &exchangeResult{
		OAuthInfo: oauthstate.OAuthInfo{
			ObjectName:      "test-rs",
			ObjectNamespace: "default",
		},
		result: 0,
		token: &oauth2.Token{
			AccessToken:  "token",
			RefreshToken: "no-no-value",
			TokenType:    "Bearer",
			Expiry:       time.Now(),
		},
		authorizationHeader: "",
	}
	rs := &v1beta1.RemoteSecret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: "default",
		},
	}
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(v1beta1.AddToScheme(sch))
	client := fake.NewClientBuilder().WithScheme(sch).WithObjects(rs).Build()

	strategy := RemoteSecretSyncStrategy{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: client},
	}

	err := strategy.syncTokenData(context.TODO(), exchange)
	assert.NoError(t, err)

	err = client.Get(context.TODO(), client2.ObjectKeyFromObject(rs), rs)
	assert.NoError(t, err)

	// no webhook here so we can check the data straight from RemoteSecret
	assert.Len(t, rs.StringUploadData, 4)
	assert.Equal(t, "token", rs.StringUploadData[corev1.BasicAuthPasswordKey])
}
