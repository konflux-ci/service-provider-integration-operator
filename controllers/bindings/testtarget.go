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

//go:build !release

package bindings

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestDeploymentTarget struct {
	GetClientImpl                    func() client.Client
	GetTypeImpl                      func() string
	GetTargetObjectKeyImpl           func() client.ObjectKey
	GetTargetNamespaceImpl           func() string
	GetSpecImpl                      func() api.LinkableSecretSpec
	GetActualSecretNameImpl          func() string
	GetActualServiceAccountNamesImpl func() []string
}

var _ SecretDeploymentTarget = (*TestDeploymentTarget)(nil)

type TestSecretDataGetter[K any] struct {
	GetDataImpl func(context.Context, K) (map[string][]byte, string, error)
}

var _ SecretDataGetter[bool] = (*TestSecretDataGetter[bool])(nil)

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

// GetActualSecretName implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetActualSecretName() string {
	if t.GetActualSecretNameImpl != nil {
		return t.GetActualSecretNameImpl()
	}

	return ""
}

// GetActualServiceAccountNames implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetActualServiceAccountNames() []string {
	if t.GetActualServiceAccountNamesImpl != nil {
		return t.GetActualServiceAccountNamesImpl()
	}

	return []string{}
}

// GetClient implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetClient() client.Client {
	if t.GetClientImpl != nil {
		return t.GetClientImpl()
	}

	return nil
}

// GetTargetObjectKey implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetTargetObjectKey() client.ObjectKey {
	if t.GetTargetObjectKeyImpl != nil {
		return t.GetTargetObjectKeyImpl()
	}

	return client.ObjectKey{}
}

// GetSpec implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetSpec() api.LinkableSecretSpec {
	if t.GetSpecImpl != nil {
		return t.GetSpecImpl()
	}

	return api.LinkableSecretSpec{}
}

// GetTargetNamespace implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetTargetNamespace() string {
	if t.GetTargetNamespaceImpl != nil {
		return t.GetTargetNamespaceImpl()
	}

	return ""
}

// GetType implements SecretDeploymentTarget
func (t *TestDeploymentTarget) GetType() string {
	if t.GetTypeImpl != nil {
		return t.GetTypeImpl()
	}

	return ""
}

// GetData implements SecretBuilder
func (g *TestSecretDataGetter[K]) GetData(ctx context.Context, secretDataKey K) (data map[string][]byte, errorReason string, err error) {
	if g.GetDataImpl != nil {
		return g.GetDataImpl(ctx, secretDataKey)
	}

	return map[string][]byte{}, "", nil
}

// IsManaged implements ObjectMarker
func (m *TestObjectMarker) IsManagedBy(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.IsManagedByImpl != nil {
		return m.IsManagedByImpl(ctx, target, obj)
	}
	return false, nil
}

// IsManagedByOther implements ObjectMarker
func (m *TestObjectMarker) IsManagedByOther(ctx context.Context, obj client.Object) (bool, error) {
	if m.IsManagedByOtherImpl != nil {
		return m.IsManagedByOtherImpl(ctx, obj)
	}
	return false, nil
}

// IsReferenced implements ObjectMarker
func (m *TestObjectMarker) IsReferencedBy(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.IsReferencedByImpl != nil {
		return m.IsReferencedByImpl(ctx, target, obj)
	}
	return false, nil
}

// ListManagedOptions implements ObjectMarker
func (m *TestObjectMarker) ListManagedOptions(ctx context.Context, target client.ObjectKey) ([]client.ListOption, error) {
	if m.ListManagedOptionsImpl != nil {
		return m.ListManagedOptionsImpl(ctx, target)
	}
	return []client.ListOption{}, nil
}

// ListReferencedOptions implements ObjectMarker
func (m *TestObjectMarker) ListReferencedOptions(ctx context.Context, target client.ObjectKey) ([]client.ListOption, error) {
	if m.ListReferencedOptionsImpl != nil {
		return m.ListReferencedOptionsImpl(ctx, target)
	}
	return []client.ListOption{}, nil
}

// MarkManaged implements ObjectMarker
func (m *TestObjectMarker) MarkManaged(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.MarkManagedImpl != nil {
		return m.MarkManagedImpl(ctx, target, obj)
	}
	return false, nil
}

// MarkReferenced implements ObjectMarker
func (m *TestObjectMarker) MarkReferenced(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.MarkReferencedImpl != nil {
		return m.MarkReferencedImpl(ctx, target, obj)
	}
	return false, nil
}

// UnmarkManaged implements ObjectMarker
func (m *TestObjectMarker) UnmarkManaged(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.UnmarkManagedImpl != nil {
		return m.UnmarkManagedImpl(ctx, target, obj)
	}
	return false, nil
}

// UnmarkReferenced implements ObjectMarker
func (m *TestObjectMarker) UnmarkReferenced(ctx context.Context, target client.ObjectKey, obj client.Object) (bool, error) {
	if m.UnmarkReferencedImpl != nil {
		return m.UnmarkReferencedImpl(ctx, target, obj)
	}
	return false, nil
}

// GetReferencingTarget implements ObjectMarker
func (m *TestObjectMarker) GetReferencingTargets(ctx context.Context, obj client.Object) ([]types.NamespacedName, error) {
	if m.GetReferencingTargetsImpl != nil {
		return m.GetReferencingTargetsImpl(ctx, obj)
	}
	return nil, nil
}
