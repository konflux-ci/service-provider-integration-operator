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
	at.ServiceProviderUrl = "https://spurl"
	at.ServiceProviderUserId = "spuserid"
	at.Token = "token"
	at.UserId = "userid"
}

func TestEmpty(t *testing.T) {
	t.Run("should be empty", func(t *testing.T) {
		mapping := &api.TokenFieldMapping{}
		assert.True(t, mapping.Empty(), "should be empty")
	})

	t.Run("should no be empty", func(t *testing.T) {
		mapping := &api.TokenFieldMapping{Token: "jdoe"}
		assert.False(t, mapping.Empty(), "should be empty")
	})
}

func TestSecretTypeDefaultFields(t *testing.T) {
	mapping := &api.TokenFieldMapping{}
	t.Run("basicAuth", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeBasicAuth, mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.ServiceProviderUserName, converted[corev1.BasicAuthUsernameKey])
		assert.Equal(t, at.Token, converted[corev1.BasicAuthPasswordKey])
	})

	t.Run("serviceAccountToken", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeServiceAccountToken, mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.Token, converted["extra"])
	})

	t.Run("dockercfg", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeDockercfg, mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.Token, converted[corev1.DockerConfigKey])
	})

	t.Run("dockerconfigjson", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeDockerConfigJson, mapping)
		assert.NoError(t, err)
		assert.Equal(t,
			`{
	"auths": {
		"spurl": {
			"auth": "c3B1c2VybmFtZTp0b2tlbg=="
		}
	}
}`, converted[corev1.DockerConfigJsonKey])
	})

	t.Run("ssh-privatekey", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeSSHAuth, mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.Token, converted[corev1.SSHAuthPrivateKey])
	})

	t.Run("default", func(t *testing.T) {
		converted, err := at.ToSecretType("", mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.Token, converted[tokenKey])
	})

	t.Run("opaque", func(t *testing.T) {
		converted, err := at.ToSecretType(corev1.SecretTypeOpaque, mapping)
		assert.NoError(t, err)
		assert.Equal(t, at.Token, converted[tokenKey])
	})
}

func TestEncodeDockerConfig(t *testing.T) {
	at = AccessTokenMapper{
		Token:                   "access-token",
		ServiceProviderUrl:      "https://url.com",
		ServiceProviderUserName: "joel",
	}

	t.Run("success", func(t *testing.T) {
		encoded, err := at.encodeDockerConfig()
		assert.NoError(t, err)
		assert.Equal(t,
			`{
	"auths": {
		"url.com": {
			"auth": "am9lbDphY2Nlc3MtdG9rZW4="
		}
	}
}`, encoded)
	})

	t.Run("url-parse-failure", func(t *testing.T) {
		newAt := at
		newAt.ServiceProviderUrl = "::bad.url"
		encoded, err := newAt.encodeDockerConfig()
		assert.Error(t, err)
		assert.Empty(t, encoded)
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

	at.fillByMapping(fields, converted)

	assert.Equal(t, at.Token, converted["TOKEN"])
	assert.Equal(t, at.Name, converted["NAME"])
	assert.Equal(t, at.ServiceProviderUrl, converted["SPURL"])
	assert.Equal(t, at.ServiceProviderUserName, converted["SPUSERNAME"])
	assert.Equal(t, at.ServiceProviderUserId, converted["SPUSERID"])
	assert.Equal(t, at.UserId, converted["USERID"])
	assert.Equal(t, str(at.ExpiredAfter), converted["EXPIREDAFTER"])
	assert.Equal(t, strings.Join(at.Scopes, ","), converted["SCOPES"])
}
