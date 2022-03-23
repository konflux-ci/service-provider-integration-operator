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

package tokenstorage

import (
	"context"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testToken = &v1beta1.Token{
	AccessToken:  "testAccessToken",
	TokenType:    "testTokenType",
	RefreshToken: "testRefreshToken",
	Expiry:       123,
}

var testSpiAccessToken = &v1beta1.SPIAccessToken{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "testSpiAccessToken",
		Namespace: "testNamespace",
	},
}

func TestStorage(t *testing.T) {
	cluster, storage := CreateTestVaultTokenStorage(t)
	defer cluster.Cleanup()

	err := storage.Store(context.TODO(), testSpiAccessToken, testToken)
	assert.NoError(t, err)

	gettedToken, err := storage.Get(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)
	assert.NotNil(t, gettedToken)
	assert.EqualValues(t, testToken, gettedToken)

	err = storage.Delete(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)

	gettedToken, err = storage.Get(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)
	assert.Nil(t, gettedToken)
}
