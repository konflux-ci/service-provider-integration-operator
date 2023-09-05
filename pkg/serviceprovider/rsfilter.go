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

package serviceprovider

import (
	"context"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

// RemoteSecretFilter is a helper interface to implement the ServiceProvider.lookupRemoteSecrets method using the GenericLookup struct.
type RemoteSecretFilter interface {
	Matches(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool
}

type RemoteSecretFilterFunc func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool

var _ = (RemoteSecretFilterFunc)(nil)

func (f RemoteSecretFilterFunc) Matches(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
	return f(ctx, matchable, remoteSecret)
}

var _ RemoteSecretFilter = (RemoteSecretFilterFunc)(nil)

// DefaultRemoteSecretFilterFunc filters RemoteSecret based on three conditions: the SecretData must be obtained,
// type of the secret must SecretTypeBasicAuth, and the RemoteSecret must have target in the same (local) namespace as matchable.
var DefaultRemoteSecretFilterFunc = RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
	dataObtained := meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, string(v1beta1.RemoteSecretConditionTypeDataObtained))
	basicAuthType := remoteSecret.Spec.Secret.Type == v1.SecretTypeBasicAuth
	correctTarget := getLocalNamespaceTargetIndex(remoteSecret.Status.Targets, matchable.ObjNamespace()) != -1
	return dataObtained && basicAuthType && correctTarget
})
