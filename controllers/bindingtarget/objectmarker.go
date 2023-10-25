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

	"github.com/redhat-appstudio/remote-secret/controllers/bindings"
	"github.com/redhat-appstudio/remote-secret/pkg/commaseparated"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BindingTargetObjectMarker struct {
}

// LinkAnnotation is used to associate the binding to the service account (even the referenced service accounts get annotated by this so that we can clean up their secret lists when the binding is deleted).
const LinkAnnotation = "spi.appstudio.redhat.com/linked-access-token-binding" //#nosec G101 -- false positive, this is just a label

// ManagedByBindingLabel marks the other objects as managed by SPI. Meaning that their lifecycle is bound
// to the lifecycle of some SPI binding.
const ManagedByBindingLabel = "spi.appstudio.redhat.com/managed-by-binding"

var _ bindings.ObjectMarker = (*BindingTargetObjectMarker)(nil)

// IsManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) IsManagedBy(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	refed, _ := m.IsReferencedBy(ctx, binding, obj)
	return refed && obj.GetLabels()[ManagedByBindingLabel] == binding.Name, nil
}

// IsReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) IsReferencedBy(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	if len(annos) == 0 {
		return false, nil
	}
	anno := annos[LinkAnnotation]
	return commaseparated.Value(anno).Contains(binding.Name), nil
}

// ListManagedOptions implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) ListManagedOptions(ctx context.Context, binding client.ObjectKey) ([]client.ListOption, error) {
	return []client.ListOption{client.MatchingLabels{ManagedByBindingLabel: binding.Name}}, nil
}

// ListReferencedOptions implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) ListReferencedOptions(ctx context.Context, binding client.ObjectKey) ([]client.ListOption, error) {
	// the link is represented using an annotation, so we cannot produce any list options
	return []client.ListOption{}, nil
}

// MarkManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) MarkManaged(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	changed, _ := m.MarkReferenced(ctx, binding, obj)

	labels := obj.GetLabels()

	val := labels[ManagedByBindingLabel]

	if val != binding.Name {
		changed = true
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ManagedByBindingLabel] = binding.Name
		obj.SetLabels(labels)
	}

	return changed, nil
}

// MarkReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) MarkReferenced(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	changed := false
	annos := obj.GetAnnotations()

	val := commaseparated.Value(annos[LinkAnnotation])

	if !val.Contains(binding.Name) {
		changed = true
		if annos == nil {
			annos = map[string]string{}
		}
		val.Add(binding.Name)
		annos[LinkAnnotation] = val.String()
		obj.SetAnnotations(annos)
	}

	return changed, nil
}

// UnmarkManaged implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) UnmarkManaged(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	name, contains := obj.GetLabels()[ManagedByBindingLabel]
	if name == binding.Name {
		delete(obj.GetLabels(), ManagedByBindingLabel)
	}
	return contains, nil
}

// UnmarkReferenced implements dependents.ObjectMarker
func (m *BindingTargetObjectMarker) UnmarkReferenced(ctx context.Context, binding client.ObjectKey, obj client.Object) (bool, error) {
	v, contains := obj.GetAnnotations()[LinkAnnotation]
	if !contains {
		return false, nil
	}

	names := commaseparated.Value(v)
	if names.Contains(binding.Name) {
		names.Remove(binding.Name)
		if names.Len() == 0 {
			delete(obj.GetAnnotations(), LinkAnnotation)
		} else {
			obj.GetAnnotations()[LinkAnnotation] = names.String()
		}
		return true, nil
	}

	return false, nil
}

func (m *BindingTargetObjectMarker) GetReferencingTargets(ctx context.Context, obj client.Object) ([]client.ObjectKey, error) {
	v, contains := obj.GetAnnotations()[LinkAnnotation]
	if !contains {
		return []client.ObjectKey{}, nil
	}

	names := commaseparated.Value(v)
	keys := make([]client.ObjectKey, names.Len())
	for i, n := range names.Values() {
		keys[i].Name = n
		keys[i].Namespace = obj.GetNamespace()
	}
	return keys, nil
}
