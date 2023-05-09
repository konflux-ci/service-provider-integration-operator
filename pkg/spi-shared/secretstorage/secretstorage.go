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

package secretstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretID is a generic identifier of the secret that we store data of. While it very
// much resembles the Kubernetes client's ObjectKey, we keep it as a separate struct to
// be more explicit and forward-compatible should any changes to this struct arise in
// the future.
type SecretID struct {
	Uid       types.UID
	Name      string
	Namespace string
}

// String returns the string representation of the SecretID.
func (s SecretID) String() string {
	return fmt.Sprintf("%s/%s [uid=%s]", s.Namespace, s.Name, s.Uid)
}

var NotFoundError = errors.New("not found")
var ErrNoUid = errors.New("kubernetes object does not have UID")

// SecretStorage is a generic storage mechanism for storing secret data keyed by the SecretID.
type SecretStorage interface {
	// Initialize initializes the connection to the underlying data store, etc.
	Initialize(ctx context.Context) error
	// Store stores the provided data under given id
	Store(ctx context.Context, id SecretID, data []byte) error
	// Get retrieves the data under the given id. A NotFoundError is returned if the data is not found.
	Get(ctx context.Context, id SecretID) ([]byte, error)
	// Delete deletes the data of given id. A NotFoundError is returned if there is no such data.
	Delete(ctx context.Context, id SecretID) error
}

// TypedSecretStorage is a generic "companion" to the "raw" SecretStorage interface which uses
// strongly typed arguments instead of the generic SecretID and []byte.
type TypedSecretStorage[ID any, D any] interface {
	// Initialize initializes the connection to the underlying data store, etc.
	Initialize(ctx context.Context) error
	// Store stores the provided data under given id
	Store(ctx context.Context, id *ID, data *D) error
	// Get retrieves the data under the given id. A NotFoundError is returned if the data is not found.
	Get(ctx context.Context, id *ID) (*D, error)
	// Delete deletes the data of given id. A NotFoundError is returned if there is no such data.
	Delete(ctx context.Context, id *ID) error
}

// DefaultTypedSecretStorage is the default implementation of the TypedSecretStorage interface
// that uses the provided functions to convert between the id and data types to SecretID and []byte
// respectively.
type DefaultTypedSecretStorage[ID any, D any] struct {
	// DataTypeName is the human-readable name of the data type that is being stored. This is used
	// in error messages.
	DataTypeName string

	// SecretStorage is the underlying secret storage used for the actual operations against the persistent
	// storage. This must be initialized explicitly before it is used in this token storage instance.
	SecretStorage SecretStorage

	// ToID is a function that converts the strongly typed ID to the generic SecretID used by the SecretStorage.
	ToID func(*ID) (*SecretID, error)

	// Serialize is a function to convert the strongly type data into a byte array. You can use
	// for example the SerializeJSON function.
	Serialize func(*D) ([]byte, error)

	// Deserialize is a function to convert the byte array back to the strongly type data. You can use
	// for example the DeserializeJSON function.
	Deserialize func([]byte, *D) error
}

// ObjectToID converts given Kubernetes object to SecretID based on the name and namespace.
func ObjectToID[O client.Object](obj O) (*SecretID, error) {
	if obj.GetUID() == "" {
		return nil, fmt.Errorf("failed to convert object '%s/%s' to secret storage ID: %w", obj.GetNamespace(), obj.GetName(), ErrNoUid)
	}
	return &SecretID{
		Uid:       obj.GetUID(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

// SerializeJSON is a thin wrapper around Marshal function of encoding/json.
func SerializeJSON[D any](obj *D) ([]byte, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize data: %w", err)
	}
	return bytes, nil
}

// DeserializeJSON is a thin wrapper around Unmarshal function of encoding/json.
func DeserializeJSON[D any](data []byte, obj *D) error {
	if err := json.Unmarshal(data, obj); err != nil {
		return fmt.Errorf("failed to deserialize the data: %w", err)
	}
	return nil
}

var _ TypedSecretStorage[string, string] = (*DefaultTypedSecretStorage[string, string])(nil)

// Delete implements TypedSecretStorage
func (s *DefaultTypedSecretStorage[ID, D]) Delete(ctx context.Context, id *ID) error {
	realId, errId := s.ToID(id)
	if errId != nil {
		return fmt.Errorf("failed to create object id during deleting the secret: %w", errId)
	}
	if err := s.SecretStorage.Delete(ctx, *realId); err != nil {
		return fmt.Errorf("failed to delete %s: %w", s.DataTypeName, err)
	}
	return nil
}

// Get implements TypedSecretStorage
func (s *DefaultTypedSecretStorage[ID, D]) Get(ctx context.Context, id *ID) (*D, error) {
	realId, errId := s.ToID(id)
	if errId != nil {
		return nil, fmt.Errorf("failed to create object id during getting the secret: %w", errId)
	}

	d, err := s.SecretStorage.Get(ctx, *realId)
	if err != nil {
		return nil, fmt.Errorf("failed to get the %s: %w", s.DataTypeName, err)
	}

	var parsed D
	if err := s.Deserialize(d, &parsed); err != nil {
		return nil, fmt.Errorf("failed to deserialize the data to %s: %w", s.DataTypeName, err)
	}
	return &parsed, nil
}

// Initialize implements TypedSecretStorage. It is a noop.
func (s *DefaultTypedSecretStorage[ID, D]) Initialize(ctx context.Context) error {
	return nil
}

// Store implements TypedSecretStorage
func (s *DefaultTypedSecretStorage[ID, D]) Store(ctx context.Context, id *ID, data *D) error {
	secretId, errId := s.ToID(id)
	if errId != nil {
		return fmt.Errorf("failed to create object id during storing the secret: %w", errId)
	}

	bytes, err := s.Serialize(data)
	if err != nil {
		return fmt.Errorf("failed to serialize the %s for storage: %w", s.DataTypeName, err)
	}

	if err = s.SecretStorage.Store(ctx, *secretId, bytes); err != nil {
		return fmt.Errorf("failed to store %s: %w", s.DataTypeName, err)
	}

	return nil
}
