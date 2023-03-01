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

	"github.com/cenkalti/backoff/v4"
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

// Dependents represent the secret and the list of the servicea accounts that are linked to a binding.
type Dependents struct {
	Secret          *corev1.Secret
	ServiceAccounts []*corev1.ServiceAccount
}

type serviceAccountLink struct {
	secret          bool
	imagePullSecret bool
}

type serviceAccountNamesAndLinkTypes map[string]serviceAccountLink

// CheckPoint is an opaque struct representing the state of the dependent objects at some point in time.
// It can be used in the DependentsHandler.RevertTo method to delete the secret/service accounts from the cluster
// that have been created after an instance of this struct has been returned from the DependentsHandler.CheckPoint
// method.
type CheckPoint struct {
	secretName          string
	serviceAccountNames serviceAccountNamesAndLinkTypes
}

// CheckPoint creates an instance of CheckPoint struct that captures the secret name and the list of known service account
// names from the Binding associated with the DependentsHandler. This can later be used to revert back to that state again.
// See RevertTo for more details.
func (d *DependentsHandler) CheckPoint(ctx context.Context) (CheckPoint, error) {
	secretName := d.Binding.Status.SyncedObjectRef.Name
	names := make(map[string]serviceAccountLink, len(d.Binding.Status.ServiceAccountNames))
	for _, n := range d.Binding.Status.ServiceAccountNames {
		link, err := d.detectLinks(ctx, secretName, n)
		if err != nil {
			return CheckPoint{}, err
		}
		names[n] = link
	}

	return CheckPoint{
		secretName:          secretName,
		serviceAccountNames: names,
	}, nil
}

func (d *DependentsHandler) detectLinks(ctx context.Context, secretName string, saName string) (serviceAccountLink, error) {
	link := serviceAccountLink{}

	sa := &corev1.ServiceAccount{}
	// we can ignore the error here, because this is essentially the same as the SA having no links. Sync will create it, if needed,
	// and RevertTo will ignore it, because it doesn't exist (it only works with the SAs actually present in the cluster).
	_ = d.Client.Get(ctx, client.ObjectKey{Name: saName, Namespace: d.Binding.Namespace}, sa)

	for _, s := range sa.Secrets {
		if s.Name == secretName {
			link.secret = true
			break
		}
	}

	for _, is := range sa.ImagePullSecrets {
		if is.Name == secretName {
			link.imagePullSecret = true
			break
		}
	}

	return link, nil
}

func (d *DependentsHandler) Sync(ctx context.Context, token *api.SPIAccessToken, sp serviceprovider.ServiceProvider) (*Dependents, api.SPIAccessTokenBindingErrorReason, error) {

	// syncing the service accounts and secrets is a 3 step process.
	// First, an empty service account needs to be created.
	// Second, a secret linking to the service account needs to be created.
	// Third, the service account needs to be updated with the link to the secret.

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
// Note that currently this method is only able to delete secrets/service accounts that should not be in the cluster. It cannot "undelete"
// what has been deleted from the cluster. That should be OK though because we don't delete stuff during the Sync call.
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
		attempt := func() (client.Object, error) {
			needsUpdate := false

			if saLink, ok := checkPoint.serviceAccountNames[sa.Name]; ok {
				// ok, the SA is in the list of the service accounts to revert to. The only thing we need to make sure is that
				// it links to the "old" secret name and doesn't link to the new secret name.
				// We also re-label it if needed.
				if checkPoint.secretName != d.Binding.Status.SyncedObjectRef.Name {
					needsUpdate = serviceAccountHandler.unlinkSecretByName(d.Binding.Status.SyncedObjectRef.Name, sa)
				}

				if saLink.secret {
					needsUpdate = serviceAccountHandler.linkSecretByName(sa, checkPoint.secretName, api.ServiceAccountLinkTypeSecret) || needsUpdate
				}

				if saLink.imagePullSecret {
					needsUpdate = serviceAccountHandler.linkSecretByName(sa, checkPoint.secretName, api.ServiceAccountLinkTypeImagePullSecret) || needsUpdate
				}
			} else {
				// so the SA is marked as linked to the binding in the cluster, but it is not in the list of SAs in the state we are reverting to.
				// if the SA is managed, we can just delete it, but if it is not managed, we need to make sure the linking of secrets is accurate.

				if sa.Labels[ManagedByLabel] == d.Binding.Name {
					if err := d.Client.Delete(ctx, sa); err != nil {
						return nil, backoff.Permanent(fmt.Errorf("failed to delete obsolete service account %s: %w", sa.Name, err)) //nolint:wrapcheck // this is just signalling to backoff.. will not bubble up.
					}
					// we don't need to do anything more on the SA because we just deleted it :)
					return nil, nil
				}

				serviceAccountHandler.unlinkSecretByName(d.Binding.Status.SyncedObjectRef.Name, sa)
				serviceAccountHandler.unlinkSecretByName(checkPoint.secretName, sa)

				// the SA should not be linked to the binding, so let's remove the annotation...
				delete(sa.Annotations, LinkAnnotation)

				needsUpdate = true
			}

			if needsUpdate {
				return sa, nil
			}

			return nil, nil
		}

		err = updateWithRetries(serviceAccountUpdateRetryCount, ctx, d.Client, attempt, "retry to update the SA while reverting to previous state of secret links", "failed to update the service account")

		if err != nil {
			return fmt.Errorf("failed to update the service account %s to revert it to prior state while recovering from failed binding reconciliation: %w", sa.Name, err)
		}
	}

	return nil
}

// childHandlers is a utility function instantiating the auxilliary handlers for secrets and service accounts.
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
