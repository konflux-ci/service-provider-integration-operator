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
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

type secretHandler[K any] struct {
	Target        SecretDeploymentTarget
	ObjectMarker  ObjectMarker
	SecretBuilder SecretBuilder[K]
}

func (h *secretHandler[K]) Sync(ctx context.Context, key K, sp serviceprovider.ServiceProvider) (*corev1.Secret, string, error) {
	data, errorReason, err := h.SecretBuilder.GetData(ctx, key)
	if err != nil {
		return nil, errorReason, fmt.Errorf("failed to obtain the secret data: %w", err)
	}

	secretName := h.Target.GetActualSecretName()
	if secretName == "" {
		secretName = h.Target.GetSpec().Name
	}

	diffOpts := secretDiffOpts

	if h.Target.GetSpec().Type == corev1.SecretTypeServiceAccountToken {
		diffOpts = serviceAccountSecretDiffOpts
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         secretName,
			GenerateName: h.Target.GetSpec().GenerateName,
			Namespace:    h.Target.GetTargetNamespace(),
			Labels:       h.Target.GetSpec().Labels,
			Annotations:  h.Target.GetSpec().Annotations,
		},
		Data: data,
		Type: h.Target.GetSpec().Type,
	}

	if secret.GenerateName == "" {
		secret.GenerateName = h.Target.GetName() + "-secret-"
	}

	_, err = h.ObjectMarker.MarkManaged(ctx, secret)
	if err != nil {
		return nil, string(ErrorReasonSecretUpdate), fmt.Errorf("failed to mark the secret as managed in the deployment target (%s): %w", h.Target.GetType(), err)
	}

	syncer := sync.New(h.Target.GetClient())

	lg := log.FromContext(ctx).V(logs.DebugLevel)
	lg.Info("syncing binding secret", "secret", secret, "secretMetadata", &secret.ObjectMeta)

	_, obj, err := syncer.Sync(ctx, nil, secret, diffOpts)
	if err != nil {
		return nil, string(ErrorReasonSecretUpdate), fmt.Errorf("failed to sync the secret with the token data: %w", err)
	}
	return obj.(*corev1.Secret), "", nil
}

func (h *secretHandler[K]) List(ctx context.Context) ([]*corev1.Secret, error) {
	sl := &corev1.SecretList{}
	opts, err := h.ObjectMarker.ListManagedOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to formulate the options to list the secrets in the deployment target (%s): %w", h.Target.GetType(), err)
	}

	opts = append(opts, client.InNamespace(h.Target.GetTargetNamespace()))

	lg := log.FromContext(ctx).V(logs.DebugLevel)
	if err := h.Target.GetClient().List(ctx, sl, opts...); err != nil {
		return []*corev1.Secret{}, fmt.Errorf("failed to list the secrets associated with the deployment target (%s) %+v: %w", h.Target.GetType(), client.ObjectKey{Name: h.Target.GetName(), Namespace: h.Target.GetNamespace()}, err)
	}

	lg.Info("listing secrets managed by target", "targetType", h.Target.GetType(), "name", h.Target.GetName(), "namespace", h.Target.GetNamespace(), "targetNamespace", h.Target.GetTargetNamespace(), "opts", opts, "secretCount", len(sl.Items))

	ret := []*corev1.Secret{}
	for i := range sl.Items {
		if ok, err := h.ObjectMarker.IsManaged(ctx, &sl.Items[i]); err != nil {
			return []*corev1.Secret{}, fmt.Errorf("failed to determine if the secret %s is managed while processing the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(&sl.Items[i]),
				h.Target.GetType(),
				client.ObjectKey{Name: h.Target.GetName(), Namespace: h.Target.GetNamespace()},
				err)
		} else if ok {
			ret = append(ret, &sl.Items[i])
		}
	}

	return ret, nil
}
