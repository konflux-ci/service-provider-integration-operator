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

package bindingtarget

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/commaseparated"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BindingTargetObjectMarker struct {
	Client  client.Client
	Binding *api.SPIAccessTokenBinding
}

// LinkAnnotation is used to associate the binding to the service account (even the referenced service accounts get annotated by this so that we can clean up their secret lists when the binding is deleted).
const LinkAnnotation = "spi.appstudio.redhat.com/linked-access-token-binding" //#nosec G101 -- false positive, this is just a label

// ManagedByBindingLabel marks the other objects as managed by SPI. Meaning that their lifecycle is bound
// to the lifecycle of some SPI binding.
const ManagedByBindingLabel = "spi.appstudio.redhat.com/managed-by-binding"

var _ bindings.ObjectMarker = (*BindingTargetObjectMarker)(nil)

// IsManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) IsManaged(ctx context.Context, obj client.Object) (bool, error) {
	refed, _ := m.IsReferenced(ctx, obj)
	return refed && obj.GetLabels()[ManagedByBindingLabel] == m.Binding.Name, nil
}

// IsReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) IsReferenced(ctx context.Context, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	if len(annos) == 0 {
		return false, nil
	}
	anno := annos[LinkAnnotation]
	return commaseparated.Value(anno).Contains(m.Binding.Name), nil
}

// ListManagedOptions implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) ListManagedOptions(ctx context.Context) ([]client.ListOption, error) {
	return []client.ListOption{client.MatchingLabels{ManagedByBindingLabel: m.Binding.Name}}, nil
}

// ListReferencedOptions implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) ListReferencedOptions(ctx context.Context) ([]client.ListOption, error) {
	// the link is represented using an annotation, so we cannot produce any list options
	return []client.ListOption{}, nil
}

// MarkManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) MarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	changed, _ := m.MarkReferenced(ctx, obj)

	labels := obj.GetLabels()

	val := labels[ManagedByBindingLabel]

	if val != m.Binding.Name {
		changed = true
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ManagedByBindingLabel] = m.Binding.Name
		obj.SetLabels(labels)
	}

	return changed, nil
}

// MarkReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) MarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	changed := false
	annos := obj.GetAnnotations()

	val := commaseparated.Value(annos[LinkAnnotation])

	if !val.Contains(m.Binding.Name) {
		changed = true
		if annos == nil {
			annos = map[string]string{}
		}
		val.Add(m.Binding.Name)
		annos[LinkAnnotation] = val.String()
		obj.SetAnnotations(annos)
	}

	return changed, nil
}

// IsManagedByOther implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) IsManagedByOther(ctx context.Context, obj client.Object) (bool, error) {
	managedBy := obj.GetLabels()[ManagedByBindingLabel]
	return managedBy != m.Binding.Name, nil
}

// UnmarkManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) UnmarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	_, contains := obj.GetLabels()[ManagedByBindingLabel]
	delete(obj.GetLabels(), ManagedByBindingLabel)
	return contains, nil
}

// UnmarkReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) UnmarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	_, contains := obj.GetAnnotations()[LinkAnnotation]
	delete(obj.GetAnnotations(), LinkAnnotation)
	return contains, nil
}
