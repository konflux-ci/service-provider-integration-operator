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
	"fmt"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GenericLookup implements a token lookup in a generic way such that the users only need to provide a function
// to provide a service-provider-specific "state" of the token and a "filter" function that uses the token and its
// state to match it against a binding
type GenericLookup struct {
	SPICredentialsSource          CredentialsSource[api.SPIAccessToken]
	RemoteSecretCredentialsSource CredentialsSource[v1beta1.RemoteSecret]
}

// SPIAccessTokenLookup uses SPICredentialsSource to lookup SPIAccessToken which is suitable for the matchable object.
// You may notice that the function returns a list of tokens even though it looks just for a single token. This is to
// preserve the compatibility with LookupTokens function of the service provider where this method is called from.
func (l GenericLookup) SPIAccessTokenLookup(ctx context.Context, cl client.Client, matchable Matchable) ([]api.SPIAccessToken, error) {
	token, err := l.SPICredentialsSource.LookupCredentialsSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup SPIAccessTokens: %w", err)
	}
	if token != nil {
		return []api.SPIAccessToken{*token}, nil
	}
	return []api.SPIAccessToken{}, nil
}

// CredentialsLookup tries to obtain credentials from SPICredentialsSource and if that fails, it tries the same with RemoteSecretCredentialsSource.
// Note that credentials may be nil if the sources do not find suitable object for matchable.
func (l GenericLookup) CredentialsLookup(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	cred, err := l.SPICredentialsSource.LookupCredentials(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup credentials from SPIAccessToken source: %w", err)
	}
	if cred != nil {
		return cred, nil
	}
	cred, err = l.RemoteSecretCredentialsSource.LookupCredentials(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup credentials from RemoteSecret source: %w", err)
	}
	return cred, nil
}
