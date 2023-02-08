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

package bindings

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// pre-allocated empty map so that we don't have to allocate new empty instances in the serviceAccountSecretDiffOpts
	emptySecretData = map[string][]byte{}

	secretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}

	// the service account secrets are treated specially by Kubernetes that automatically adds "ca.crt", "namespace" and
	// "token" entries into the secret's data.
	serviceAccountSecretDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
		cmp.FilterPath(func(p cmp.Path) bool {
			return p.Last().String() == ".Data"
		}, cmp.Comparer(func(a map[string][]byte, b map[string][]byte) bool {
			// cmp.Equal short-circuits if it sees nil maps - but we don't want that...
			if a == nil {
				a = emptySecretData
			}
			if b == nil {
				b = emptySecretData
			}

			return cmp.Equal(a, b, cmpopts.IgnoreMapEntries(func(key string, _ []byte) bool {
				switch key {
				case "ca.crt", "namespace", "token":
					return true
				default:
					return false
				}
			}))
		}),
		),
	}
)

type secretHandler struct {
	Binding      *api.SPIAccessTokenBinding
	Client       client.Client
	TokenStorage tokenstorage.TokenStorage
}

func (h *secretHandler) Sync(ctx context.Context, tokenObject *api.SPIAccessToken, sp serviceprovider.ServiceProvider) (*corev1.Secret, api.SPIAccessTokenBindingErrorReason, error) {
	token, err := h.TokenStorage.Get(ctx, tokenObject)
	if err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, fmt.Errorf("failed to get the token data from token storage: %w", err)
	}

	if token == nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenRetrieval, AccessTokenDataNotFoundError
	}

	at, err := sp.MapToken(ctx, h.Binding, tokenObject, token)
	if err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenAnalysis, fmt.Errorf("failed to analyze the token to produce the mapping to the secret: %w", err)
	}

	stringData, err := at.ToSecretType(h.Binding.Spec.Secret.Type, &h.Binding.Spec.Secret.Fields)
	if err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenAnalysis, fmt.Errorf("failed to create data to be injected into the secret: %w", err)
	}

	// copy the string data into the byte-array data so that sync works reliably. If we didn't sync, we could have just
	// used the Secret.StringData, but Sync gives us other goodies.
	// So let's bite the bullet and convert manually here.
	data := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		data[k] = []byte(v)
	}

	secretName := h.Binding.Status.SyncedObjectRef.Name
	if secretName == "" {
		secretName = h.Binding.Spec.Secret.Name
	}

	annos := h.Binding.Spec.Secret.Annotations

	diffOpts := secretDiffOpts

	if h.Binding.Spec.Secret.Type == corev1.SecretTypeServiceAccountToken {
		diffOpts = serviceAccountSecretDiffOpts
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   h.Binding.GetNamespace(),
			Labels:      h.Binding.Spec.Secret.Labels,
			Annotations: annos,
		},
		Data: data,
		Type: h.Binding.Spec.Secret.Type,
	}

	if secret.Name == "" {
		secret.GenerateName = h.Binding.Name + "-secret-"
	}

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}

	secret.Labels[ManagedByLabel] = h.Binding.Name

	syncer := sync.New(h.Client)

	_, obj, err := syncer.Sync(ctx, nil, secret, diffOpts)
	if err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenSync, fmt.Errorf("failed to sync the secret with the token data: %w", err)
	}
	return obj.(*corev1.Secret), api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func (h *secretHandler) List(ctx context.Context) ([]*corev1.Secret, error) {
	sl := &corev1.SecretList{}
	if err := h.Client.List(ctx, sl, client.MatchingLabels{ManagedByLabel: h.Binding.Name}, client.InNamespace(h.Binding.Namespace)); err != nil {
		return []*corev1.Secret{}, fmt.Errorf("failed to list the secrets associated with the binding %+v: %w", client.ObjectKeyFromObject(h.Binding), err)
	}

	ret := []*corev1.Secret{}
	for i := range sl.Items {
		ret = append(ret, &sl.Items[i])
	}

	return ret, nil
}
