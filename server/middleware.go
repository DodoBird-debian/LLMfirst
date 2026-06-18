package server

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/dodobird/llm-webui/db"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(userContextKey).(*db.User)
	return u
}

// AuthMiddleware checks the session token cookie and populates context with the user.
func AuthMiddleware(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := db.ValidateSession(database, cookie.Value)
			if err != nil {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "session_token",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminMiddleware ensures the authenticated user is an admin.
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
