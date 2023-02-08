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
	specInconsistentWithStatusError    = stderrors.New("the number of service accounts in the spec doesn't correspond to the number of found service accounts")
	managedServiceAccountAlreadyExists = stderrors.New("a service account with same name as the managed one already exists")
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
		saSpec := link.ServiceAccount
		sa := serviceAccounts[i]
		updated := false
		hasLink := false
		linkType := saSpec.EffectiveSecretLinkType()

		if linkType == api.ServiceAccountLinkTypeSecret {
			for _, r := range sa.Secrets {
				if r.Name == secret.Name {
					hasLink = true
					break
				}
			}

			if !hasLink {
				sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: secret.Name})
				updated = true
			}
		} else if linkType == api.ServiceAccountLinkTypeImagePullSecret {
			for _, r := range sa.ImagePullSecrets {
				if r.Name == secret.Name {
					hasLink = true
					break
				}
			}

			if !hasLink {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: secret.Name})
				updated = true
			}
		}

		if updated {
			if err := h.Client.Update(ctx, sa); err != nil {
				return fmt.Errorf("failed to update the service account '%s' with the link to the secret '%s' while processing binding '%s': %w", sa.Name, secret.Name, client.ObjectKeyFromObject(h.Binding), err)
			}
		}
	}

	return nil
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
	updated := false

	if len(serviceAccount.Secrets) > 0 {
		saSecrets := make([]corev1.ObjectReference, 0, len(serviceAccount.Secrets))
		for i := range serviceAccount.Secrets {
			r := serviceAccount.Secrets[i]
			if r.Name == secret.Name {
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
			if r.Name == secret.Name {
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
	if err := h.Client.Get(ctx, client.ObjectKey{Name: spec.Name, Namespace: h.Binding.Namespace}, sa); err != nil {
		return nil, api.SPIAccessTokenBindingErrorReasonServiceAccountUnavailable, fmt.Errorf("failed to get the referenced service account exists: %w", err)
	}

	origAnno := sa.Annotations[LinkAnnotation]
	linkAnno := ensureBindingNameInAnnotation(origAnno, h.Binding.Name)
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	sa.Annotations[LinkAnnotation] = linkAnno

	if origAnno != linkAnno {
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
		// the service account already exists. We need to make sure that it belongs to this binding, otherwise we
		// error out because we found a pre-existing SA that should be managed
		if !BindingNameInAnnotation(sa.Annotations[LinkAnnotation], h.Binding.Name) {
			err = managedServiceAccountAlreadyExists
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
