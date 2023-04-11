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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	dependents "github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BindingNamespaceTarget is the SecretDeploymentTarget that deploys to the same namespace as the SPI access token binding.
type BindingNamespaceTarget struct {
	Client  client.Client
	Binding *api.SPIAccessTokenBinding
}

var _ dependents.SecretDeploymentTarget = (*BindingNamespaceTarget)(nil)

// GetActualSecretName implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetActualSecretName() string {
	return t.Binding.Status.SyncedObjectRef.Name
}

// GetActualServiceAccountNames implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetActualServiceAccountNames() []string {
	return t.Binding.Status.ServiceAccountNames
}

// GetClient implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetClient() client.Client {
	return t.Client
}

// GetName implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetName() string {
	return t.Binding.Name
}

// GetNamespace implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetNamespace() string {
	return t.Binding.Namespace
}

// GetSpec implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetSpec() api.LinkableSecretSpec {
	return t.Binding.Spec.Secret.LinkableSecretSpec
}

// GetTargetNamespace implements dependents.SecretDeploymentTarget
func (t *BindingNamespaceTarget) GetTargetNamespace() string {
	return t.Binding.Namespace
}

// GetType implements dependents.SecretDeploymentTarget
func (*BindingNamespaceTarget) GetType() string {
	return "Binding"
}
