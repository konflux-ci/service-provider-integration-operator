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

package gitlab

type Scope string

const (
	ScopeApi             Scope = "api"
	ScopeReadApi         Scope = "read_api"
	ScopeReadUser        Scope = "read_user"
	ScopeReadRepository  Scope = "read_repository"
	ScopeWriteRepository Scope = "write_repository"
	ScopeReadRegistry    Scope = "read_registry"
	ScopeWriteRegistry   Scope = "write_registry"
	ScopeSudo            Scope = "sudo" // less understood and less relevant scopes begin
	ScopeOpenid          Scope = "openid"
	ScopeProfile         Scope = "profile"
	ScopeEmail           Scope = "email"
)

type TokenState struct {
	// TODO: implement
}

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}
	switch s {
	case ScopeApi:
		return other == ScopeReadApi || other == ScopeReadUser || other == ScopeReadRepository ||
			other == ScopeWriteRepository || other == ScopeReadRegistry || other == ScopeWriteRegistry
	case ScopeReadApi:
		return other == ScopeReadUser || other == ScopeReadRepository || other == ScopeReadRegistry
	case ScopeWriteRepository:
		return other == ScopeReadRepository
	case ScopeWriteRegistry:
		return other == ScopeReadRegistry
	}
	return false
}

func IsValidScope(scope string) bool {
	switch Scope(scope) {
	case ScopeApi,
		ScopeReadApi,
		ScopeReadUser,
		ScopeReadRepository,
		ScopeWriteRepository,
		ScopeReadRegistry,
		ScopeWriteRegistry,
		ScopeSudo,
		ScopeOpenid,
		ScopeProfile,
		ScopeEmail:
		return true
	}
	return false
}
