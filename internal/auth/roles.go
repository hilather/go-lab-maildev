package auth

import "github.com/hilather/go-lab-maildev/internal/model"

// DefaultScopes returns the frozen role → scope set. An explicit token
// Scopes list wins over role expansion.
func DefaultScopes(role string) []string {
	switch role {
	case model.RoleViewer:
		return []string{model.ScopeMailRead}
	case model.RoleOperator:
		return []string{model.ScopeMailRead, model.ScopeMailWrite}
	case model.RoleAdministrator:
		return allScopes()
	default:
		return nil
	}
}

func allScopes() []string {
	return []string{
		model.ScopeMailRead,
		model.ScopeMailWrite,
		model.ScopeMailAdmin,
		model.ScopeMailAuditRead,
	}
}

func expandScopes(role string, scopes []string) (string, []string) {
	out := append([]string(nil), scopes...)
	if len(out) > 0 {
		if role == "" {
			role = model.RoleAdministrator
		}
		return role, out
	}
	if role == "" {
		role = model.RoleAdministrator
	}
	return role, DefaultScopes(role)
}
