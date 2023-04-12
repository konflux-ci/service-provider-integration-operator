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
	stderrors "errors"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	specInconsistentWithStatusError              = stderrors.New("the number of service accounts in the spec doesn't correspond to the number of found service accounts")
	managedServiceAccountAlreadyExists           = stderrors.New("a service account with same name as the managed one already exists")
	managedServiceAccountManagedByAnotherBinding = stderrors.New("the service account already exists and is managed by another binding object")
)

const (
	// serviceAccountUpdateRetryCount is the number of times we retry update operations on a service account. This a completely arbitrary number that is bigger
	// than 2. We need this because OpenShift automagically updates service accounts with dockerconfig secrets, etc, and so the service account change change
	// underneath our hands basically immediatelly after creation.
	serviceAccountUpdateRetryCount = 10
)

type serviceAccountHandler struct {
	Target       SecretDeploymentTarget
	ObjectMarker ObjectMarker
}

func (h *serviceAccountHandler) Sync(ctx context.Context) ([]*corev1.ServiceAccount, string, error) {
	sas := []*corev1.ServiceAccount{}

	for i, link := range h.Target.GetSpec().LinkedTo {
		sa, errorReason, err := h.ensureServiceAccount(ctx, i, &link.ServiceAccount)
		if err != nil {
			return []*corev1.ServiceAccount{}, errorReason, err
		}
		if sa != nil {
			sas = append(sas, sa)
		}
	}

	return sas, "", nil
}

func (h *serviceAccountHandler) LinkToSecret(ctx context.Context, serviceAccounts []*corev1.ServiceAccount, secret *corev1.Secret) error {
	if len(h.Target.GetSpec().LinkedTo) != len(serviceAccounts) {
		return specInconsistentWithStatusError
	}

	for i, link := range h.Target.GetSpec().LinkedTo {
		sa := serviceAccounts[i]
		linkType := link.ServiceAccount.EffectiveSecretLinkType()

		// we first try with the state of the service account as is, but because service accounts are treated somewhat specially at least in OpenShift
		// the environment might be making updates to them under our hands. So let's have a couple of retries here so that we don't have to retry until
		// "everything" (our updates and OpenShift udates to the SA) clicks just in the right order.
		//
		// Note that this SHOULD do at most 1 retry, but let's try a little harder than that to allow for multiple out-of-process concurrent updates
		// on the SA.
		attempt := func() (client.Object, error) {
			if h.linkSecretByName(sa, secret.Name, linkType) {
				return sa, nil
			}
			// no update needed
			return nil, nil
		}

		err := updateWithRetries(serviceAccountUpdateRetryCount, ctx, h.Target.GetClient(), attempt, "retrying SA secret linking update due to conflict",
			fmt.Sprintf("failed to update the service account '%s' with the link to the secret '%s' while processing the deployment target (%s) '%s'", sa.Name, secret.Name, h.Target.GetType(), h.Target.GetTargetObjectKey()))

		if err != nil {
			return fmt.Errorf("failed to link the secret %s to the service account %s while processing the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(secret),
				client.ObjectKeyFromObject(sa),
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				err)
		}
	}

	return nil
}

func (h *serviceAccountHandler) linkSecretByName(sa *corev1.ServiceAccount, secretName string, linkType api.ServiceAccountLinkType) bool {
	updated := false
	hasLink := false

	if linkType == api.ServiceAccountLinkTypeSecret {
		for _, r := range sa.Secrets {
			if r.Name == secretName {
				hasLink = true
				break
			}
		}

		if !hasLink {
			sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: secretName})
			updated = true
		}
	} else if linkType == api.ServiceAccountLinkTypeImagePullSecret {
		for _, r := range sa.ImagePullSecrets {
			if r.Name == secretName {
				hasLink = true
				break
			}
		}

		if !hasLink {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
			updated = true
		}
	}

	return updated
}

func (h *serviceAccountHandler) List(ctx context.Context) ([]*corev1.ServiceAccount, error) {
	sal := &corev1.ServiceAccountList{}

	opts, err := h.ObjectMarker.ListReferencedOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to construct list options: %w", err)
	}

	opts = append(opts, client.InNamespace(h.Target.GetTargetNamespace()))
	// unlike secrets that are always exclusive to the binding, there can be more service accounts
	// associated with the binding. Therefore we need to manually filter them.
	if err := h.Target.GetClient().List(ctx, sal, opts...); err != nil {
		return []*corev1.ServiceAccount{}, fmt.Errorf("failed to list the service accounts in the namespace '%s' while processing the deployment target (%s) %s: %w",
			h.Target.GetTargetNamespace(),
			h.Target.GetType(),
			h.Target.GetTargetObjectKey(),
			err)
	}

	ret := []*corev1.ServiceAccount{}

	for i := range sal.Items {
		sa := sal.Items[i]
		if ok, err := h.ObjectMarker.IsReferenced(ctx, &sa); err != nil {
			return []*corev1.ServiceAccount{}, fmt.Errorf("failed to determine if the service account %s is referenced while processing the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(&sa),
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				err)
		} else if ok {
			ret = append(ret, &sa)
		}
	}

	return ret, nil
}

// Unlink removes the provided secret from any links in the service account (either secrets or image pull secrets fields).
// Returns `true` if the service account object was changed, `false` otherwise. This does not update the object in the cluster!
func (h *serviceAccountHandler) Unlink(secret *corev1.Secret, serviceAccount *corev1.ServiceAccount) bool {
	return h.unlinkSecretByName(secret.Name, serviceAccount)
}

func (h *serviceAccountHandler) unlinkSecretByName(secretName string, serviceAccount *corev1.ServiceAccount) bool {
	updated := false

	if len(serviceAccount.Secrets) > 0 {
		saSecrets := make([]corev1.ObjectReference, 0, len(serviceAccount.Secrets))
		for i := range serviceAccount.Secrets {
			r := serviceAccount.Secrets[i]
			if r.Name == secretName {
				updated = true
			} else {
				saSecrets = append(saSecrets, r)
			}
		}
		serviceAccount.Secrets = saSecrets
	}

	if len(serviceAccount.ImagePullSecrets) > 0 {
		saIPSecrets := make([]corev1.LocalObjectReference, 0, len(serviceAccount.ImagePullSecrets))
		for i := range serviceAccount.ImagePullSecrets {
			r := serviceAccount.ImagePullSecrets[i]
			if r.Name == secretName {
				updated = true
			} else {
				saIPSecrets = append(saIPSecrets, r)
			}
		}
		serviceAccount.ImagePullSecrets = saIPSecrets
	}

	return updated
}

// ensureServiceAccount loads the service account configured in the binding from the cluster or creates a new one if needed.
// It also makes sure that the service account is correctly labeled.
func (h *serviceAccountHandler) ensureServiceAccount(ctx context.Context, specIdx int, spec *api.ServiceAccountLink) (*corev1.ServiceAccount, string, error) {

	if spec.Reference.Name != "" {
		return h.ensureReferencedServiceAccount(ctx, &spec.Reference)
	} else if spec.Managed.Name != "" || spec.Managed.GenerateName != "" {
		return h.ensureManagedServiceAccount(ctx, specIdx, &spec.Managed)
	}

	return nil, "", nil
}

func (h *serviceAccountHandler) ensureReferencedServiceAccount(ctx context.Context, spec *corev1.LocalObjectReference) (*corev1.ServiceAccount, string, error) {
	sa := &corev1.ServiceAccount{}
	key := client.ObjectKey{Name: spec.Name, Namespace: h.Target.GetTargetNamespace()}
	if err := h.Target.GetClient().Get(ctx, key, sa); err != nil {
		return nil, string(ErrorReasonServiceAccountUnavailable), fmt.Errorf("failed to get the referenced service account (%s): %w", key, err)
	}

	// Only delete the managed binding label from the SA if it is our binding that was previously managing it.
	// This makes sure that if there are multiple bindings associated with an SA that is also managed by one of them
	// We don't end up in a ping-pong situation where the bindings reconcilers would "fight" over the value of this
	// annotation.
	managedChanged := false
	if managed, err := h.ObjectMarker.IsManaged(ctx, sa); err != nil {
		return nil, string(ErrorReasonServiceAccountUpdate), fmt.Errorf("failed to determine if the service account (%s) is managed while making it just referenced when processing the deployment target (%s) %s: %w",
			key,
			h.Target.GetType(),
			h.Target.GetTargetObjectKey(),
			err)
	} else if managed {
		if _, err := h.ObjectMarker.UnmarkManaged(ctx, sa); err != nil {
			return nil, string(ErrorReasonServiceAccountUpdate), fmt.Errorf("failed to remove the managed mark from the service account (%s) while making it just referenced when processing the deployment target (%s) %s: %w",
				key,
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				err)
		}
		managedChanged = true
	}
	changed, err := h.ObjectMarker.MarkReferenced(ctx, sa)
	if err != nil {
		return nil, string(ErrorReasonServiceAccountUpdate),
			fmt.Errorf("failed to mark the service account (%s) as referenced when processing the deployment target (%s) %s: %w",
				key,
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				err)
	}

	if changed || managedChanged {
		// we need to update the service account with the new link to the binding
		if err := h.Target.GetClient().Update(ctx, sa); err != nil {
			return nil, string(ErrorReasonServiceAccountUpdate), fmt.Errorf("failed to update the annotations in the referenced service account %s while processing the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(sa),
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				err)
		}
	}

	return sa, "", nil
}

func (h *serviceAccountHandler) ensureManagedServiceAccount(ctx context.Context, specIdx int, spec *api.ManagedServiceAccountSpec) (*corev1.ServiceAccount, string, error) {
	var name string
	actualSANames := h.Target.GetActualServiceAccountNames()
	if len(actualSANames) > specIdx {
		name = actualSANames[specIdx]
	}
	if name == "" {
		name = spec.Name
	}

	requestedLabels := map[string]string{}
	for k, v := range spec.Labels {
		requestedLabels[k] = v
	}

	requestedAnnotations := map[string]string{}
	for k, v := range spec.Annotations {
		requestedAnnotations[k] = v
	}

	var err error
	sa := &corev1.ServiceAccount{}

	if err = h.Target.GetClient().Get(ctx, client.ObjectKey{Name: name, Namespace: h.Target.GetTargetNamespace()}, sa); err != nil {
		if errors.IsNotFound(err) {
			sa = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:         name,
					Namespace:    h.Target.GetTargetNamespace(),
					GenerateName: spec.GenerateName,
				},
			}
			_, err = h.ObjectMarker.MarkManaged(ctx, sa)
			if err == nil {
				err = h.Target.GetClient().Create(ctx, sa)
			}
		}
	} else {
		// The service account already exists. We need to make sure that it is associated with this binding,
		// otherwise we error out because we found a pre-existing SA that should be managed.
		// Note that the managed SAs always also have the link annotation pointing to the managing binding.
		if ok, rerr := h.ObjectMarker.IsReferenced(ctx, sa); rerr != nil {
			err = fmt.Errorf("failed to determine if service account %s is referenced by the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(sa),
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				rerr)
		} else if !ok {
			err = managedServiceAccountAlreadyExists
		}

		// we also check that if the SA is managed, it is managed by us and not by some other binding.
		if ok, merr := h.ObjectMarker.IsManagedByOther(ctx, sa); merr != nil {
			err = fmt.Errorf("failed to determine if service account %s is managed by the deployment target (%s) %s: %w",
				client.ObjectKeyFromObject(sa),
				h.Target.GetType(),
				h.Target.GetTargetObjectKey(),
				merr)
		} else if ok {
			err = managedServiceAccountManagedByAnotherBinding
		}
	}

	// make sure all our labels have the values we want
	if err == nil {
		needsUpdate := false

		if sa.Labels == nil {
			sa.Labels = map[string]string{}
		}

		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}

		for k, rv := range requestedLabels {
			v, ok := sa.Labels[k]
			if !ok || v != rv {
				needsUpdate = true
			}
			sa.Labels[k] = rv
		}

		for k, rv := range requestedAnnotations {
			v, ok := sa.Annotations[k]
			if !ok || v != rv {
				needsUpdate = true
			}
			sa.Annotations[k] = rv
		}

		var changed bool
		changed, err = h.ObjectMarker.MarkManaged(ctx, sa)
		if err != nil {
			return nil, string(ErrorReasonServiceAccountUpdate),
				fmt.Errorf("failed to sync the configured service account %s to the deployment target (%s) %s: %w",
					client.ObjectKeyFromObject(sa),
					h.Target.GetType(),
					h.Target.GetTargetObjectKey(),
					err)
		}
		needsUpdate = needsUpdate || changed

		if needsUpdate {
			err = h.Target.GetClient().Update(ctx, sa)
		}
	}

	if err != nil {
		return nil, string(ErrorReasonServiceAccountUpdate), fmt.Errorf("failed to sync the configured service account of the deployment target: %w", err)
	}

	return sa, "", nil
}
