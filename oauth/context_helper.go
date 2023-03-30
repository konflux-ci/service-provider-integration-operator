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

import "context"

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

var (
	NamespaceContextKey = contextKey("namespace")
)

func GetNamespaceFromContext(ctx context.Context) (string, bool) {
	namespace, ok := ctx.Value(NamespaceContextKey).(string)
	return namespace, ok
}
