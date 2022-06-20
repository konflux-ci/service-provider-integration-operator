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

package httptransport

import (
	"context"
	"net/http"
)

// the private type and value making sure that only this package can create the contexts usable for the authenticating
// http client
type authenticatingRoundTripperContextKeyType struct{}

var authenticatingRoundTripperContextKey = authenticatingRoundTripperContextKeyType{}

// AuthenticatingRoundTripper is a wrapper around an HTTP round tripper that add a bearer token from the context
// (if any) before performing the request using the wrapped round tripper.
type AuthenticatingRoundTripper struct {
	http.RoundTripper
}

// WithBearerToken inserts the bearer token into the returned context which is based on the provided context. If this
// context is then used with the HTTP request in a client having the AuthenticatingRoundTripper as its transport, the
// round tripper will insert this token into the Authorization header of the request.
func WithBearerToken(ctx context.Context, bearerToken string) context.Context {
	return context.WithValue(ctx, authenticatingRoundTripperContextKey, bearerToken)
}

func (r AuthenticatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token, ok := req.Context().Value(authenticatingRoundTripperContextKey).(string)

	if ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return r.RoundTripper.RoundTrip(req) //nolint:wrapcheck // the errors should be handled by the users of the HTTP client configured with this roundtripper
}
