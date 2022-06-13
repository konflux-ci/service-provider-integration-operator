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
	"strings"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticatingRoundTripper_RoundTrip(t *testing.T) {
	ctx := WithBearerToken(context.TODO(), "token")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://over.the.rainbow", strings.NewReader(""))
	assert.NoError(t, err)

	roundtripProcessed := false

	tr := AuthenticatingRoundTripper{util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		roundtripProcessed = true
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		return nil, nil
	})}

	_, _ = tr.RoundTrip(req)

	assert.True(t, roundtripProcessed)
}
