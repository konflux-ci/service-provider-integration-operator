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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers/bindings"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ManagedByRemoteSecretNameAnnotation = "spi.appstudio.redhat.com/managing-remote-secret-name"                //#nosec G101 -- false positive, this is just a label
const ManagedByRemoteSecretNamespaceAnnotation = "spi.appstudio.redhat.com/managing-remote-secret-ns"             //#nosec G101 -- false positive, this is just a label
const ManagedByRemoteSecretSpiInstanceAnnotation = "spi.appstudio.redhat.com/managing-remote-secret-spi-instance" //#nosec G101 -- false positive, this is just a label

// NamespaceTarget is the SecretDeploymentTarget that deploys the secrets and service accounts to some namespace on the cluster.
type NamespaceTarget struct {
	Client              client.Client
	RemoteSecret        *api.RemoteSecret
	Namespace           string
	SecretName          string
	ServiceAccountNames []string
}

var _ bindings.SecretDeploymentTarget = (*NamespaceTarget)(nil)

func (t *NamespaceTarget) GetSpec() api.LinkableSecretSpec {
	return t.RemoteSecret.Spec.Secret
}

func (t *NamespaceTarget) GetClient() client.Client {
	return t.Client
}

// GetName implements DeploymentTarget
func (t *NamespaceTarget) GetName() string {
	return t.RemoteSecret.Name
}

// GetNamespace implements DeploymentTarget
func (t *NamespaceTarget) GetNamespace() string {
	return t.RemoteSecret.Namespace
}

func (t *NamespaceTarget) GetTargetNamespace() string {
	return t.Namespace
}

// GetSecretName implements DeploymentTarget
func (t *NamespaceTarget) GetActualSecretName() string {
	return t.SecretName
}

// GetServiceAccountNames implements DeploymentTarget
func (t *NamespaceTarget) GetActualServiceAccountNames() []string {
	return t.ServiceAccountNames
}

// GetType implements DeploymentTarget
func (t *NamespaceTarget) GetType() string {
	return "Namespace"
}
