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

// NamespaceTarget is the SecretDeploymentTarget that deploys the secrets and service accounts to some namespace on the cluster.
type NamespaceTarget struct {
	Client       client.Client
	TargetKey    client.ObjectKey
	SecretSpec   *api.LinkableSecretSpec
	TargetSpec   *api.RemoteSecretTarget
	TargetStatus *api.TargetStatus
}

var _ bindings.SecretDeploymentTarget = (*NamespaceTarget)(nil)

func (t *NamespaceTarget) GetSpec() api.LinkableSecretSpec {
	return *t.SecretSpec
}

func (t *NamespaceTarget) GetClient() client.Client {
	return t.Client
}

func (t *NamespaceTarget) GetTargetObjectKey() client.ObjectKey {
	return t.TargetKey
}

func (t *NamespaceTarget) GetTargetNamespace() string {
	// target spec can be nil if the caller specifically wants to only process existing stuff
	// (e.g. finalizer that just deletes stuff) or if the status and spec are out of sync
	// (e.g. when we reconcile after a user removed a target from the spec of the remote secret).
	// target status is going to be nil if the spec and status are out of sync (e.g. user
	// added stuff to spec).
	if t.TargetSpec != nil {
		return t.TargetSpec.Namespace
	} else if t.TargetStatus != nil {
		return t.TargetStatus.Namespace.Namespace
	} else {
		// should never happen, but we need to return something
		return ""
	}
}

func (t *NamespaceTarget) GetActualSecretName() string {
	if t.TargetStatus == nil {
		return ""
	} else {
		return t.TargetStatus.Namespace.SecretName
	}
}

func (t *NamespaceTarget) GetActualServiceAccountNames() []string {
	if t.TargetStatus == nil {
		return []string{}
	} else {
		return t.TargetStatus.Namespace.ServiceAccountNames
	}
}

func (t *NamespaceTarget) GetType() string {
	return "Namespace"
}
