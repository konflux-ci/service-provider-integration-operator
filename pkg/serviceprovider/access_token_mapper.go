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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// Key for token using in Opaque Secret
const tokenKey = "token"

// AccessTokenMapper is a helper to convert token (together with its metadata) into maps suitable for storing in
// secrets according to the secret type.
type AccessTokenMapper struct {
	Name                    string   `json:"name"`
	Token                   string   `json:"token"`
	ServiceProviderUrl      string   `json:"serviceProviderUrl"` // We assume ServiceProviderUrl contains scheme, otherwise we can't parse the host properly.
	ServiceProviderUserName string   `json:"serviceProviderUserName"`
	ServiceProviderUserId   string   `json:"serviceProviderUserId"`
	UserId                  string   `json:"userId"`
	ExpiredAfter            *uint64  `json:"expiredAfter"`
	Scopes                  []string `json:"scopes"`
}

// ToSecretType converts the data in the mapper to a map with fields corresponding to the provided secret type.
func (at AccessTokenMapper) ToSecretType(secretType corev1.SecretType, mapping *api.TokenFieldMapping) (map[string]string, error) {
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
		dockerConfig, err := at.encodeDockerConfig()
		if err != nil {
			return nil, err
		}
		ret[corev1.DockerConfigJsonKey] = dockerConfig
	case corev1.SecretTypeSSHAuth:
		ret[corev1.SSHAuthPrivateKey] = at.Token
	default:
		at.fillByMapping(mapping, ret)
	}

	return ret, nil
}

// encodeDockerConfig constructs docker config json which can be used to pull private images from remote image registry.
func (at AccessTokenMapper) encodeDockerConfig() (string, error) {
	parsed, err := url.Parse(at.ServiceProviderUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse service provider url: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(at.ServiceProviderUserName + ":" + at.Token))

	type Auths map[string]struct {
		Auth string `json:"auth"`
	}
	dockerConfig := struct {
		Auths Auths `json:"auths"`
	}{
		Auths: Auths{
			parsed.Host: {
				Auth: encoded,
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(dockerConfig, "", "\t")
	if err != nil {
		return "", fmt.Errorf("failed to marshal docker config json: %w", err)
	}
	return string(jsonBytes), nil
}

// fillByMapping sets the data from the mapper into the provided map according to the settings specified in the provided
// mapping.
func (at AccessTokenMapper) fillByMapping(mapping *api.TokenFieldMapping, existingMap map[string]string) {

	// if there are no field in the mapping - add "token" key
	if mapping.Empty() {
		existingMap[tokenKey] = at.Token

	} else {
		if mapping.Token != "" {
			existingMap[mapping.Token] = at.Token
		}

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

		if mapping.UserId != "" {
			existingMap[mapping.UserId] = at.UserId
		}
	}

}
