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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type tokenRecordField string

const (
	tokenField                  tokenRecordField = "token"
	nameField                   tokenRecordField = "name"
	serviceProviderUrlField     tokenRecordField = "serviceProviderUrl"
	serviceProvideUserNameField tokenRecordField = "serviceProviderUserName"
	serviceProviderUserIdField  tokenRecordField = "serviceProviderUserId"
	userIdField                 tokenRecordField = "userId"
	expiredAfterField           tokenRecordField = "expiredAfter"
	scopesField                 tokenRecordField = "scopes"
)

var (
	fieldToKeyBySecretType map[corev1.SecretType]map[tokenRecordField]string
)

func init() {
	fieldToKeyBySecretType = map[corev1.SecretType]map[tokenRecordField]string{
		corev1.SecretTypeBasicAuth: {
			serviceProvideUserNameField: corev1.BasicAuthUsernameKey,
			tokenField:                  corev1.BasicAuthPasswordKey,
		},
		corev1.SecretTypeServiceAccountToken: {
			tokenField: "extra",
		},
		corev1.SecretTypeDockercfg: {
			tokenField: corev1.DockerConfigKey,
		},
		corev1.SecretTypeDockerConfigJson: {
			tokenField: corev1.DockerConfigJsonKey,
		},
	}
}

type accessToken map[tokenRecordField]string

func (at accessToken) toSecretType(secretType corev1.SecretType) map[string]string {
	ret := map[string]string{}
	mapping, ok := fieldToKeyBySecretType[secretType]
	if !ok {
		return ret
	}

	for f, k := range mapping {
		ret[k] = at[f]
	}

	return ret
}

func (at accessToken) fillByMapping(mapping *api.AccessTokenSecretFieldMapping, existingMap map[string]string) {
	if mapping.ExpiredAfter != "" {
		existingMap[mapping.ExpiredAfter] = at[expiredAfterField]
	}

	if mapping.Name != "" {
		existingMap[mapping.Name] = at[nameField]
	}

	if mapping.Scopes != "" {
		existingMap[mapping.Scopes] = at[scopesField]
	}

	if mapping.ServiceProviderUrl != "" {
		existingMap[mapping.ServiceProviderUrl] = at[serviceProviderUrlField]
	}

	if mapping.ServiceProviderUserId != "" {
		existingMap[mapping.ServiceProviderUserId] = at[serviceProviderUserIdField]
	}

	if mapping.ServiceProviderUserName != "" {
		existingMap[mapping.ServiceProviderUserName] = at[serviceProvideUserNameField]
	}

	if mapping.Token != "" {
		existingMap[mapping.Token] = at[tokenField]
	}

	if mapping.UserId != "" {
		existingMap[mapping.UserId] = at[userIdField]
	}
}
