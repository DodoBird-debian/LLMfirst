package server

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	appdb "github.com/dodobird/llm-webui/db"
)

func handleListKeys(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys, err := appdb.ListKeys(db)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, keys)
	}
}

func handleCreateKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Provider string `json:"provider"`
			Label    string `json:"label"`
			KeyValue string `json:"key_value"`
			BaseURL  string `json:"base_url"`
		}
		if err := decodeJSON(r, &body); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		key, err := appdb.CreateKey(db, body.Provider, body.Label, body.KeyValue, body.BaseURL)
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
		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		var body struct {
			Label   string `json:"label"`
			BaseURL string `json:"base_url"`
		}
		decodeJSON(r, &body)
		if err := appdb.UpdateKey(db, id, body.Label, body.BaseURL); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleDeleteKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if err := appdb.DeleteKey(db, id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
