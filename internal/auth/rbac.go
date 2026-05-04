// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"log/slog"
	"net/http"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

func roleRank(r Role) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

func HasAtLeast(actual, required Role) bool {
	return roleRank(actual) >= roleRank(required)
}

func RequireRole(required Role, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeAuthError(w)
				return
			}
			if !HasAtLeast(Role(user.Role), required) {
				logger.Warn("rbac forbidden", "user_id", user.ID, "role", user.Role, "required", string(required))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"forbidden","description":"forbidden"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
