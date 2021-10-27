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

package controllers

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

var at accessToken

func init() {
	at = accessToken{}
	at[expiredAfterField] = "expired"
	at[nameField] = "name"
	at[scopesField] = "scopes"
	at[serviceProvideUserNameField] = "spusername"
	at[serviceProviderUrlField] = "spurl"
	at[serviceProviderUserIdField] = "spuserid"
	at[tokenField] = "token"
	at[userIdField] = "userid"
}

func TestSecretTypeDefaultFields(t *testing.T) {
	t.Run("basicAuth", func(t *testing.T) {
		converted := at.toSecretType(corev1.SecretTypeBasicAuth)
		assert.Equal(t, at[serviceProvideUserNameField], converted[corev1.BasicAuthUsernameKey])
		assert.Equal(t, at[tokenField], converted[corev1.BasicAuthPasswordKey])
	})

	t.Run("serviceAccountToken", func(t *testing.T) {
		converted := at.toSecretType(corev1.SecretTypeServiceAccountToken)
		assert.Equal(t, at[tokenField], converted["extra"])
	})

	t.Run("dockercfg", func(t *testing.T) {
		converted := at.toSecretType(corev1.SecretTypeDockercfg)
		assert.Equal(t, at[tokenField], converted[corev1.DockerConfigKey])
	})

	t.Run("dockerconfigjson", func(t *testing.T) {
		converted := at.toSecretType(corev1.SecretTypeDockerConfigJson)
		assert.Equal(t, at[tokenField], converted[corev1.DockerConfigJsonKey])
	})
}

func TestMapping(t *testing.T) {
	fields := &api.AccessTokenSecretFieldMapping{
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

	assert.Equal(t, at[tokenField], converted["TOKEN"])
	assert.Equal(t, at[nameField], converted["NAME"])
	assert.Equal(t, at[serviceProviderUrlField], converted["SPURL"])
	assert.Equal(t, at[serviceProvideUserNameField], converted["SPUSERNAME"])
	assert.Equal(t, at[serviceProviderUserIdField], converted["SPUSERID"])
	assert.Equal(t, at[userIdField], converted["USERID"])
	assert.Equal(t, at[expiredAfterField], converted["EXPIREDAFTER"])
	assert.Equal(t, at[scopesField], converted["SCOPES"])
}
