package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	appdb "github.com/dodobird/llm-webui/db"
	"github.com/dodobird/llm-webui/providers"
)

func handleHealth(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "connected"
		if err := db.Ping(); err != nil {
			status = "error: " + err.Error()
		}
		writeJSON(w, map[string]string{"status": "ok", "db": status})
	}
}

func handleModels(db *sql.DB, reg *providers.Registry, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := r.URL.Query().Get("provider")
		keyIDStr := r.URL.Query().Get("keyId")

		p, err := reg.Get(providerName)
		if err != nil {
			http.Error(w, "unknown provider: "+providerName, 400)
			return
		}

		// Look up API key if provided
		var apiKey, baseURL string
		if keyIDStr != "" {
			if id, err := strconv.ParseInt(keyIDStr, 10, 64); err == nil && id > 0 {
				currentUser := UserFromContext(r.Context())
				if currentUser == nil {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				apiKey, baseURL, err = appdb.GetKeyValue(db, secret, id, currentUser.ID, currentUser.Role)
				if err != nil {
					http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
					return
				}
			}
		}

		models, err := p.ListModels(r.Context(), apiKey, baseURL)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, models)
	}
}

func handleOllamaStatus(reg *providers.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o := reg.Ollama()
		models, err := o.ListModels(r.Context(), "", "")
		detected := err == nil
		writeJSON(w, map[string]interface{}{
			"detected": detected,
			"url":      o.BaseURL(),
			"models":   models,
		})
	}
}

func handleOllamaURL(db *sql.DB, reg *providers.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
			http.Error(w, "bad request", 400)
			return
		}
		o := reg.Ollama()
		if err := o.SetBaseURL(db, body.URL); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		models, _ := o.ListModels(r.Context(), "", "")
		writeJSON(w, map[string]interface{}{"status": "ok", "models": models})
	}
}

// writeJSON encodes v as JSON and writes it to w with Content-Type set.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// decodeJSON decodes JSON from r.Body into v.
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
