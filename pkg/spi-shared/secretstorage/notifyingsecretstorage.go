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
	"fmt"

	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/remote-secret/pkg/secretstorage"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This is a wrapper around the provided SecretStorage that creates the SPIAccessTokenDataUpdate
// objects on data modifications.
// The supplied secret storage must be initialized explicitly before it can be used by this storage.
type NotifyingSecretStorage struct {
	ClientFactory kubernetesclient.K8sClientFactory
	SecretStorage secretstorage.SecretStorage
	Group         string
	Kind          string
}

var _ secretstorage.SecretStorage = (*NotifyingSecretStorage)(nil)

// Delete implements SecretStorage
func (s *NotifyingSecretStorage) Delete(ctx context.Context, id secretstorage.SecretID) error {
	if err := s.SecretStorage.Delete(ctx, id); err != nil {
		return fmt.Errorf("wrapped storage error: %w", err)
	}

	return s.createDataUpdate(ctx, id)
}

// Get implements SecretStorage
func (s *NotifyingSecretStorage) Get(ctx context.Context, id secretstorage.SecretID) ([]byte, error) {
	var data []byte
	var err error

	if data, err = s.SecretStorage.Get(ctx, id); err != nil {
		return []byte{}, fmt.Errorf("wrapped storage error: %w", err)
	}
	return data, nil
}

// Initialize implements SecretStorage. It is a noop.
func (s *NotifyingSecretStorage) Initialize(ctx context.Context) error {
	return nil
}

// Store implements SecretStorage
func (s *NotifyingSecretStorage) Store(ctx context.Context, id secretstorage.SecretID, data []byte) error {
	if err := s.SecretStorage.Store(ctx, id, data); err != nil {
		return fmt.Errorf("wrapped storage error: %w", err)
	}

	return s.createDataUpdate(ctx, id)
}

func (s *NotifyingSecretStorage) createDataUpdate(ctx context.Context, id secretstorage.SecretID) error {
	update := &api.SPIAccessTokenDataUpdate{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "data-update-",
			Namespace:    id.Namespace,
		},
		Spec: api.SPIAccessTokenDataUpdateSpec{
			DataOwner: corev1.TypedLocalObjectReference{
				APIGroup: &s.Group,
				Kind:     s.Kind,
				Name:     id.Name,
			},
		},
	}

	cl, err := s.ClientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create the k8s client to use: %w", err)
	}

	err = cl.Create(ctx, update)
	if err != nil {
		return fmt.Errorf("error creating data update: %w", err)
	}
	return nil
}
