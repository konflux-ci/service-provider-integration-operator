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

package oauth

import (
	"context"
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
)

// Test Controller that do nothing.
type NopController struct {
}

var _ Controller = (*NopController)(nil)

func (n NopController) Authenticate(w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo) {
}
func (n NopController) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo) {
}
