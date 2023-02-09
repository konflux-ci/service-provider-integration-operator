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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DependentsHandler is taking care of the dependent objects of the binding
type DependentsHandler struct {
	Client       client.Client
	TokenStorage tokenstorage.TokenStorage
	Binding      *api.SPIAccessTokenBinding
}

type Dependents struct {
	Secret          *corev1.Secret
	ServiceAccounts []*corev1.ServiceAccount
}

type CheckPoint struct {
	secretName          string
	serviceAccountNames []string
}

func (d *DependentsHandler) CheckPoint() CheckPoint {
	names := make([]string, 0, len(d.Binding.Status.ServiceAccountNames))
	copy(names, d.Binding.Status.ServiceAccountNames)

	return CheckPoint{
		secretName:          d.Binding.Status.SyncedObjectRef.Name,
		serviceAccountNames: names,
	}
}

func (d *DependentsHandler) Sync(ctx context.Context, token *api.SPIAccessToken, sp serviceprovider.ServiceProvider) (*Dependents, api.SPIAccessTokenBindingErrorReason, error) {

	// syncing the service accounts and secrets is a 3 step process.
	// First, an empty service account needs to be created.
	// Second, a secret linking to the service account needs to be created.
	// Third, the service account needs to be updated with the link to the secret.

	// TODO this can leave the SA behind if we subsequently fail to update the status of the binding

	secretsHandler, saHandler := d.childHandlers()

	serviceAccounts, errorReason, err := saHandler.Sync(ctx)
	if err != nil {
		return nil, errorReason, err
	}

	sec, errorReason, err := secretsHandler.Sync(ctx, token, sp)
	if err != nil {
		return nil, errorReason, err
	}

	if err = saHandler.LinkToSecret(ctx, serviceAccounts, sec); err != nil {
		return nil, errorReason, err
	}

	deps := &Dependents{
		Secret:          sec,
		ServiceAccounts: serviceAccounts,
	}

	return deps, api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func (d *DependentsHandler) Cleanup(ctx context.Context) error {
	secretsHandler, saHandler := d.childHandlers()

	sal, err := saHandler.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list the service accounts to clean up because of binding %s: %w", client.ObjectKeyFromObject(d.Binding), err)
	}

	sl, err := secretsHandler.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list the secrets to clean up because of binding %s: %w", client.ObjectKeyFromObject(d.Binding), err)
	}

	for _, sa := range sal {
		if sa.Labels[ManagedByLabel] == d.Binding.Name {
			if err := d.Client.Delete(ctx, sa); err != nil {
				return fmt.Errorf("failed to delete the managed service account %s while cleaning up dependent objects of binding %s: %w", client.ObjectKeyFromObject(sa), client.ObjectKeyFromObject(d.Binding), err)
			}
		} else {
			persist := false
			for _, s := range sl {
				// Unlink must go first, because Go only has lazy bool eval
				persist = saHandler.Unlink(s, sa) || persist
			}
			if persist {
				if err := d.Client.Update(ctx, sa); err != nil {
					return fmt.Errorf("failed to remove the linked secrets from the service account %s while cleaning up dependent objects of binding %s: %w", client.ObjectKeyFromObject(sa), client.ObjectKeyFromObject(d.Binding), err)
				}
			}
		}
	}

	for _, s := range sl {
		if err := d.Client.Delete(ctx, s); err != nil {
			return fmt.Errorf("failed to delete the secret %s while cleaning up dependent objects of binding %s: %w", client.ObjectKeyFromObject(s), client.ObjectKeyFromObject(d.Binding), err)
		}
	}

	return nil
}

// RevertTo reverts the reconciliation "transaction". I.e. this should be called after Sync in case the subsequent steps in the reconciliation
// fail and the operator needs to revert the changes made in sync so that the changes remain idempontent. The provided checkpoint represents
// the state obtained from the DependentsHandler.Binding prior to making any changes by Sync().
func (d *DependentsHandler) RevertTo(ctx context.Context, checkPoint CheckPoint) error {
	secretHandler, serviceAccountHandler := d.childHandlers()

	sl, err := secretHandler.List(ctx)
	if err != nil {
		return err
	}
	for _, s := range sl {
		if s.Name != checkPoint.secretName {
			if err := d.Client.Delete(ctx, s); err != nil {
				return fmt.Errorf("failed to delete obsolete synced secret %s: %w", s.Name, err)
			}
		}
	}

	sal, err := serviceAccountHandler.List(ctx)
	if err != nil {
		return err
	}
	for _, sa := range sal {
		if !containsName(checkPoint.serviceAccountNames, sa.Name) {
			if err := d.Client.Delete(ctx, sa); err != nil {
				return fmt.Errorf("failed to delete obsolete service account %s: %w", sa.Name, err)
			}
		}
	}

	return nil
}

func (d *DependentsHandler) childHandlers() (*secretHandler, *serviceAccountHandler) {
	secretsHandler := &secretHandler{
		Binding:      d.Binding,
		Client:       d.Client,
		TokenStorage: d.TokenStorage,
	}

	saHandler := &serviceAccountHandler{
		Binding: d.Binding,
		Client:  d.Client,
	}

	return secretsHandler, saHandler
}

func containsName(list []string, name string) bool {
	for _, o := range list {
		if o == name {
			return true
		}
	}

	return false
}
