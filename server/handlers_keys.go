package server

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	appdb "github.com/dodobird/llm-webui/db"
)

func handleListKeys(db *sql.DB, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		keys, err := appdb.ListKeys(db, secret, currentUser.ID, currentUser.Role)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, keys)
	}
}

func handleCreateKey(db *sql.DB, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var body struct {
			Provider string `json:"provider"`
			Label    string `json:"label"`
			KeyValue string `json:"key_value"`
			BaseURL  string `json:"base_url"`
			IsShared bool   `json:"is_shared"`
		}
		if err := decodeJSON(r, &body); err != nil {
			http.Error(w, "bad request", 400)
			return
		}

		// Only admins can create shared keys
		isShared := body.IsShared && currentUser.Role == "admin"
		var userID *int64
		if !isShared {
			userID = &currentUser.ID
		}

		key, err := appdb.CreateKey(db, secret, userID, isShared, body.Provider, body.Label, body.KeyValue, body.BaseURL)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, key)
	}
}

func handleUpdateKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		var body struct {
			Label   string `json:"label"`
			BaseURL string `json:"base_url"`
		}
		decodeJSON(r, &body)
		if err := appdb.UpdateKey(db, id, body.Label, body.BaseURL, currentUser.ID, currentUser.Role); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleDeleteKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if err := appdb.DeleteKey(db, id, currentUser.ID, currentUser.Role); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
