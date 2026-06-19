# LLM WebUI — Exhaustive System & Architecture Documentation

This document serves as the **Absolute Source of Truth** for the LLM WebUI project. It is written at a level of exhaustive detail that permits a software engineer to rebuild the entire application from this specification alone, without referring to the original source code. 

Every single file, struct, function signature, internal dependency, and call graph trace is documented below.

---

## Table of Contents
1. [Core Architectural Paradigm](#1-core-architectural-paradigm)
2. [Project Root (`/`)](#2-project-root)
3. [Global Configuration (`config/`)](#3-global-configuration)
4. [Storage & Database Layer (`db/`)](#4-storage--database-layer)
5. [Network Routing & Handlers (`server/`)](#5-network-routing--handlers)
6. [LLM Provider Registry (`providers/`)](#6-llm-provider-registry)
7. [Frontend Build & Assets (`web/`)](#7-frontend-build--assets)
8. [CSS Styling Engine (`web/static/css/`)](#8-css-styling-engine)
9. [Client-Side JS Logic (`web/static/js/`)](#9-client-side-js-logic)

---

## 1. Core Architectural Paradigm

The LLM WebUI is engineered as a zero-dependency, local-first binary running on Go 1.22.
- **Dependency Isolation**: No Node.js, no Webpack, no Docker required. 
- **Frontend Delivery**: Uses the `//go:embed` compiler directive to embed raw HTML/CSS/JS files directly into the compiled executable, served via the standard `net/http` package.
- **Database**: SQLite, but explicitly utilizing the CGo-free `modernc.org/sqlite` package. This guarantees cross-compilation for ARM/Windows/Linux without needing a local C compiler toolchain (e.g., GCC or MinGW).
- **Network Pipeline**: The backend proxies requests to external LLM providers (Ollama, OpenAI, Anthropic, Gemini) and pipes the Server-Sent Events (SSE) down to the vanilla JS frontend client using `http.Flusher`.

---

## 2. Project Root (`/`)

### `main.go`
**Purpose**: The entry execution point for the application.
**Call Graph Flow**:
1. Imports `config`, `db`, and `server`.
2. `func main()` invokes `config.ParseFlags()`.
3. Calls `db.OpenDB(cfg.DBPath)` to lock or initialize the SQLite file.
4. Executes `db.Migrate(database)` to run `CREATE TABLE IF NOT EXISTS` commands.
5. Injects the `database` reference and `cfg` into `server.NewRouter(database, cfg)`.
6. Prints a boot log to stdout: `log.Printf("LLM WebUI running at http://%s:%s", cfg.Host, cfg.Port)`.
7. Blocks execution using `http.ListenAndServe(...)`.

### `Makefile`
**Purpose**: Compilation macros.
- `make all`: Resolves to `make build`.
- `make build`: Runs `go build -o llm-webui.exe .`
- `make run`: Runs the binary using default parameters (`--port 42068 --db ./data.db`).
- `make clean`: `rm -f llm-webui.exe llm-webui`

### `go.mod` & `go.sum`
**Purpose**: Go module declarations.
- Direct dependencies: `github.com/go-chi/chi/v5` (routing), `github.com/google/uuid` (session and chat tokens), `golang.org/x/crypto` (bcrypt hashing), `modernc.org/sqlite` (DB driver).

---

## 3. Global Configuration (`config/`)

### `config/config.go`
**Purpose**: Single-source-of-truth for runtime parameters, parsed from CLI flags.
**Struct Definition**:
```go
type Config struct {
    Host      string
    Port      string
    DBPath    string
    OllamaURL string
    Secret    string
}
```
**Function `ParseFlags() Config`**:
- Binds standard library `flag.StringVar` for `--host` (default `0.0.0.0`), `--port` (default `42068`), `--db` (default `data.db`), `--ollama-url` (default `http://localhost:11434`), and `--secret`.
- Executes `flag.Parse()`.

---

## 4. Storage & Database Layer (`db/`)

The database layer abstracts raw SQL queries into tightly bound exported functions returning structured Go data. No generic ORM is used.

### `db/schema.sql`
**Purpose**: The embedded raw text payload executed during migration.
**Tables Defined**:
1. **`users`**: `id INTEGER PRIMARY KEY`, `username TEXT UNIQUE`, `password_hash TEXT`, `role TEXT` (admin/user), `created_at DATETIME`.
2. **`sessions`**: `token TEXT PRIMARY KEY` (UUID string), `user_id INTEGER` (Foreign Key cascading to users.id), `expires_at DATETIME`, `created_at DATETIME`.
3. **`api_keys`**: `id INTEGER PRIMARY KEY`, `user_id INTEGER`, `provider TEXT`, `label TEXT`, `key_value TEXT`, `base_url TEXT`, `is_shared BOOLEAN`.
4. **`conversations`**: `id TEXT PRIMARY KEY` (UUID string), `user_id INTEGER`, `title TEXT`, `provider TEXT`, `model TEXT`, `system_prompt TEXT`, `created_at`, `updated_at`.
5. **`messages`**: `id INTEGER PRIMARY KEY AUTOINCREMENT`, `conversation_id TEXT` (FK cascading), `role TEXT`, `content TEXT`, `token_count INTEGER`, `created_at`.
6. **`settings`**: `key TEXT PRIMARY KEY`, `value TEXT`.

### `db/db.go`
**Call Graph**: Called by `main.go`.
- `OpenDB(path string) (*sql.DB, error)`: Imports `_ "modernc.org/sqlite"`, opens DB using specific Pragmas for WAL (Write-Ahead Logging) to prevent database locks (`?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)`).
- `Migrate(db *sql.DB)`: Reads `//go:embed schema.sql` via string parsing and calls `db.Exec()`.

### `db/users.go`
**Purpose**: Authentication logic and bcrypt hashing.
- `CreateUser`: Computes `bcrypt.GenerateFromPassword(cost=12)`. Checks if `COUNT(*) FROM users` is zero; if so, hardcodes the `role` to `"admin"`.
- `AuthenticateUser`: Selects row, calls `bcrypt.CompareHashAndPassword`.
- `CreateSession`: Uses `uuid.New().String()`, sets `expires_at` using `time.Now().Add()`.

### `db/conversations.go`
**Purpose**: CRUD operations for the chat view sidebar and history mapping.
- `CreateConversation`: Uses `uuid.New().String()`. Executes `INSERT INTO conversations (id, user_id, provider, model, system_prompt, title)`.
- `GetConversation`: Uses `COALESCE(system_prompt, '')` to prevent null pointer crashes in the Go SQL driver.
- `ListConversations`: Fetches scoped to `user_id`. Note: Admins can be configured to see all if needed, but UI restricts to personal tokens.
- `SaveMessage`: Highly critical. Executes `INSERT INTO messages`, then immediately executes an `UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`. This guarantees the sidebar correctly pushes active chats to the top.
- `GetMessages`: Selects `* FROM messages WHERE conversation_id = ? ORDER BY id ASC`. Order is absolute necessity to maintain chat history continuity for LLM context payload.

### `db/keys.go`
**Purpose**: API key persistence.
- `ListKeys`: Crucial security detail here. Retrieves the rows, then replaces the raw `key_value` strings in memory with `"****" + key[len-4:]` *before* returning them to the HTTP handler.
- `GetKeyValue`: The internal retrieval method used by `server/handlers_chat.go` to access the true secret payload for outbound HTTP REST requests. Asserts ownership (`user_id == req.user_id` OR `is_shared == 1`).

---

## 5. Network Routing & Handlers (`server/`)

The `chi` router abstracts HTTP multiplexing, providing a cleaner interface over standard `net/http`.

### `server/server.go`
**Purpose**: Global routing table.
**Call Graph**:
- Invoked by `main.go`. Instantiates the `providers.Registry`.
- Mounts standard `chi/v5/middleware`: `Logger`, `Recoverer` (prevents panic crashes), `RequestID`.
- Defines Public Routes: `/api/health`, `/api/auth/login`, `/api/auth/me`, `/api/auth/setup`, `/api/auth/logout`.
- Mounts Protected Route Group (`r.Group`): Wraps with `AuthMiddleware(db)`. Includes all `GET`/`POST`/`PUT`/`DELETE` configurations for `/api/conversations`, `/api/chat`, and `/api/keys`.
- Mounts Admin Group: Wraps with `AdminMiddleware` for `/api/users`.

### `server/middleware.go`
**Purpose**: Request gating.
- `AuthMiddleware(db)`: Intercepts all requests. Checks `r.Cookie("session_token")`. Calls `db.ValidateSession()`. If valid, uses `context.WithValue(r.Context(), userContextKey, user)` to map the User struct into the memory boundary of the request.
- `AdminMiddleware`: Examines `UserFromContext(r.Context()).Role == "admin"`. Emits `403 Forbidden` if false.

### `server/handlers_chat.go`
**Purpose**: The complex proxy logic for SSE streaming.
- `type chatRequest struct`: JSON parser for incoming POST payloads. Fields: `conversationId`, `provider`, `model`, `keyId`, `messages[]`, `temperature`, `top_p`, `max_tokens`.
- **`handleChat()` Flow**:
  1. Validates input JSON.
  2. Extracts the final user message in `req.Messages` and calls `db.SaveMessage()` to persist it.
  3. Prepares SSE headers on the ResponseWriter (`Content-Type: text/event-stream`).
  4. Retrieves the `Provider` interface instance from `providers.Registry`.
  5. Retrieves the raw API key from `db.GetKeyValue(req.KeyId)`.
  6. Maps `temperature`, `top_p`, `max_tokens` into a `providers.Options` struct.
  7. Invokes `Provider.ChatStream(...)`.
  8. Enters a `bufio.Scanner` `for` loop over the returned `io.ReadCloser`.
  9. For each buffer line, writes `data: {line}\n\n` to the response and forces a flush using `http.Flusher`.
  10. Accumulates the string fragments into a master variable.
  11. On exit, calls `db.SaveMessage` using role `assistant` and the master accumulated string. Emits `data: [DONE]\n\n`.

### `server/handlers_auth.go` & `server/handlers_keys.go`
**Purpose**: Basic REST mappings.
- Parse JSON using DRY generic functions (`decodeJSON` and `writeJSON`).
- Map inputs directly to `db.CreateX`, `db.DeleteX`, returning standard HTTP status codes (`201 Created`, `204 No Content`).
- `handleSetup` strictly blocks registration if `db.HasUsers()` is true, ensuring secure bootstrapping.

---

## 6. LLM Provider Registry (`providers/`)

The plugin architecture that normalizes varying remote APIs into a single Go Interface.

### `providers/provider.go`
**Interface Contract**:
```go
type Options struct { Temperature, TopP float32; MaxTokens int }
type Provider interface {
    ChatStream(ctx, model, apiKey, baseURL, messages, opts) (io.ReadCloser, error)
    ListModels(ctx, apiKey, baseURL) ([]string, error)
}
```

### `providers/registry.go`
**Purpose**: Holds map of provider singletons.
- Returns implementations when given strings like `"openai"`, `"anthropic"`, `"gemini"`.

### `providers/ollama.go`
**Target**: Local HTTP JSON endpoints.
- **Payload Translation**: Packages `opts` fields into the nested `"options"` JSON object mandated by the Ollama API.
- **Parsing**: Parses Newline-Delimited JSON (NDJSON). Scans `chunk.Message.Content`, ignores empty whitespaces, and writes pure string bytes into an `io.PipeWriter` to conform to the `io.ReadCloser` interface constraint.

### `providers/openai.go`
**Target**: `/v1/chat/completions`.
- **Payload Translation**: Places `temperature`, `top_p`, `max_tokens` at the root JSON level alongside `messages` and `model`.
- **Parsing**: Standard OpenAI SSE protocol. Strips `data: `, unmarshals `chunk.Choices[0].Delta.Content`. Bypasses lines equal to `[DONE]`.

### `providers/anthropic.go`
**Target**: `/v1/messages`.
- **Edge Case Mapping**: Anthropic prohibits `"system"` roles within the messages array. This file dynamically loops the messages, extracts the `system` role, deletes it from the array, and moves the text to a top-level `"system"` string attribute in the root payload.
- **Parsing**: Checks for `chunk.Type == "content_block_delta"`, then extracts `chunk.Delta.Text`.

### `providers/gemini.go`
**Target**: Google Generative Language REST APIs.
- **Payload Translation**: Roles must be `"user"` or `"model"` (translates `"assistant"`). Formats `opts` into a deeply nested `generationConfig` block mapping `MaxTokens` to `maxOutputTokens`.
- **Parsing**: Unpacks highly nested JSON trees: `chunk.Candidates[0].Content.Parts[0].Text`.

---

## 7. Frontend Build & Assets (`web/`)

### `web/web.go`
**Call Graph**: Invoked by `server/server.go`.
- Uses `//go:embed all:static` to compile the `web/static/` subfolder into the binary memory map.
- Exports a singleton `http.Handler` via `http.FileServer(http.FS(subFS))`.

### `web/static/index.html`
**DOM Layout Graph**:
- **`<div id="app">`**: Root viewport constraint.
  - **`<nav id="sidebar">`**: Left panel.
    - `div.logo`: Text header.
    - `button#btn-new-chat`: Reset context.
    - `input#search-conv`: Text input for sidebar fuzzy-matching.
    - `div#conversation-list`: Overflow container mapped by JS.
    - `div.sidebar-footer`: Contains Gear icon (`#btn-settings`) for global parameters.
  - **`<main id="main">`**: Right panel.
    - **`<header id="toolbar">`**: Top horizontal bar.
      - Includes the `<select>` inputs for `provider`, `model`, `key`. 
      - Contains `#btn-chat-settings` for inference parameters.
      - Contains `#theme-toggle` (Sun/Moon icon).
    - **`<div id="chat-area">`**:
      - `div#welcome`: Preset prompt chips.
      - `div#messages`: Dynamic conversational flow.
    - **`<div id="input-area">`**:
      - Textarea `#msg-input` for multi-line inputs.
      - `#btn-send` (Send Arrow) and `#btn-stop` (Stop Streaming square).
- **Modals**: `#modal-settings`, `#modal-rename`, `#modal-chat-settings`. Absolute positioned z-index overlays.

---

## 8. CSS Styling Engine (`web/static/css/`)

Engineered with BEM-inspired naming and robust Flexbox grids, adhering to strict DOM boundaries.

### `tokens.css`
The foundation of visual aesthetics.
- Maps `:root` to a set of Dark Mode variable constants.
- Contains a `[data-theme="light"]` attribute block that heavily mutates `--col-bg`, `--col-surface`, `--col-text`, and `--col-border` to bright, readable palettes seamlessly.
- Exposes glow filters via `--col-accent-glow` for button hover states.

### `layout.css`
Structural enforcement.
- Ensures the `body` and `#app` tags use `overflow: hidden; height: 100vh;` to completely nullify accidental viewport elasticity in browsers.
- Forces independent scrolling contexts specifically on `#conversation-list` and `#messages` using `overflow-y: auto`.

### `components.css`
Widget-specific paint rules.
- **`.msg-bubble`**: Controls margin layouts. Assumes Flexbox wrapping.
- **`.conv-actions` / `.msg-actions`**: Hidden on default (`opacity: 0`). Uses CSS `:hover` states on parent containers to fade them into view, providing "Edit" and "Regenerate" UI buttons without crowding the display.
- **Markdown Tokens**: Explicit `.syntax-keyword`, `.syntax-string` colors applied for the zero-dependency syntax highlighter.

---

## 9. Client-Side JS Logic (`web/static/js/`)

A masterclass in memory-lean Vanilla JavaScript using ES6 Modules and native web APIs.

### `state.js`
The central brain for UI relativity.
- `window.State`: A mutable JS object holding `conversations`, `messages`, `provider`, `model`, `theme`, `searchQuery`, and `chatSettings` (Temp, TopP).
- Functions mutate these variables and broadcast state boundaries.

### `main.js`
The Bootstrapper.
- Triggers on `DOMContentLoaded`.
- Calls `loadConversations()` and `loadKeys()` concurrently via `Promise.all`.
- Attaches massive delegator events: Keydowns (Ctrl+K shortcuts, Enter to send without shift), Dropdown changes, Theme toggle triggers.
- Injects `textarea` automatic vertical height resizing logic bounded by a `max-height` pixel limit.

### `auth.js`
Authentication interception pipeline.
- Overrides `window.fetch` using a proxy function. If any XHR/Fetch request to the server returns `401 Unauthorized`, it globally halts the app, deletes local context, and throws the un-closable Auth Login/Setup full-screen modal.
- Controls the Admin Panel UI generation logic for `/api/users`.

### `ui.js`
The DOM manipulation workhorse.
- **`renderConversationList()`**: Rebuilds the sidebar. Applies CSS `.hidden` classes to rows that fail to substring-match `State.searchQuery`, offering instant filtering.
- **`appendMessage()`**: Dynamically constructs the HTML string for message bubbles. Crucially, it attaches the `msg-actions` div container (containing the SVG icons for Edit and Regenerate) only if the message passes logic gates (e.g., Regenerate only on Assistant messages, Edit only on User messages).
- **`applyTheme()`**: Alters the `data-theme` attribute on `document.documentElement` and swaps the inner SVG paths for the Moon/Sun toggle.

### `chat.js`
API coordination and SSE consumption.
- **`sendMessage()`**: Validates input. Bypasses execution if `State.streaming === true`. Creates JSON payload.
- **`regenerateMessage(id)`**: Advanced workflow. Locates the targeted message. Deletes all subsequent messages in memory. Executes HTTP `DELETE` calls to the `/api/messages/{id}` endpoint to purge them from the SQLite database. Invokes `startStreaming()` to request a fresh LLM generation.
- **`editMessage(id)`**: Identical purge logic to `regenerate`, but instead of streaming, it rips the previous text payload and drops it back into the `#msg-input` textarea, focusing the cursor for the user to rewrite.
- **`startStreaming()`**:
  - Initializes `new AbortController()` to allow mid-generation stoppage.
  - Uses `fetch` to POST to `/api/chat`.
  - Captures `res.body.getReader()`. Loops over `reader.read()` byte arrays.
  - Accumulates text, passes it into `window.renderMarkdown()`, and progressively edits the `.msg-content` DOM node innerHTML, generating the "typing" visual effect.

### `markdown.js`
Zero-dependency Markdown parser.
- Escapes arbitrary `<` and `>` tags to prevent XSS payloads.
- **Fenced Code Blocks**: Locates triple-backticks. Randomly generates UUID `pre` block IDs. Implements a regex mapping sequence for basic lexical highlighting (injecting `<span class="syntax-*">` for JS/Python/Go keywords, integers, and quoted strings).
- Injects a floating `Copy` button relative to every code block container that interacts with `navigator.clipboard.writeText`.
- Formats standard Markdown syntax for `**bold**`, `*italics*`, `> quotes`, and list indentations using regex look-aheads and capture groups.

---
*End of Documentation. Using this guide, a developer can perfectly trace any behavior, database transition, or visual rendering step in the application.*
