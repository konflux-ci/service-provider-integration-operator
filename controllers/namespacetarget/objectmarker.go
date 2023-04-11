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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceObjectMarker struct {
	RemoteSecret  api.RemoteSecret
	SPIInstanceID string
}

// IsManaged implements bindings.ObjectMarker
func (m *NamespaceObjectMarker) IsManaged(ctx context.Context, obj client.Object) (bool, error) {
	annos := obj.GetAnnotations()
	return annos[ManagedByRemoteSecretNameAnnotation] == m.RemoteSecret.Name &&
		annos[ManagedByRemoteSecretNamespaceAnnotation] == m.RemoteSecret.GetNamespace() &&
		annos[ManagedByRemoteSecretSpiInstanceAnnotation] == m.SPIInstanceID, nil
}

// IsManagedByOther implements bindings.ObjectMarker
func (*NamespaceObjectMarker) IsManagedByOther(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

// IsReferenced implements bindings.ObjectMarker
func (*NamespaceObjectMarker) IsReferenced(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

// ListManagedOptions implements bindings.ObjectMarker
func (*NamespaceObjectMarker) ListManagedOptions(ctx context.Context) ([]client.ListOption, error) {
	panic("unimplemented")
}

// ListReferencedOptions implements bindings.ObjectMarker
func (*NamespaceObjectMarker) ListReferencedOptions(ctx context.Context) ([]client.ListOption, error) {
	panic("unimplemented")
}

// MarkManaged implements bindings.ObjectMarker
func (*NamespaceObjectMarker) MarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

// MarkReferenced implements bindings.ObjectMarker
func (*NamespaceObjectMarker) MarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

// UnmarkManaged implements bindings.ObjectMarker
func (*NamespaceObjectMarker) UnmarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

// UnmarkReferenced implements bindings.ObjectMarker
func (*NamespaceObjectMarker) UnmarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	panic("unimplemented")
}

var _ bindings.ObjectMarker = (*NamespaceObjectMarker)(nil)
