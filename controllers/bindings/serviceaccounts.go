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
	"strings"

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
	Binding *api.SPIAccessTokenBinding
	Client  client.Client
}

func (h *serviceAccountHandler) Sync(ctx context.Context) ([]*corev1.ServiceAccount, api.SPIAccessTokenBindingErrorReason, error) {
	sas := []*corev1.ServiceAccount{}

	for i, link := range h.Binding.Spec.Secret.LinkedTo {
		sa, errorReason, err := h.ensureServiceAccount(ctx, i, &link.ServiceAccount)
		if err != nil {
			return []*corev1.ServiceAccount{}, errorReason, err
		}
		if sa != nil {
			sas = append(sas, sa)
		}
	}

	return sas, api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func (h *serviceAccountHandler) LinkToSecret(ctx context.Context, serviceAccounts []*corev1.ServiceAccount, secret *corev1.Secret) error {
	if len(h.Binding.Spec.Secret.LinkedTo) != len(serviceAccounts) {
		return specInconsistentWithStatusError
	}

	for i, link := range h.Binding.Spec.Secret.LinkedTo {
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

		err := updateWithRetries(serviceAccountUpdateRetryCount, ctx, h.Client, attempt, "retrying SA secret linking update due to conflict",
			fmt.Sprintf("failed to update the service account '%s' with the link to the secret '%s' while processing binding '%s'", sa.Name, secret.Name, client.ObjectKeyFromObject(h.Binding)))

		if err != nil {
			return fmt.Errorf("failed to link the secret %s to the service account %s: %w", client.ObjectKeyFromObject(secret), client.ObjectKeyFromObject(sa), err)
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

	// unlike secrets that are always exclusive to the binding, there can be more service accounts
	// associated with the binding. Therefore we need to manually filter them.
	if err := h.Client.List(ctx, sal, client.InNamespace(h.Binding.Namespace)); err != nil {
		return []*corev1.ServiceAccount{}, fmt.Errorf("failed to list the service accounts in the namespace '%s': %w", h.Binding.Namespace, err)
	}

	ret := []*corev1.ServiceAccount{}

	for i := range sal.Items {
		sa := sal.Items[i]
		if BindingNameInAnnotation(sa.Annotations[LinkAnnotation], h.Binding.Name) {
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
func (h *serviceAccountHandler) ensureServiceAccount(ctx context.Context, specIdx int, spec *api.ServiceAccountLink) (*corev1.ServiceAccount, api.SPIAccessTokenBindingErrorReason, error) {

	if spec.Reference.Name != "" {
		return h.ensureReferencedServiceAccount(ctx, &spec.Reference)
	} else if spec.Managed.Name != "" || spec.Managed.GenerateName != "" {
		return h.ensureManagedServiceAccount(ctx, specIdx, &spec.Managed)
	}

	return nil, api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func (h *serviceAccountHandler) ensureReferencedServiceAccount(ctx context.Context, spec *corev1.LocalObjectReference) (*corev1.ServiceAccount, api.SPIAccessTokenBindingErrorReason, error) {
	sa := &corev1.ServiceAccount{}
	key := client.ObjectKey{Name: spec.Name, Namespace: h.Binding.Namespace}
	if err := h.Client.Get(ctx, key, sa); err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonServiceAccountUnavailable, fmt.Errorf("failed to get the referenced service account (%s): %w", key, err)
	}

	origAnno := sa.Annotations[LinkAnnotation]
	linkAnno := ensureBindingNameInAnnotation(origAnno, h.Binding.Name)
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	sa.Annotations[LinkAnnotation] = linkAnno

	// Only delete the managed binding label from the SA if it is our binding that was previously managing it.
	// This makes sure that if there are multiple bindings associated with an SA that is also managed by one of them
	// We don't end up in a ping-pong situation where the bindings reconcilers would "fight" over the value of this
	// annotation.
	managedChanged := false
	managingBinding := sa.Labels[ManagedByLabel]
	if managingBinding == h.Binding.Name {
		managedChanged = true
		delete(sa.Labels, ManagedByLabel)
	}

	if origAnno != linkAnno || managedChanged {
		// we need to update the service account with the new link to the binding
		if err := h.Client.Update(ctx, sa); err != nil {
			return nil, api.SPIAccessTokenBindingErrorReasonServiceAccountUpdate, fmt.Errorf("failed to update the annotations in the referenced service account %s of binding %s: %w", client.ObjectKeyFromObject(sa), client.ObjectKeyFromObject(h.Binding), err)
		}
	}

	return sa, api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func (h *serviceAccountHandler) ensureManagedServiceAccount(ctx context.Context, specIdx int, spec *api.ManagedServiceAccountSpec) (*corev1.ServiceAccount, api.SPIAccessTokenBindingErrorReason, error) {
	var name string
	if len(h.Binding.Status.ServiceAccountNames) > specIdx {
		name = h.Binding.Status.ServiceAccountNames[specIdx]
	}
	if name == "" {
		name = spec.Name
	}

	requestedLabels := map[string]string{}
	for k, v := range spec.Labels {
		requestedLabels[k] = v
	}
	requestedLabels[ManagedByLabel] = h.Binding.Name

	requestedAnnotations := map[string]string{}
	for k, v := range spec.Annotations {
		requestedAnnotations[k] = v
	}
	requestedAnnotations[LinkAnnotation] = h.Binding.Name

	var err error
	sa := &corev1.ServiceAccount{}

	if err = h.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: h.Binding.Namespace}, sa); err != nil {
		if errors.IsNotFound(err) {
			sa = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:         name,
					Namespace:    h.Binding.Namespace,
					GenerateName: spec.GenerateName,
					Annotations:  requestedAnnotations,
					Labels:       requestedLabels,
				},
			}
			err = h.Client.Create(ctx, sa)
		}
	} else {
		// The service account already exists. We need to make sure that it is associated with this binding,
		// otherwise we error out because we found a pre-existing SA that should be managed.
		// Note that the managed SAs always also have the link annotation pointing to the managing binding.
		if !BindingNameInAnnotation(sa.Annotations[LinkAnnotation], h.Binding.Name) {
			err = managedServiceAccountAlreadyExists
		}

		// we also check that if the SA is managed, it is managed by us and not by some other binding.
		managedBy := sa.Labels[ManagedByLabel]
		if err == nil && managedBy != "" && managedBy != h.Binding.Name {
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
		} else {
			linkAnno := ensureBindingNameInAnnotation(sa.Annotations[LinkAnnotation], h.Binding.Name)
			requestedAnnotations[LinkAnnotation] = linkAnno
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

		if needsUpdate {
			err = h.Client.Update(ctx, sa)
		}
	}

	if err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonTokenSync, fmt.Errorf("failed to sync the configured service account of the binding: %w", err)
	}

	return sa, api.SPIAccessTokenBindingErrorReasonNoError, nil
}

func ensureBindingNameInAnnotation(annotationValue string, bindingName string) string {
	for _, v := range strings.Split(annotationValue, ",") {
		if strings.TrimSpace(v) == bindingName {
			return annotationValue
		}
	}

	annotationValue = strings.TrimSpace(annotationValue)
	if annotationValue == "" {
		return bindingName
	} else {
		return annotationValue + "," + bindingName
	}
}

func BindingNameInAnnotation(annotationValue string, bindingName string) bool {
	for _, v := range strings.Split(annotationValue, ",") {
		if strings.TrimSpace(v) == bindingName {
			return true
		}
	}

	return false
}
