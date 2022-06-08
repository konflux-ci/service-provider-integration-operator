/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package serviceprovider

import (
	"strconv"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

var at AccessTokenMapper

func uint64Ptr(val uint64) *uint64 {
	return &val
}

func str(val *uint64) string {
	if val == nil {
		return ""
	}

	return strconv.FormatUint(*val, 10)
}

func init() {
	at = AccessTokenMapper{}
	at.ExpiredAfter = uint64Ptr(42)
	at.Name = "name"
	at.Scopes = []string{"scope1", "scope2"}
	at.ServiceProviderUserName = "spusername"
	at.ServiceProviderUrl = "spurl"
	at.ServiceProviderUserId = "spuserid"
	at.Token = "token"
	at.UserId = "userid"
}

func TestSecretTypeDefaultFields(t *testing.T) {
	t.Run("basicAuth", func(t *testing.T) {
		converted := at.ToSecretType(corev1.SecretTypeBasicAuth)
		assert.Equal(t, at.ServiceProviderUserName, converted[corev1.BasicAuthUsernameKey])
		assert.Equal(t, at.Token, converted[corev1.BasicAuthPasswordKey])
	})

	t.Run("serviceAccountToken", func(t *testing.T) {
		converted := at.ToSecretType(corev1.SecretTypeServiceAccountToken)
		assert.Equal(t, at.Token, converted["extra"])
	})

	t.Run("dockercfg", func(t *testing.T) {
		converted := at.ToSecretType(corev1.SecretTypeDockercfg)
		assert.Equal(t, at.Token, converted[corev1.DockerConfigKey])
	})

	t.Run("dockerconfigjson", func(t *testing.T) {
		converted := at.ToSecretType(corev1.SecretTypeDockerConfigJson)
		assert.Equal(t, `{"auths":{"spurl":{"username":"spusername","password":"token"}}}`, converted[corev1.DockerConfigJsonKey])
	})

	t.Run("dockerconfigjson-urlWithScheme", func(t *testing.T) {
		newAt := at // copy to not affect other tests
		newAt.ServiceProviderUrl = "http://quay.io/somepath"
		converted := newAt.ToSecretType(corev1.SecretTypeDockerConfigJson)
		assert.Equal(t, `{"auths":{"quay.io":{"username":"spusername","password":"token"}}}`, converted[corev1.DockerConfigJsonKey])
	})

	t.Run("ssh-privatekey", func(t *testing.T) {
		converted := at.ToSecretType(corev1.SecretTypeSSHAuth)
		assert.Equal(t, at.Token, converted[corev1.SSHAuthPrivateKey])
	})
}

func TestMapping(t *testing.T) {
	fields := &api.TokenFieldMapping{
		Token:                   "TOKEN",
		Name:                    "NAME",
		ServiceProviderUrl:      "SPURL",
		ServiceProviderUserName: "SPUSERNAME",
		ServiceProviderUserId:   "SPUSERID",
		UserId:                  "USERID",
		ExpiredAfter:            "EXPIREDAFTER",
		Scopes:                  "SCOPES",
	}

	converted := map[string]string{}

	at.FillByMapping(fields, converted)

	assert.Equal(t, at.Token, converted["TOKEN"])
	assert.Equal(t, at.Name, converted["NAME"])
	assert.Equal(t, at.ServiceProviderUrl, converted["SPURL"])
	assert.Equal(t, at.ServiceProviderUserName, converted["SPUSERNAME"])
	assert.Equal(t, at.ServiceProviderUserId, converted["SPUSERID"])
	assert.Equal(t, at.UserId, converted["USERID"])
	assert.Equal(t, str(at.ExpiredAfter), converted["EXPIREDAFTER"])
	assert.Equal(t, strings.Join(at.Scopes, ","), converted["SCOPES"])
}
