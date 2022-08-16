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
	"encoding/json"
	"os"
	"testing"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testToken = &v1beta1.Token{
	Username:     "testUsername",
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

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}

func TestStorage(t *testing.T) {
	cluster, storage := CreateTestVaultTokenStorage(t)
	defer cluster.Cleanup()

	test := func(ctx context.Context) {
		err := storage.Store(ctx, testSpiAccessToken, testToken)
		assert.NoError(t, err)

		gettedToken, err := storage.Get(ctx, testSpiAccessToken)
		assert.NoError(t, err)
		assert.NotNil(t, gettedToken)
		assert.EqualValues(t, testToken, gettedToken)

		err = storage.Delete(ctx, testSpiAccessToken)
		assert.NoError(t, err)

		gettedToken, err = storage.Get(ctx, testSpiAccessToken)
		assert.NoError(t, err)
		assert.Nil(t, gettedToken)
	}

	t.Run("single cluster", func(t *testing.T) {
		test(context.TODO())
	})

	t.Run("kcp", func(t *testing.T) {
		test(logicalcluster.WithCluster(context.Background(), logicalcluster.New("test_workspace")))
	})

	t.Run("token accessible only in it's workspace", func(t *testing.T) {
		workspaceCtx := logicalcluster.WithCluster(context.Background(), logicalcluster.New("test_workspace"))

		err := storage.Store(workspaceCtx, testSpiAccessToken, testToken)
		assert.NoError(t, err)

		gettedToken, err := storage.Get(context.TODO(), testSpiAccessToken)
		assert.NoError(t, err)
		assert.Nil(t, gettedToken)

		gettedToken, err = storage.Get(logicalcluster.WithCluster(context.Background(), logicalcluster.New("another_workspace")), testSpiAccessToken)
		assert.NoError(t, err)
		assert.Nil(t, gettedToken)

		gettedToken, err = storage.Get(workspaceCtx, testSpiAccessToken)
		assert.NoError(t, err)
		assert.NotNil(t, gettedToken)
		assert.EqualValues(t, testToken, gettedToken)

		err = storage.Delete(workspaceCtx, testSpiAccessToken)
		assert.NoError(t, err)
	})

}

func TestParseToken(t *testing.T) {
	t.Run("nil data", func(t *testing.T) {
		token, err := parseToken(nil)
		assert.Nil(t, token)
		assert.NotNil(t, err)
	})

	t.Run("wrong data", func(t *testing.T) {
		token, err := parseToken(v1beta1.SPIAccessToken{})
		assert.Nil(t, token)
		assert.NotNil(t, err)
	})

	t.Run("full token", func(t *testing.T) {
		data := map[string]interface{}{
			"username":      "un",
			"access_token":  "at",
			"token_type":    "tt",
			"refresh_token": "rt",
			"expiry":        json.Number("1337"),
		}
		token, err := parseToken(data)
		assert.Nil(t, err)
		assert.Equal(t, "un", token.Username)
		assert.Equal(t, "at", token.AccessToken)
		assert.Equal(t, "tt", token.TokenType)
		assert.Equal(t, "rt", token.RefreshToken)
		assert.Equal(t, uint64(1337), token.Expiry)
	})

	t.Run("empty token", func(t *testing.T) {
		data := map[string]interface{}{}
		token, err := parseToken(data)
		assert.Nil(t, err)
		assert.NotNil(t, token)
		assert.Equal(t, "", token.Username)
		assert.Equal(t, "", token.AccessToken)
		assert.Equal(t, "", token.TokenType)
		assert.Equal(t, "", token.RefreshToken)
		assert.Equal(t, uint64(0), token.Expiry)
	})

	t.Run("expiry not json.Number", func(t *testing.T) {
		data := map[string]interface{}{
			"expiry": 1337,
		}
		token, err := parseToken(data)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), token.Expiry)
	})

	t.Run("invalid expiry", func(t *testing.T) {
		data := map[string]interface{}{
			"expiry": json.Number("blabol"),
		}
		token, err := parseToken(data)
		assert.NotNil(t, err)
		assert.Nil(t, token)
	})
}
