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
	"fmt"
	"net/url"
	"strconv"
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// AccessTokenMapper is a helper to convert token (together with its metadata) into maps suitable for storing in
// secrets according to the secret type.
type AccessTokenMapper struct {
	Name                    string   `json:"name"`
	Token                   string   `json:"token"`
	ServiceProviderUrl      string   `json:"serviceProviderUrl"`
	ServiceProviderUserName string   `json:"serviceProviderUserName"`
	ServiceProviderUserId   string   `json:"serviceProviderUserId"`
	UserId                  string   `json:"userId"`
	ExpiredAfter            *uint64  `json:"expiredAfter"`
	Scopes                  []string `json:"scopes"`
}

// ToSecretType converts the data in the mapper to a map with fields corresponding to the provided secret type.
func (at AccessTokenMapper) ToSecretType(secretType corev1.SecretType) map[string]string {
	ret := map[string]string{}
	switch secretType {
	case corev1.SecretTypeBasicAuth:
		ret[corev1.BasicAuthUsernameKey] = at.ServiceProviderUserName
		ret[corev1.BasicAuthPasswordKey] = at.Token
	case corev1.SecretTypeServiceAccountToken:
		ret["extra"] = at.Token
	case corev1.SecretTypeDockercfg:
		ret[corev1.DockerConfigKey] = at.Token
	case corev1.SecretTypeDockerConfigJson:
		// parse host from ServiceProviderUrl if that is not possible use the whole ServiceProviderUrl as host
		host := at.ServiceProviderUrl
		parsed, err := url.Parse(at.ServiceProviderUrl)
		if err == nil && parsed.Host != "" {
			host = parsed.Host
		}
		ret[corev1.DockerConfigJsonKey] = fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s"}}}`,
			host,
			at.ServiceProviderUserName,
			at.Token)
	case corev1.SecretTypeSSHAuth:
		ret[corev1.SSHAuthPrivateKey] = at.Token
	}

	return ret
}

// FillByMapping sets the data from the mapper into the provided map according to the settings specified in the provided
// mapping.
func (at AccessTokenMapper) FillByMapping(mapping *api.TokenFieldMapping, existingMap map[string]string) {
	if mapping.ExpiredAfter != "" && at.ExpiredAfter != nil {
		existingMap[mapping.ExpiredAfter] = strconv.FormatUint(*at.ExpiredAfter, 10)
	}

	if mapping.Name != "" {
		existingMap[mapping.Name] = at.Name
	}

	if mapping.Scopes != "" {
		existingMap[mapping.Scopes] = strings.Join(at.Scopes, ",")
	}

	if mapping.ServiceProviderUrl != "" {
		existingMap[mapping.ServiceProviderUrl] = at.ServiceProviderUrl
	}

	if mapping.ServiceProviderUserId != "" {
		existingMap[mapping.ServiceProviderUserId] = at.ServiceProviderUserId
	}

	if mapping.ServiceProviderUserName != "" {
		existingMap[mapping.ServiceProviderUserName] = at.ServiceProviderUserName
	}

	if mapping.Token != "" {
		existingMap[mapping.Token] = at.Token
	}

	if mapping.UserId != "" {
		existingMap[mapping.UserId] = at.UserId
	}
}
