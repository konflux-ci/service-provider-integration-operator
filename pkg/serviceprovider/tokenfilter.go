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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
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
