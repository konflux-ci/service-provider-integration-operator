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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretDeploymentTarget, together with SecretBuilder and ObjectMarker, represents a method of obtaining enough
// information for the DependentsHandler to be able to deliver the secrets and service accounts to some "target"
// place in (some) K8s cluster.
type SecretDeploymentTarget interface {
	// GetClient returns the client to use when connecting to the target "destination" to deploy the dependent objects to.
	GetClient() client.Client
	// GetType returns the type of the secret deployment target object.
	GetType() string
	// GetTargetObjectKey is the location of the object that describes the target. This is mainly used in error reporting.
	GetTargetObjectKey() client.ObjectKey
	// GetTargetNamespace specifies the namespace to which the secret and service accounts
	// should be deployed to.
	GetTargetNamespace() string
	// GetSpec gives the spec from which the secrets and service accounts should be created.
	GetSpec() api.LinkableSecretSpec
	// GetActualSecretName returns the actual name of the secret, if any (as opposed to the
	// configured name from the spec, which may not fully represent what's in the cluster
	// if for example GenerateName is used).
	GetActualSecretName() string
	// GetActualServiceAccountNames returns the names of the service accounts that the spec
	// configures.
	GetActualServiceAccountNames() []string
}

// SecretBuilder is an abstraction that, given the provided key, is able to obtain the secret data from some kind of backing
// secret storage.
type SecretBuilder[K any] interface {
	// GetData returns the secret data from the backend storage given the key. If the data is not found, this method
	// MUST return the AccessTokenDataNotFoundError.
	GetData(ctx context.Context, secretDataKey K) (data map[string][]byte, errorReason string, err error)
}

// ObjectMarker is used to mark or unmark some object with a link to the target with which this instance is initialized.
// What constitutes a target and how to mark/unmark it is dependent on the type of the deployment target.
type ObjectMarker interface {
	MarkManaged(ctx context.Context, obj client.Object) (bool, error)
	UnmarkManaged(ctx context.Context, obj client.Object) (bool, error)
	MarkReferenced(ctx context.Context, obj client.Object) (bool, error)
	UnmarkReferenced(ctx context.Context, obj client.Object) (bool, error)
	IsManaged(ctx context.Context, obj client.Object) (bool, error)
	IsManagedByOther(ctx context.Context, obj client.Object) (bool, error)
	IsReferenced(ctx context.Context, obj client.Object) (bool, error)
	ListManagedOptions(ctx context.Context) ([]client.ListOption, error)
	ListReferencedOptions(ctx context.Context) ([]client.ListOption, error)
}

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
