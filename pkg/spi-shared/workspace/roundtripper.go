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

package workspace

import (
	"fmt"
	"net/http"
	"path"
)

type RoundTripper struct {
	Next http.RoundTripper
}

func (w RoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if wsName, hasWsName := FromContext(request.Context()); hasWsName {
		request.URL.Path = path.Join("/workspaces", wsName, request.URL.Path)
	}
	response, err := w.Next.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("failed to run next http roundtrip: %w", err)
	}
	return response, nil
}
