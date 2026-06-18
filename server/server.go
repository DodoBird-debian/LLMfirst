package server

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dodobird/llm-webui/config"
	"github.com/dodobird/llm-webui/providers"
	"github.com/dodobird/llm-webui/web"
)

// NewRouter sets up and returns the HTTP router.
func NewRouter(db *sql.DB, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Provider registry
	reg := providers.NewRegistry(db, cfg)

	// Static web assets (embedded)
	r.Handle("/*", web.Handler())

	// Health
	r.Get("/api/health", handleHealth(db))

	// Public Auth routes
	r.Get("/api/auth/me", MeHandler(db))
	r.Post("/api/auth/login", LoginHandler(db))
	r.Post("/api/auth/setup", SetupHandler(db))
	r.Post("/api/auth/logout", LogoutHandler(db))

	// Protected routes group
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(db))

		// Models
		r.Get("/api/models", handleModels(db, reg, cfg.Secret))

		// Chat (SSE streaming)
		r.Post("/api/chat", handleChat(db, reg, cfg.Secret))

		// Ollama
		r.Get("/api/ollama/status", handleOllamaStatus(reg))
		r.Put("/api/ollama/url", handleOllamaURL(db, reg))

		// Conversations
		r.Get("/api/conversations", handleListConversations(db))
		r.Post("/api/conversations", handleCreateConversation(db))
		r.Get("/api/conversations/{id}", handleGetConversation(db))
		r.Put("/api/conversations/{id}", handleUpdateConversation(db))
		r.Delete("/api/conversations/{id}", handleDeleteConversation(db))

		// Messages
		r.Post("/api/conversations/{id}/messages", handleAddMessage(db))
		r.Delete("/api/messages/{id}", handleDeleteMessage(db))

		// API Keys
		r.Get("/api/keys", handleListKeys(db, cfg.Secret))
		r.Post("/api/keys", handleCreateKey(db, cfg.Secret))
		r.Put("/api/keys/{id}", handleUpdateKey(db))
		r.Delete("/api/keys/{id}", handleDeleteKey(db))

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(AdminMiddleware)
			r.Get("/api/users", GetUsersHandler(db))
			r.Post("/api/users", CreateUserHandler(db))
			r.Delete("/api/users/{id}", DeleteUserHandler(db))
		})
	})

	return r
}
