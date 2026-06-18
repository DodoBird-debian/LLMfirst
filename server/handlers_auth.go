package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/dodobird/llm-webui/db"
	"github.com/go-chi/chi/v5"
)

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type MeResponse struct {
	NeedsSetup    bool     `json:"needs_setup"`
	Authenticated bool     `json:"authenticated"`
	User          *db.User `json:"user,omitempty"`
}

// SetupHandler handles first-boot setup.
func SetupHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasUsers, err := db.HasUsers(database)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if hasUsers {
			http.Error(w, "Setup already completed", http.StatusForbidden)
			return
		}

		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		user, err := db.CreateUser(database, req.Username, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Auto login
		sess, err := db.CreateSession(database, user.ID, 30*24*time.Hour)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sess.Token,
			Path:     "/",
			Expires:  sess.ExpiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

// LoginHandler handles standard login.
func LoginHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := db.AuthenticateUser(database, req.Username, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		sess, err := db.CreateSession(database, user.ID, 30*24*time.Hour)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sess.Token,
			Path:     "/",
			Expires:  sess.ExpiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

// LogoutHandler logs out the current user.
func LogoutHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			_ = db.DeleteSession(database, cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		w.WriteHeader(http.StatusOK)
	}
}

// MeHandler returns the currently logged in user info or if setup is needed.
func MeHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasUsers, err := db.HasUsers(database)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := MeResponse{
			NeedsSetup: !hasUsers,
		}

		cookie, err := r.Cookie("session_token")
		if err == nil {
			user, err := db.ValidateSession(database, cookie.Value)
			if err == nil {
				resp.Authenticated = true
				resp.User = user
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// GetUsersHandler retrieves all users (Admin only).
func GetUsersHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := db.GetUsers(database)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}
}

// CreateUserHandler registers a new user (Admin only).
func CreateUserHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		user, err := db.CreateUserByAdmin(database, req.Username, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

// DeleteUserHandler removes a user (Admin only).
func DeleteUserHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		currentUser := UserFromContext(r.Context())
		if currentUser != nil && currentUser.ID == id {
			http.Error(w, "You cannot delete yourself", http.StatusBadRequest)
			return
		}

		err = db.DeleteUser(database, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
