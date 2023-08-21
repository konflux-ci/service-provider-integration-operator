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
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// TokenFilter is a helper interface to implement the ServiceProvider.LookupToken method using the GenericLookup struct.
type TokenFilter interface {
	Matches(ctx context.Context, matchable Matchable, token *api.SPIAccessToken) (bool, error)
}

// TokenFilterFunc converts a function into the implementation of the TokenFilter interface
type TokenFilterFunc func(ctx context.Context, matchable Matchable, token *api.SPIAccessToken) (bool, error)

var _ TokenFilter = (TokenFilterFunc)(nil)

func (f TokenFilterFunc) Matches(ctx context.Context, matchable Matchable, token *api.SPIAccessToken) (bool, error) {
	return f(ctx, matchable, token)
}

// MatchAllTokenFilter is a TokenFilter that match any token
var MatchAllTokenFilter TokenFilter = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
	debugLog := log.FromContext(ctx).V(logs.DebugLevel)
	debugLog.Info("Unconditional token match", "token", token)
	return true, nil
})

func NewFilter(policy config.TokenPolicy, exactTokenFilter TokenFilter) TokenFilter {
	if policy == config.AnyTokenPolicy {
		return MatchAllTokenFilter
	}
	return exactTokenFilter
}

// RemoteSecretFilter is a helper interface to implement the ServiceProvider.LookupRemoteSecrets method using the GenericLookup struct.
type RemoteSecretFilter interface {
	Matches(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool
}

type RemoteSecretFilterFunc func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool

var _ = (RemoteSecretFilterFunc)(nil)

func (f RemoteSecretFilterFunc) Matches(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
	return f(ctx, matchable, remoteSecret)
}

var _ RemoteSecretFilter = (RemoteSecretFilterFunc)(nil)

var DefaultRemoteSecretFilterFunc = RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
	dataObtained := meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, string(v1beta1.RemoteSecretConditionTypeDataObtained))
	basicAuthType := remoteSecret.Spec.Secret.Type == v1.SecretTypeBasicAuth
	correctTarget := getLocalNamespaceTargetIndex(remoteSecret.Status.Targets, matchable.ObjNamespace()) != -1
	return dataObtained && basicAuthType && correctTarget
})
