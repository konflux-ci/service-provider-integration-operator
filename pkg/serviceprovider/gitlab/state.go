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
	ScopeSudo            Scope = "sudo" // less understood scopes bellow
	ScopeOpenid          Scope = "openid"
	ScopeProfile         Scope = "profile"
	ScopeEmail           Scope = "email"
)

type TokenState struct {
}

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}
	switch s {
	case ScopeApi:
		return other == ScopeReadApi || other == ScopeReadUser || other == ScopeReadRepository || other == ScopeWriteRepository || other == ScopeReadRegistry || other == ScopeWriteRegistry
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
