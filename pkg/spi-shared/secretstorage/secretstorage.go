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
	"errors"
	"fmt"
)

// SecretID is a generic identifier of the secret that we store data of. While it very
// much resembles the Kubernetes client's ObjectKey, we keep it as a separate struct to
// be more explicit and forward-compatible should any changes to this struct arise in
// the future.
type SecretID struct {
	Name          string
	Namespace     string
	SpiInstanceId string
}

// String returns the string representation of the SecretID.
func (s SecretID) String() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

var NotFoundError = errors.New("not found")

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
