package server

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	appdb "github.com/dodobird/llm-webui/db"
	"github.com/dodobird/llm-webui/providers"
)

type chatRequest struct {
	ConversationID string              `json:"conversationId"`
	Provider       string              `json:"provider"`
	Model          string              `json:"model"`
	KeyID          int64               `json:"keyId"`
	Messages       []providers.Message `json:"messages"`
	Temperature    float32             `json:"temperature,omitempty"`
	TopP           float32             `json:"top_p,omitempty"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
}

func handleChat(db *sql.DB, reg *providers.Registry, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Persist user message (last in list)
		if len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == "user" {
				tokenCount := len(last.Content) / 4
				_, _ = appdb.SaveMessage(db, req.ConversationID, last.Role, last.Content, tokenCount, currentUser.ID, currentUser.Role)
			}
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		p, err := reg.Get(req.Provider)
		if err != nil {
			fmt.Fprintf(w, "data: {\"error\": %q}\n\n", err.Error())
			flusher.Flush()
			return
		}

		// Get key
		var apiKey, baseURL string
		if req.KeyID > 0 {
			apiKey, baseURL, err = appdb.GetKeyValue(db, secret, req.KeyID, currentUser.ID, currentUser.Role)
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": %q}\n\n", err.Error())
				flusher.Flush()
				return
			}
		}

		opts := providers.Options{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			MaxTokens:   req.MaxTokens,
		}

		stream, err := p.ChatStream(r.Context(), req.Model, apiKey, baseURL, req.Messages, opts)
		if err != nil {
			fmt.Fprintf(w, "data: {\"error\": %q}\n\n", err.Error())
			flusher.Flush()
			return
		}
		defer stream.Close()

		var full string
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			full += line
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}

		// Persist full assistant response
		if req.ConversationID != "" {
			tokenCount := len(full) / 4
			_, _ = appdb.SaveMessage(db, req.ConversationID, "assistant", full, tokenCount, currentUser.ID, currentUser.Role)
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

func handleListConversations(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		convs, err := appdb.ListConversations(db, currentUser.ID, currentUser.Role)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, convs)
	}
}

func handleCreateConversation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var body struct {
			Provider     string `json:"provider"`
			Model        string `json:"model"`
			SystemPrompt string `json:"system_prompt"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		conv, err := appdb.CreateConversation(db, currentUser.ID, body.Provider, body.Model, body.SystemPrompt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, conv)
	}
}

func handleGetConversation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		id := chi.URLParam(r, "id")
		conv, err := appdb.GetConversation(db, id, currentUser.ID, currentUser.Role)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		msgs, err := appdb.GetMessages(db, id, currentUser.ID, currentUser.Role)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]interface{}{"conversation": conv, "messages": msgs})
	}
}

func handleUpdateConversation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		id := chi.URLParam(r, "id")
		var body struct {
			Title        string `json:"title"`
			SystemPrompt string `json:"system_prompt"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if err := appdb.UpdateConversation(db, id, body.Title, body.SystemPrompt, currentUser.ID, currentUser.Role); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleDeleteConversation(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		id := chi.URLParam(r, "id")
		if err := appdb.DeleteConversation(db, id, currentUser.ID, currentUser.Role); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAddMessage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		convID := chi.URLParam(r, "id")
		var body struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		tokenCount := len(body.Content) / 4
		msg, err := appdb.SaveMessage(db, convID, body.Role, body.Content, tokenCount, currentUser.ID, currentUser.Role)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, msg)
	}
}

func handleDeleteMessage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := UserFromContext(r.Context())
		if currentUser == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if err := appdb.DeleteMessage(db, id, currentUser.ID, currentUser.Role); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
