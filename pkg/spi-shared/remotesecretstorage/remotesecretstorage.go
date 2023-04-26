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

package remotesecretstorage

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
)

type SecretData map[string][]byte
type RemoteSecretStorage secretstorage.TypedSecretStorage[api.RemoteSecret, SecretData]

// NewJSONSerializingRemoteSecretStorage is a convenience function to construct a RemoteSecretStorage instance
// based on the provided SecretStorage and serializing the data to JSON for persistence.
// The returned object is an instance of DefaultTypedSecretStorage set up to work with RemoteSecret
// objects as data keys and returning the SecretData instances.
// NOTE that the provided secret storage MUST BE initialized before this call.
func NewJSONSerializingRemoteSecretStorage(secretStorage secretstorage.SecretStorage) RemoteSecretStorage {
	return &secretstorage.DefaultTypedSecretStorage[api.RemoteSecret, SecretData]{
		DataTypeName:  "remote secret",
		SecretStorage: secretStorage,
		ToID:          secretstorage.ObjectToID[*api.RemoteSecret],
		Serialize:     secretstorage.SerializeJSON[SecretData],
		Deserialize:   secretstorage.DeserializeJSON[SecretData],
	}
}
