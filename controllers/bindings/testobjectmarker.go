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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestObjectMarker struct {
	IsManagedImpl             func(context.Context, client.Object) (bool, error)
	IsManagedByOtherImpl      func(context.Context, client.Object) (bool, error)
	IsReferencedImpl          func(context.Context, client.Object) (bool, error)
	ListManagedOptionsImpl    func(context.Context) ([]client.ListOption, error)
	ListReferencedOptionsImpl func(context.Context) ([]client.ListOption, error)
	MarkManagedImpl           func(context.Context, client.Object) (bool, error)
	MarkReferencedImpl        func(context.Context, client.Object) (bool, error)
	UnmarkManagedImpl         func(context.Context, client.Object) (bool, error)
	UnmarkReferencedImpl      func(context.Context, client.Object) (bool, error)
}

var _ ObjectMarker = (*TestObjectMarker)(nil)

// IsManaged implements ObjectMarker
func (t *TestObjectMarker) IsManaged(ctx context.Context, obj client.Object) (bool, error) {
	if t.IsManagedImpl != nil {
		return t.IsManagedImpl(ctx, obj)
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
func (t *TestObjectMarker) IsReferenced(ctx context.Context, obj client.Object) (bool, error) {
	if t.IsReferencedImpl != nil {
		return t.IsReferencedImpl(ctx, obj)
	}
	return false, nil
}

// ListManagedOptions implements ObjectMarker
func (t *TestObjectMarker) ListManagedOptions(ctx context.Context) ([]client.ListOption, error) {
	if t.ListManagedOptionsImpl != nil {
		return t.ListManagedOptionsImpl(ctx)
	}
	return []client.ListOption{}, nil
}

// ListReferencedOptions implements ObjectMarker
func (t *TestObjectMarker) ListReferencedOptions(ctx context.Context) ([]client.ListOption, error) {
	if t.ListReferencedOptionsImpl != nil {
		return t.ListReferencedOptionsImpl(ctx)
	}
	return []client.ListOption{}, nil
}

// MarkManaged implements ObjectMarker
func (t *TestObjectMarker) MarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	if t.MarkManagedImpl != nil {
		return t.MarkManagedImpl(ctx, obj)
	}
	return false, nil
}

// MarkReferenced implements ObjectMarker
func (t *TestObjectMarker) MarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	if t.MarkReferencedImpl != nil {
		return t.MarkReferencedImpl(ctx, obj)
	}
	return false, nil
}

// UnmarkManaged implements ObjectMarker
func (t *TestObjectMarker) UnmarkManaged(ctx context.Context, obj client.Object) (bool, error) {
	if t.UnmarkManagedImpl != nil {
		return t.UnmarkManagedImpl(ctx, obj)
	}
	return false, nil
}

// UnmarkReferenced implements ObjectMarker
func (t *TestObjectMarker) UnmarkReferenced(ctx context.Context, obj client.Object) (bool, error) {
	if t.UnmarkReferencedImpl != nil {
		return t.UnmarkReferencedImpl(ctx, obj)
	}
	return false, nil
}
