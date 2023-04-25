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

package namespacetarget

import (
	"context"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/commaseparated"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LinkedByRemoteSecretLabel          = "spi.appstudio.redhat.com/linked-by-remote-secret" //#nosec G101 -- false positive, this is just a label
	ManagingRemoteSecretNameAnnotation = "spi.appstudio.redhat.com/managing-remote-secret"  //#nosec G101 -- false positive, this is just a label
	LinkedRemoteSecretsAnnotation      = "spi.appstudio.redhat.com/linked-remote-secrets"   //#nosec G101 -- false positive, this is just a label
)

type NamespaceObjectMarker struct {
}

var _ bindings.ObjectMarker = (*NamespaceObjectMarker)(nil)

// IsManaged implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) IsManagedBy(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	refed, _ := m.IsReferencedBy(ctx, rs, obj)
	return refed && annos[ManagingRemoteSecretNameAnnotation] == rs.String(), nil
}

// IsReferenced implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) IsReferencedBy(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	labels := obj.GetLabels()

	if labels[LinkedByRemoteSecretLabel] != "true" {
		return false, nil
	}

	return commaseparated.Value(annos[LinkedRemoteSecretsAnnotation]).Contains(rs.String()), nil
}

// ListManagedOptions implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) ListManagedOptions(ctx context.Context, rs client.ObjectKey) ([]client.ListOption, error) {
	return m.ListReferencedOptions(ctx, rs)
}

// ListReferencedOptions implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) ListReferencedOptions(ctx context.Context, rs client.ObjectKey) ([]client.ListOption, error) {
	return []client.ListOption{
		client.MatchingLabels{
			LinkedByRemoteSecretLabel: "true",
		},
	}, nil
}

// MarkManaged implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) MarkManaged(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	refChanged, _ := m.MarkReferenced(ctx, rs, obj)

	value := rs.String()
	shouldChange := false
	annos := obj.GetAnnotations()

	if annos == nil {
		annos = map[string]string{}
		obj.SetAnnotations(annos)
		shouldChange = true
	} else {
		shouldChange = annos[ManagingRemoteSecretNameAnnotation] != value
	}

	if shouldChange {
		annos[ManagingRemoteSecretNameAnnotation] = value
	}

	return refChanged || shouldChange, nil
}

// MarkReferenced implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) MarkReferenced(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	shouldChange := false
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
		obj.SetLabels(labels)
		shouldChange = true
	} else {
		shouldChange = labels[LinkedByRemoteSecretLabel] != "true"
	}

	labels[LinkedByRemoteSecretLabel] = "true"

	annos := obj.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
		obj.SetAnnotations(annos)
		shouldChange = true
	}

	link := rs.String()

	val := commaseparated.Value(annos[LinkedRemoteSecretsAnnotation])
	shouldChange = !val.Contains(link) || shouldChange

	if shouldChange {
		val.Add(link)
		annos[LinkedRemoteSecretsAnnotation] = val.String()
	}

	return shouldChange, nil
}

// UnmarkManaged implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) UnmarkManaged(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	if annos == nil {
		return false, nil
	}

	val := annos[ManagingRemoteSecretNameAnnotation]

	if val == rs.String() {
		delete(annos, ManagingRemoteSecretNameAnnotation)
		return true, nil
	}

	return false, nil
}

// UnmarkReferenced implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) UnmarkReferenced(ctx context.Context, rs client.ObjectKey, obj client.Object) (bool, error) {
	wasManaged, _ := m.UnmarkManaged(ctx, rs, obj)

	annos := obj.GetAnnotations()
	if annos == nil {
		return wasManaged, nil
	}

	link := rs.String()

	val := commaseparated.Value(annos[LinkedRemoteSecretsAnnotation])
	containsLink := val.Contains(link)

	if containsLink {
		val.Remove(link)
	}

	unlabeled := false
	if val.Len() == 0 {
		labels := obj.GetLabels()
		if labels != nil && labels[LinkedByRemoteSecretLabel] == "true" {
			delete(labels, LinkedByRemoteSecretLabel)
			unlabeled = true
		}
			delete(annos, LinkedRemoteSecretsAnnotation)
	} else {
		annos[LinkedRemoteSecretsAnnotation] = val.String()
	}

	return unlabeled || wasManaged || containsLink, nil
}

// GetReferencingTargets implements bindings.ObjectMarker
func (*NamespaceObjectMarker) GetReferencingTargets(ctx context.Context, obj client.Object) ([]types.NamespacedName, error) {
	val := commaseparated.Value(obj.GetAnnotations()[LinkedRemoteSecretsAnnotation])

	ret := make([]types.NamespacedName, val.Len())

	for i, v := range val.Values() {
		names := strings.Split(v, string(types.Separator))
		ret[i].Name = names[1]
		ret[i].Namespace = names[0]
	}

	return ret, nil
}
