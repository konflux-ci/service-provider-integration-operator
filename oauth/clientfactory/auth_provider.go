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

package clientfactory

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const authPluginName = "spi.appstudio.redhat.com/auth-from-request"

var (
	noBearerTokenError = errors.New("no bearer token found")
)

func init() {
	utilruntime.Must(rest.RegisterAuthProviderPlugin(authPluginName, func(string, map[string]string, rest.AuthProviderConfigPersister) (rest.AuthProvider, error) {
		return &fromContextAuthProvider{}, nil
	}))
}

// fromContextAuthProvider is the implementation of the rest.AuthProvider interface that looks for the bearer tokens
// in the context.
type fromContextAuthProvider struct{}

var _ rest.AuthProvider = (*fromContextAuthProvider)(nil)

// AugmentConfiguration modifies the provided Kubernetes client configuration such that it uses bearer tokens stored
// in the context using the WithAuthFromRequestIntoContext or WithAuthIntoContext functions.
func AugmentConfiguration(config *rest.Config) {
	config.AuthProvider = &api.AuthProviderConfig{Name: authPluginName}
}

// ExtractTokenFromAuthorizationHeader extracts the token value from the authorization header assumed to be formatted
// as a bearer token.
func ExtractTokenFromAuthorizationHeader(authHeader string) string {
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return ""
	}

	return authHeader[len("Bearer "):]
}

// WithAuthFromRequestIntoContext looks into the provided HTTP request and stores the bearer token from that request's Authorization
// header into the returned context which is based on the provided context.
// If used with a client constructed from configuration augmented using the AugmentConfiguration function, the requests
// to the Kubernetes API will be authenticated using this token.
//
// To link the contexts, you can reuse the context of the provided request: WithAuthFromRequestIntoContext(req, req.Context())
func WithAuthFromRequestIntoContext(r *http.Request, ctx context.Context) (context.Context, error) {
	token := ExtractTokenFromAuthorizationHeader(r.Header.Get("Authorization"))

	if token == "" {
		return nil, noBearerTokenError
	}

	return WithAuthIntoContext(token, ctx), nil
}

// WithAuthIntoContext stores the provided bearer token into the returned context which is based on the provided
// context.
// If used with a client constructed from configuration augmented using the AugmentConfiguration function, the requests
// to the Kubernetes API will be authenticated using this token.
func WithAuthIntoContext(bearerToken string, ctx context.Context) context.Context {
	return httptransport.WithBearerToken(ctx, bearerToken)
}

func (f fromContextAuthProvider) WrapTransport(tripper http.RoundTripper) http.RoundTripper {
	return &httptransport.AuthenticatingRoundTripper{
		RoundTripper: tripper,
	}
}

func (f fromContextAuthProvider) Login() error {
	return nil
}
