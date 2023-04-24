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

// go:build

package bindings

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestObjectMarker struct {
	IsManagedByImpl           func(context.Context, client.ObjectKey, client.Object) (bool, error)
	IsManagedByOtherImpl      func(context.Context, client.Object) (bool, error)
	IsReferencedByImpl        func(context.Context, client.ObjectKey, client.Object) (bool, error)
	ListManagedOptionsImpl    func(context.Context, client.ObjectKey) ([]client.ListOption, error)
	ListReferencedOptionsImpl func(context.Context, client.ObjectKey) ([]client.ListOption, error)
	MarkManagedImpl           func(context.Context, client.ObjectKey, client.Object) (bool, error)
	MarkReferencedImpl        func(context.Context, client.ObjectKey, client.Object) (bool, error)
	UnmarkManagedImpl         func(context.Context, client.ObjectKey, client.Object) (bool, error)
	UnmarkReferencedImpl      func(context.Context, client.ObjectKey, client.Object) (bool, error)
	GetReferencingTargetsImpl func(context.Context, client.Object) ([]client.ObjectKey, error)
}

var _ ObjectMarker = (*TestObjectMarker)(nil)

// IsManaged implements ObjectMarker
func (t *TestObjectMarker) IsManagedBy(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.IsManagedByImpl != nil {
		return t.IsManagedByImpl(ctx, target, obj)
	}
	return false, nil
}

// IsManagedByOther implements ObjectMarker
func (t *TestObjectMarker) IsManagedByOther(ctx context.Context, obj client.Object) (bool, error) {
	if t.IsManagedByOtherImpl != nil {
		return t.IsManagedByOtherImpl(ctx, obj)
	}
	return false, nil
}

// IsReferenced implements ObjectMarker
func (t *TestObjectMarker) IsReferencedBy(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.IsReferencedByImpl != nil {
		return t.IsReferencedByImpl(ctx, target, obj)
	}
	return false, nil
}

// ListManagedOptions implements ObjectMarker
func (t *TestObjectMarker) ListManagedOptions(ctx context.Context, target client.ObjectKey) ([]client.ListOption, error) {
	if t.ListManagedOptionsImpl != nil {
		return t.ListManagedOptionsImpl(ctx, target)
	}
	return []client.ListOption{}, nil
}

// ListReferencedOptions implements ObjectMarker
func (t *TestObjectMarker) ListReferencedOptions(ctx context.Context, target client.ObjectKey) ([]client.ListOption, error) {
	if t.ListReferencedOptionsImpl != nil {
		return t.ListReferencedOptionsImpl(ctx, target)
	}
	return []client.ListOption{}, nil
}

// MarkManaged implements ObjectMarker
func (t *TestObjectMarker) MarkManaged(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.MarkManagedImpl != nil {
		return t.MarkManagedImpl(ctx, target, obj)
	}
	return false, nil
}

// MarkReferenced implements ObjectMarker
func (t *TestObjectMarker) MarkReferenced(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.MarkReferencedImpl != nil {
		return t.MarkReferencedImpl(ctx, target, obj)
	}
	return false, nil
}

// UnmarkManaged implements ObjectMarker
func (t *TestObjectMarker) UnmarkManaged(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.UnmarkManagedImpl != nil {
		return t.UnmarkManagedImpl(ctx, target, obj)
	}
	return false, nil
}

// UnmarkReferenced implements ObjectMarker
func (t *TestObjectMarker) UnmarkReferenced(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if t.UnmarkReferencedImpl != nil {
		return t.UnmarkReferencedImpl(ctx, target, obj)
	}
	return false, nil
}

// GetReferencingTarget implements ObjectMarker
func (t *TestObjectMarker) GetReferencingTargets(ctx context.Context, obj client.Object) ([]types.NamespacedName, error) {
	if t.GetReferencingTargetsImpl != nil {
		return t.GetReferencingTargetsImpl(ctx, obj)
	}
	return nil, nil
}
