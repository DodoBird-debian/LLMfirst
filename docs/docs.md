# LLM WebUI — Exhaustive System & Architecture Documentation

This document is the absolute, comprehensive reference for the **LLM WebUI** project. It leaves no file, function, struct, or script undocumented. This acts as an architectural blueprint, REST API specification, database schema ledger, and UI behavioral guide for all edge cases across the entire codebase.

---

## Table of Contents
1. [Core Architecture](#1-core-architecture)
2. [Global Configuration (`main.go` & `config/`)](#2-global-configuration)
3. [Database & Storage Layer (`db/`)](#3-database--storage-layer)
4. [REST API Handlers (`server/`)](#4-rest-api-handlers)
5. [LLM Provider Registry (`providers/`)](#5-llm-provider-registry)
6. [Frontend Assets & Server (`web/`)](#6-frontend-assets--server)
7. [CSS Styling Layers (`web/static/css/`)](#7-css-styling-layers)
8. [Client-Side JavaScript (`web/static/js/`)](#8-client-side-javascript)

---

## 1. Core Architecture
The LLM WebUI is engineered as a zero-dependency, local-first binary. 
- **Language**: Go 1.22
- **Compilation**: Produces a single executable containing all HTML/CSS/JS via `//go:embed`.
- **Database**: SQLite (using the `modernc.org/sqlite` CGo-free driver).
- **Network**: HTTP Server handled by `github.com/go-chi/chi/v5`.
- **Frontend Paradigm**: Pure Vanilla ES6 + CSS3. No preprocessors, no React/Vue, no Webpack.

---

## 2. Global Configuration

### 2.1 `main.go`
**Location**: `main.go`
The entry execution point for the compiled binary.
- **`func main()`**
  - **Logic**: 
    1. Calls `config.ParseFlags()`.
    2. Opens the SQLite file via `db.OpenDB(cfg.DBPath)`. If the file does not exist, SQLite automatically touches it.
    3. Handles catastrophic failures (e.g., file permissions) with `log.Fatalf`.
    4. Executes `db.Migrate()` to assert schema parity.
    5. Injects the DB connection and Config into `server.NewRouter()`.
    6. Binds to `cfg.Host` and `cfg.Port` using standard `http.ListenAndServe()`.

### 2.2 `config/config.go`
**Location**: `config/config.go`
Manages CLI parameter parsing.
- **`type Config struct`**
  - `Host string`: The network interface to bind to (e.g., `127.0.0.1` or `0.0.0.0`).
  - `Port string`: The network port (e.g., `42068`).
  - `DBPath string`: The file path for SQLite.
  - `OllamaURL string`: Default base URL for Ollama instances.
  - `Secret string`: Currently reserved for future AES encryption implementations.
- **`func ParseFlags() Config`**
  - Uses the standard library `flag` package. Parses the OS arguments into the struct.

---

## 3. Database & Storage Layer

All database routines are in the `db/` package. The design enforces explicit function signatures rather than generic ORM queries.

### 3.1 `db/db.go` & `db/schema.sql`
- **`schema.sql`**: Contains `CREATE TABLE IF NOT EXISTS` for tables:
  1. `users`: `id` (PK), `username` (UNIQUE), `password_hash`, `role` (admin/user), `created_at`.
  2. `sessions`: `id` (UUID PK), `user_id` (FK cascading), `expires_at`, `created_at`.
  3. `api_keys`: `id` (PK), `user_id` (FK cascading), `provider`, `label`, `key_value`, `base_url`, `is_shared`, `created_at`.
  4. `conversations`: `id` (UUID PK), `user_id` (FK cascading), `title`, `provider`, `model`, `system_prompt`, `created_at`, `updated_at`.
  5. `messages`: `id` (PK AUTOINCREMENT), `conversation_id` (FK cascading), `role`, `content`, `token_count`, `created_at`.
  6. `settings`: `key` (PK), `value`.
- **`func OpenDB(path string) (*sql.DB, error)`**: Opens the DB and enforces a ping verify. Returns `*sql.DB`.
- **`func Migrate(db *sql.DB) error`**: Runs the embedded `schema.sql` text as a raw execution payload. Safe to run sequentially due to `IF NOT EXISTS`.

### 3.2 `db/conversations.go`
Contains all chat history storage logic.
- **`type Conversation struct`**: JSON mapped fields for IDs and timestamps.
- **`type Message struct`**: JSON mapped fields. Token count is defined but largely unused in UI (set to `0` initially).
- **`func CreateConversation(db *sql.DB, provider, model, systemPrompt string) (*Conversation, error)`**: Uses `github.com/google/uuid` to generate a `v4` UUID. Inserts and returns the row by querying `GetConversation`.
- **`func GetConversation(db *sql.DB, id string) (*Conversation, error)`**: Queries by ID. Uses `COALESCE` for `system_prompt` to avoid `NULL` scanning errors.
- **`func ListConversations(db *sql.DB) ([]Conversation, error)`**: Fetches all rows, orders by `updated_at DESC`.
- **`func UpdateConversation(db *sql.DB, id, title, systemPrompt string) error`**: Updates the metadata and forces `updated_at=CURRENT_TIMESTAMP`.
- **`func DeleteConversation(db *sql.DB, id string) error`**: Executes `DELETE FROM conversations`. The `ON DELETE CASCADE` in the schema automatically purges related messages.
- **`func SaveMessage(db *sql.DB, convID, role, content string, tokenCount int) (*Message, error)`**: Inserts a new row to `messages`. Also issues an `UPDATE conversations SET updated_at=CURRENT_TIMESTAMP` to bump the conversation to the top of the sidebar.
- **`func GetMessages(db *sql.DB, convID string) ([]Message, error)`**: Queries all messages for a `convID`, ordered by `id ASC` to preserve chronological chat flow.
- **`func DeleteMessage(db *sql.DB, id int64) error`**: Singular message deletion.

### 3.3 `db/keys.go`
Contains storage logic for API keys and global settings. Now scopes keys to specific `user_id`s, while Admins can set `is_shared=true` for global keys.
- **`type APIKey struct`**: Holds credentials, including `base_url`, `user_id`, and `is_shared`.
- **`func CreateKey(db *sql.DB, userID int64, provider, label, keyValue, baseURL string, isShared bool)`**: Basic `INSERT` with scoping.
- **`func ListKeys(db *sql.DB, userID int64, role string)`**: **CRITICAL SECURITY MEASURE**: Manually overwrites `KeyValue` to `"****" + kv[len(kv)-4:]` before sending to the UI. It returns personal keys plus any `is_shared=true` keys.
- **`func GetKeyValue(db *sql.DB, id int64, userID int64, role string) (string, string, error)`**: Fetches the unredacted key for server-side use, ensuring access rights.

### 3.4 `db/users.go`
Handles authentication and user lifecycle.
- **`type User struct` / `type Session struct`**: Core auth models.
- **`func CreateUser(db *sql.DB, username, password string) error`**: Uses bcrypt to hash passwords. Automatically assigns `admin` role to the first registered user.
- **`func AuthenticateUser(db *sql.DB, username, password string) (*User, error)`**: Verifies bcrypt hash.
- **`func CreateSession(...)` / `func GetSessionUser(...)`**: Issues UUID session tokens and validates them against the `sessions` table.

---

## 4. REST API Handlers

### 4.1 `server/server.go`
- **`func NewRouter(db *sql.DB, cfg config.Config) http.Handler`**
  - **Middlewares**: Applies `chi.Logger`, `chi.Recoverer` (prevents crashing on panic), and `chi.RequestID`.
  - **Static Serving**: Maps `/*` to `web.Handler()`.
  - **Routing Table**: Maps all `/api/*` endpoints to their respective handler functions below.

### 4.2 `server/handlers_chat.go`
The primary operational logic of the proxy.
- **`type chatRequest struct`**: Deserializes JSON payloads containing `conversationId`, `provider`, `model`, `keyId`, `messages`, and the custom inference options: `temperature`, `top_p`, and `max_tokens`.
- **`func handleChat(db *sql.DB, reg *providers.Registry) http.HandlerFunc`**
  - **Edge Case handling**: Returns `400 Bad Request` if JSON is malformed.
  - **Persistence**: Grabs the last element of `req.Messages` (the user input) and passes it to `appdb.SaveMessage`.
  - **Headers**: Sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`. Casts the `ResponseWriter` to `http.Flusher`.
  - **Provider Lookup**: Grabs the provider from `reg.Get()`. Fetches unredacted key from `appdb.GetKeyValue()`.
  - **Streaming Loop**:
    1. Instantiates `providers.Options{Temperature, TopP, MaxTokens}`.
    2. Calls `ChatStream()`. Receives an `io.ReadCloser`.
    3. Opens a `bufio.Scanner` to read the token chunks.
    4. Prints `data: {token}\n\n` to the stream and immediately calls `flusher.Flush()`.
    5. Accumulates tokens into a `full` string variable.
  - **Post-Stream Processing**: Saves `full` string into DB as `assistant` message. Prints `data: [DONE]\n\n`.

### 4.3 `server/handlers_keys.go`
Wrappers for API Key interactions.
- **`handleListKeys`**: Calls `ListKeys()`. The DB layer has already redacted the secrets.
- **`handleCreateKey`**: Calls `decodeJSON` and inserts key.
- **`handleUpdateKey`**: Parses URL ID integer. Calls `UpdateKey()`.
- **`handleDeleteKey`**: Returns `204 No Content`.

### 4.4 `server/handlers_models.go`
- **`handleHealth`**: Runs `db.Ping()` and returns `{"status":"ok", "db":"connected"}`.
- **`handleModels`**: Reads `?provider=X&keyId=Y` URL queries. Fetches raw key. Passes it to the specific Provider's `ListModels()` implementation.
- **`handleOllamaStatus`**: Invokes Ollama directly to test connectivity. Used by frontend to show green/gray pill dot.
- **`handleOllamaURL`**: Edits the `ollama_url` setting and injects it into memory.

### 4.5 `server/handlers_auth.go` & `server/middleware.go`
- **`handleSetup`**: Enables the initial admin registration.
- **`handleLogin`**: Authenticates user, creates a DB session, and sets a secure HTTP-only cookie.
- **`handleLogout`**: Deletes the session from the DB and clears the cookie.
- **`AuthMiddleware`**: Intercepts requests, validates the cookie against `db.GetSessionUser`, and injects the `*db.User` into the HTTP Request Context. Redirects `401 Unauthorized` on failure.

---

## 5. LLM Provider Registry

The plugin architecture for backend LLM bridging.

### 5.1 `providers/provider.go`
- **`type Message struct`**: `{Role, Content}`.
- **`type Options struct`**: `{Temperature, TopP, MaxTokens}`.
- **`type Provider interface`**:
  1. `ChatStream(ctx, model, apiKey, baseURL, messages, opts) (io.ReadCloser, error)`
  2. `ListModels(ctx, apiKey, baseURL) ([]string, error)`

### 5.2 `providers/registry.go`
- **`type Registry struct`**: Contains a map `map[string]Provider` and a specific pointer to `*OllamaProvider`.
- **`func NewRegistry`**: Fetches `ollama_url` from DB fallback to CLI. Instantiates all providers.
- **`func Get`**: Map lookup. Returns custom `errUnknownProvider` on miss.

### 5.3 `providers/ollama.go`
- **`ChatStream`**: 
  - Submits POST to `/api/chat`. Parses `opts` into the `options` key inside the JSON payload.
  - **Edge Case**: Ollama sends NDJSON. The scanner unpacks JSON strings line-by-line, formatting them into raw text and writing to an `io.PipeWriter`, converting NDJSON into a standard byte stream for the caller.

### 5.4 `providers/openai.go`
- **`ChatStream`**: Hits `/v1/chat/completions`. Dynamically embeds `temperature`, `top_p`, and `max_tokens` directly into the root request payload. Scans lines for `data: ` prefix. Unmarshals `chunk.Choices[0].Delta.Content`. Breaks on `[DONE]`.

### 5.5 `providers/anthropic.go`
- **`ChatStream`**: 
  - Submits to `/v1/messages`. Maps `temperature`, `top_p`, and dynamically translates `max_tokens` into Anthropic's expected payload format.
  - Extracts the `system` message out of the array and promotes it to a top-level JSON field, as required by Claude's API design.

### 5.6 `providers/gemini.go`
- **`ChatStream`**:
  - Targets `/models/{model}:streamGenerateContent?alt=sse`. Maps `opts` fields into Gemini's specific `generationConfig` wrapper object (`temperature`, `topP`, `maxOutputTokens`).
  - Translates role `"assistant"` to `"model"`. Parses array `chunk.Candidates[0].Content.Parts[0].Text`.

---

## 6. Frontend Assets & Server

### 6.1 `web/web.go`
- Uses `//go:embed all:static` to compile the filesystem into binary.
- Extracts sub-filesystem via `fs.Sub(staticFiles, "static")` to strip directory prefixes. Returns `http.FileServer`.

### 6.2 `web/static/index.html`
The entire DOM hierarchy:
- **`#sidebar`**: Sidebar container.
  - `#search-conv`: Real-time text filter for conversation history.
  - `#conversation-list`: Scrollable div.
- **`#main`**: Main body content grid.
  - **`#toolbar`**: Select dropdowns, dynamic Title, Chat Settings gear icon, and Theme Toggle (`[data-theme]`).
  - **`#chat-area`**: `#welcome` view and `#messages` scroll container.
- **Modals**: `#modal-settings` (API Keys, Admin panel), `#modal-chat-settings` (Temp, Top-P, Tokens, System Prompt), `#modal-rename`.

---

## 7. CSS Styling Layers

### 7.1 `web/static/css/tokens.css`
Declares CSS custom properties under `:root`.
- **Theming**: A `[data-theme="light"]` attribute block overrides the base dark theme variables, seamlessly switching `--col-bg`, `--col-surface`, `--col-text`, and `--col-border` dynamically.
- **Palette**: Accent `--col-accent` (`#7c6af7`), used across glows and active states.

### 7.2 `web/static/css/components.css`
Component-specific scopes mapping to UI widgets.
- **Conversation Items**: Nested structures with `.conv-actions` (including `.action-btn`) initially set to `opacity: 0`, and revealed upon `:hover` of parent `.conv-item`.
- **Chat Bubbles**: Support hover-revealed `.msg-actions` for Edit and Regenerate buttons. 
- **Markdown Specifics**: Targeting `pre`, `code`. Includes syntax highlighting token colors (`.syntax-keyword`, `.syntax-string`, `.syntax-number`).

---

## 8. Client-Side JavaScript

### 8.1 `web/static/js/state.js`
The global `window.State` singleton.
- Variables: `conversations`, `messages`, `theme`, `searchQuery`, `chatSettings` (Temp, TopP, MaxTokens).

### 8.2 `web/static/js/ui.js`
DOM modification logic.
- **`renderConversationList()`**: Wipes and maps `State.conversations`, applying `.hidden` via `State.searchQuery` matching.
- **`applyTheme()`**: Alters document attribute and updates toggle icon.
- **Message UI**: Injects `appendMessage()` with full Edit/Regenerate buttons integrated into the `.msg-header`.
- **Settings UI**: Binds inference parameter slider inputs and System Prompt textareas in `#modal-chat-settings`.

### 8.3 `web/static/js/chat.js`
Business logic bridging UI to API.
- **`sendMessage()`**: Evaluates context, pushes to `State.messages`, formats `options` JSON, and initiates `startStreaming()`.
- **`regenerateMessage(id)`**: Locates the assistant message index, strips it and all subsequent messages, triggers a delete API call loop for purged records, and calls `startStreaming()` to get a fresh response.
- **`editMessage(id)`**: Converts a user message back into the input textarea, purges subsequent conversation history in the database, and awaits the user's re-submission.
- **`startStreaming()`**: Streams SSE from `/api/chat` using `AbortController` and `getReader()`. Progressively overwrites DOM.

### 8.4 `web/static/js/markdown.js`
The zero-dependency Markdown parser `window.renderMarkdown(text)`.
- Replaces HTML entities (`<`, `>`, `&`).
- Features a **Syntax Highlighting Regex Engine**: Progressively tokenizes code blocks applying spans for numbers, strings (single/double/backticks), and common programming keywords.
- Standard Regex mapping for inline code, `h1-h4`, blockquotes, italics, lists, and paragraphs.
- Copy Code button injection logic.

### 8.5 `web/static/js/main.js`
The Application Bootstrapper bound to `DOMContentLoaded`.
- Evaluates `Promise.all` for loading history and keys. 
- Binds global UI event listeners, including Theme Toggle, Search Filter, and Modal interactions.

### 8.6 `web/static/js/auth.js`
Handles all authentication logic and the Admin Panel.
- Global `fetch` Interceptor overrides `window.fetch` to globally catch `401 Unauthorized` responses.

---
*End of Comprehensive Documentation.*
