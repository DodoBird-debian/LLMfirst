# LLM WebUI — Full Documentation

A single cross-platform Go binary that serves an embedded Web UI, proxies LLM API requests, and persists everything in a local SQLite database.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Directory Structure](#directory-structure)
3. [Building & Running](#building--running)
4. [CLI Flags](#cli-flags)
5. [Go Packages](#go-packages)
   - [main](#main)
   - [config](#config)
   - [db](#db)
   - [server](#server)
   - [providers](#providers)
   - [web](#web)
6. [REST API Reference](#rest-api-reference)
7. [Frontend Architecture](#frontend-architecture)
8. [Database Schema](#database-schema)
9. [Adding a New Provider](#adding-a-new-provider)
10. [Design Decisions](#design-decisions)

---

## Architecture Overview

```
Browser → http://localhost:42068
              │
              ▼
         Go HTTP Server  (chi router + middleware)
              │
         ┌────┴──────────────────────┐
         │                           │
    Static Files               API Handlers
    (embedded via              /api/chat (SSE)
     go:embed)                 /api/conversations
                               /api/keys
                               /api/models
                               /api/ollama/*
                                    │
                               ┌────┴────┐
                           Provider    SQLite
                           Registry     DB
                           (Ollama,   (data.db)
                            OpenAI,
                            Anthropic,
                            Gemini)
```

Everything ships as **one binary** (`llm-webui.exe` on Windows, `llm-webui` on Linux/Mac). The SQLite `.db` file is created automatically alongside the binary if it doesn't exist.

---

## Directory Structure

```
llm-webui/
├── main.go                   # Entry point
├── go.mod                    # Module definition & dependencies
├── go.sum                    # Dependency checksums
│
├── config/
│   └── config.go             # CLI flag parsing → Config struct
│
├── db/
│   ├── db.go                 # OpenDB(), Migrate() — SQLite init
│   ├── schema.sql            # Embedded SQL schema (tables + indexes)
│   ├── conversations.go      # Conversation & Message CRUD
│   └── keys.go               # APIKey CRUD + Settings key/value store
│
├── server/
│   ├── server.go             # Chi router setup, middleware, route table
│   ├── handlers_chat.go      # POST /api/chat (SSE), conversation handlers
│   ├── handlers_keys.go      # API key CRUD handlers
│   └── handlers_models.go    # GET /api/models, /api/health, Ollama handlers
│
├── providers/
│   ├── provider.go           # Provider interface definition
│   ├── registry.go           # Registry — holds all providers, lookup by name
│   ├── ollama.go             # Ollama (local) implementation
│   ├── openai.go             # OpenAI (and compatible) implementation
│   ├── anthropic.go          # Anthropic Claude implementation
│   └── gemini.go             # Google Gemini implementation
│
└── web/
    ├── web.go                # go:embed wrapper → http.Handler
    └── static/
        ├── index.html        # Single-page app shell
        ├── css/
        │   ├── tokens.css    # Design tokens (colors, spacing, fonts)
        │   ├── layout.css    # App shell, sidebar, main, input area
        │   ├── components.css# Buttons, selects, messages, modals, chips
        │   └── animations.css# Keyframes (fadeIn, slideUp, typing, pulse…)
        └── js/
            ├── markdown.js   # Lightweight MD → HTML renderer
            ├── state.js      # Global mutable app state object
            ├── ui.js         # DOM helpers (render conversations, messages…)
            ├── chat.js       # SSE streaming, conversation lifecycle
            └── main.js       # DOMContentLoaded — wires all event listeners
```

---

## Building & Running

### Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go   | 1.21+   | https://go.dev/dl |
| Git  | any     | Required by Go toolchain for module resolution |

### First-time setup

```powershell
cd C:\Users\dodobird\.gemini\antigravity-ide\scratch\llm-webui
go mod tidy          # Downloads all dependencies
```

### Build

```powershell
go build -o llm-webui.exe .
```

Produces a single self-contained binary. No runtime dependencies.

### Run

```powershell
.\llm-webui.exe --port 42068 --db .\data.db
```

Open **http://localhost:42068** in your browser.

### Cross-compile (optional)

```powershell
# Linux binary from Windows
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -o llm-webui .

# macOS binary from Windows
$env:GOOS="darwin"; $env:GOARCH="arm64"
go build -o llm-webui-mac .
```

---

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `42068` | HTTP port to listen on |
| `--host` | `127.0.0.1` | Host/IP to bind to. Use `0.0.0.0` to expose on LAN |
| `--db` | `./data.db` | Path to the SQLite database file |
| `--ollama-url` | `http://localhost:11434` | Default Ollama base URL (overridden by saved setting) |

**Examples:**

```powershell
# Default — localhost only, default port
.\llm-webui.exe

# Custom port
.\llm-webui.exe --port 8080

# Expose on local network (all interfaces)
.\llm-webui.exe --host 0.0.0.0 --port 42068

# Use a different database file (e.g. work vs personal)
.\llm-webui.exe --db .\work.db
```

---

## Go Packages

### `main`

**File:** [`main.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/main.go)

The binary entry point. Responsible for:

1. Parsing CLI flags via `config.ParseFlags()`
2. Opening + migrating the SQLite DB via `db.OpenDB()` / `db.Migrate()`
3. Creating the HTTP router via `server.NewRouter()`
4. Starting `http.ListenAndServe`

```go
func main() {
    cfg := config.ParseFlags()
    sqlDB, _ := db.OpenDB(cfg.DBPath)
    db.Migrate(sqlDB)
    r := server.NewRouter(sqlDB, cfg)
    http.ListenAndServe(cfg.Host+":"+cfg.Port, r)
}
```

---

### `config`

**File:** [`config/config.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/config/config.go)

Parses command-line flags into a `Config` struct.

```go
type Config struct {
    Port      string
    Host      string
    DBPath    string
    OllamaURL string
}

func ParseFlags() Config
```

---

### `db`

**Files:** [`db/db.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/db/db.go), [`db/conversations.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/db/conversations.go), [`db/keys.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/db/keys.go)

#### `db.go` — Initialization

```go
// OpenDB opens (or creates) a SQLite database at the given path.
// Enables WAL mode and foreign keys.
func OpenDB(path string) (*sql.DB, error)

// Migrate runs the embedded schema.sql against the database.
// Safe to call on an existing DB — uses CREATE TABLE IF NOT EXISTS.
func Migrate(db *sql.DB) error
```

The schema is embedded at compile time using `//go:embed schema.sql`.

#### `conversations.go` — Conversation & Message CRUD

```go
type Conversation struct {
    ID           string    `json:"id"`            // UUID v4
    Title        string    `json:"title"`
    Provider     string    `json:"provider"`
    Model        string    `json:"model"`
    SystemPrompt string    `json:"system_prompt"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type Message struct {
    ID             int64     `json:"id"`
    ConversationID string    `json:"conversation_id"`
    Role           string    `json:"role"`           // "user" | "assistant" | "system"
    Content        string    `json:"content"`
    TokensUsed     int       `json:"tokens_used"`
    CreatedAt      time.Time `json:"created_at"`
}

func CreateConversation(db, provider, model, systemPrompt) (*Conversation, error)
func GetConversation(db, id) (*Conversation, error)
func ListConversations(db) ([]Conversation, error)     // ordered by updated_at DESC
func UpdateConversation(db, id, title, systemPrompt) error
func DeleteConversation(db, id) error                  // cascades to messages

func SaveMessage(db, conversationID, role, content, tokensUsed) (*Message, error)
func GetMessages(db, conversationID) ([]Message, error)
func DeleteMessage(db, id) error
```

#### `keys.go` — API Keys + Settings

```go
type APIKey struct {
    ID        int64
    Provider  string   // "openai" | "anthropic" | "gemini" | "ollama"
    Label     string   // human-readable name
    KeyValue  string   // redacted (****xxxx) when listed, raw when fetched by ID
    BaseURL   string   // optional custom base URL for this key
    CreatedAt time.Time
}

func CreateKey(db, provider, label, keyValue, baseURL) (*APIKey, error)
func ListKeys(db) ([]APIKey, error)       // key_value is redacted (last 4 chars shown)
func GetKeyValue(db, id) (string, string, error)  // returns (rawKey, baseURL, err)
func UpdateKey(db, id, label, baseURL) error
func DeleteKey(db, id) error

// Settings: simple key/value store (e.g. "ollama_url")
func GetSetting(db, key) (string, error)
func SetSetting(db, key, value) error     // upserts
```

> [!IMPORTANT]
> `ListKeys` **redacts** the key value — only the last 4 characters are returned to the browser. The full key is only ever accessed server-side via `GetKeyValue`.

---

### `server`

**Files:** [`server/server.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/server/server.go), [`handlers_chat.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/server/handlers_chat.go), [`handlers_keys.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/server/handlers_keys.go), [`handlers_models.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/server/handlers_models.go)

#### `server.go` — Router

Uses [chi](https://github.com/go-chi/chi) as the HTTP router. Sets up:

- `chi.Logger` middleware — logs every request with method, path, status, size, duration
- `chi.Recoverer` — catches panics and returns 500
- All `/api/*` routes
- Fallback `/*` route → serves embedded static files

```go
func NewRouter(db *sql.DB, cfg config.Config) http.Handler
```

#### `handlers_chat.go` — Chat + Conversation Handlers

| Handler | Purpose |
|---------|---------|
| `handleChat` | `POST /api/chat` — streams SSE tokens from provider |
| `handleListConversations` | `GET /api/conversations` |
| `handleCreateConversation` | `POST /api/conversations` |
| `handleGetConversation` | `GET /api/conversations/{id}` — returns conversation + all messages |
| `handleUpdateConversation` | `PUT /api/conversations/{id}` — title / system prompt |
| `handleDeleteConversation` | `DELETE /api/conversations/{id}` |
| `handleAddMessage` | `POST /api/conversations/{id}/messages` |
| `handleDeleteMessage` | `DELETE /api/messages/{id}` |

**SSE Streaming flow in `handleChat`:**

1. Decode request body → `chatRequest{provider, model, keyId, messages}`
2. Persist the user message to DB
3. Set `Content-Type: text/event-stream` headers
4. Look up raw API key from DB by `keyId`
5. Call `provider.ChatStream(ctx, model, apiKey, baseURL, messages)`
6. Read the returned `io.ReadCloser` line-by-line
7. Forward each line as `data: <token>\n\n`
8. After stream ends, persist the full assistant response to DB
9. Send `data: [DONE]\n\n`

#### `handlers_keys.go` — Key Management

| Handler | Purpose |
|---------|---------|
| `handleListKeys` | `GET /api/keys` — returns all keys with redacted values |
| `handleCreateKey` | `POST /api/keys` |
| `handleUpdateKey` | `PUT /api/keys/{id}` |
| `handleDeleteKey` | `DELETE /api/keys/{id}` |

#### `handlers_models.go` — Models + Health

| Handler | Purpose |
|---------|---------|
| `handleHealth` | `GET /api/health` — returns `{"status":"ok","db":"connected"}` |
| `handleModels` | `GET /api/models?provider=X&keyId=Y` — calls `provider.ListModels()` |
| `handleOllamaStatus` | `GET /api/ollama/status` — checks if Ollama is reachable |
| `handleOllamaURL` | `PUT /api/ollama/url` — updates the Ollama base URL in settings |

---

### `providers`

**Files:** [`providers/provider.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/provider.go), [`registry.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/registry.go), [`ollama.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/ollama.go), [`openai.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/openai.go), [`anthropic.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/anthropic.go), [`gemini.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/providers/gemini.go)

#### The `Provider` Interface

```go
type Message struct {
    Role    string `json:"role"`    // "user" | "assistant" | "system"
    Content string `json:"content"`
}

type Provider interface {
    // ChatStream initiates a streaming chat completion.
    // Returns an io.ReadCloser yielding raw token text (NOT SSE-formatted).
    // The caller (handleChat) is responsible for SSE framing.
    ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message) (io.ReadCloser, error)

    // ListModels returns available model IDs for this provider.
    // apiKey and baseURL may be empty for local providers (Ollama).
    ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error)
}
```

#### `Registry`

```go
type Registry struct { ... }

func NewRegistry(db *sql.DB, cfg config.Config) *Registry

// Get returns a Provider by name ("ollama", "openai", "anthropic", "gemini").
func (r *Registry) Get(name string) (Provider, error)

// Ollama returns the Ollama provider directly (for URL management).
func (r *Registry) Ollama() *OllamaProvider
```

At startup, `NewRegistry` reads the saved `ollama_url` from the settings table and initializes `OllamaProvider` with it.

#### Provider Implementations

| Provider | `ListModels` | `ChatStream` endpoint |
|----------|-------------|----------------------|
| **Ollama** | `GET {ollamaURL}/api/tags` | `POST {ollamaURL}/api/chat` (NDJSON → pipe) |
| **OpenAI** | `GET api.openai.com/v1/models` (filtered to `gpt-*`, `o1`, `o3`) | `POST /v1/chat/completions` (SSE) |
| **Anthropic** | `GET api.anthropic.com/v1/models` | `POST /v1/messages` (SSE) |
| **Gemini** | `GET generativelanguage.googleapis.com/v1beta/models` (filtered to `generateContent`-capable) | `POST .../models/{model}:streamGenerateContent?alt=sse` |

> [!TIP]
> The `baseURL` parameter on every provider method allows pointing to **OpenAI-compatible local servers** (LM Studio, vLLM, llama.cpp server, etc.) by using the OpenAI provider with a custom base URL.

---

### `web`

**File:** [`web/web.go`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/web/web.go)

```go
//go:embed all:static
var staticFiles embed.FS

// Handler returns an http.FileServer serving everything under web/static/.
func Handler() http.Handler
```

The `//go:embed all:static` directive tells the Go compiler to bundle the entire `static/` directory into the binary at compile time. No files need to be distributed alongside the binary.

---

## REST API Reference

All endpoints are under `http://localhost:42068`.

### Health

```
GET /api/health
→ {"status":"ok","db":"connected"}
```

### Conversations

```
GET    /api/conversations
→ [{"id":"uuid","title":"...","provider":"gemini","model":"gemini-2.0-flash","updated_at":"..."}]

POST   /api/conversations
Body:  {"provider":"gemini","model":"gemini-2.0-flash","system_prompt":""}
→ 201 {"id":"uuid","title":"New Chat",...}

GET    /api/conversations/{id}
→ {"conversation":{...},"messages":[{"role":"user","content":"..."},{"role":"assistant","content":"..."}]}

PUT    /api/conversations/{id}
Body:  {"title":"My Chat","system_prompt":"You are a pirate."}
→ {"status":"ok"}

DELETE /api/conversations/{id}
→ 204 No Content
```

### Messages

```
POST   /api/conversations/{id}/messages
Body:  {"role":"user","content":"Hello"}
→ 201 {"id":42,"role":"user","content":"Hello",...}

DELETE /api/messages/{id}
→ 204 No Content
```

### Chat (SSE Streaming)

```
POST   /api/chat
Body:  {
  "conversationId": "uuid-or-empty",
  "provider":       "gemini",
  "model":          "gemini-2.0-flash",
  "keyId":          4,
  "messages":       [{"role":"user","content":"Hello!"}]
}

Response: text/event-stream
data: Hello
data: ,
data:  world
data: !
data: [DONE]
```

Each `data:` line is a raw token chunk. The frontend accumulates them into a full string for markdown rendering.

### Models

```
GET /api/models?provider=gemini&keyId=4
→ ["gemini-2.0-flash","gemini-1.5-pro","gemini-1.5-flash",...]

GET /api/models?provider=ollama&keyId=0
→ ["llama3.2:latest","mistral:7b",...]
```

### API Keys

```
GET    /api/keys
→ [{"id":1,"provider":"openai","label":"My Key","key_value":"****xyzw","base_url":""}]
   Note: key_value is always redacted in this response.

POST   /api/keys
Body:  {"provider":"gemini","label":"AI Studio","key_value":"AIzaSy...","base_url":""}
→ 201 {"id":5,"provider":"gemini","label":"AI Studio","base_url":""}

PUT    /api/keys/{id}
Body:  {"label":"New Label","base_url":"http://custom-proxy.example.com"}
→ {"status":"ok"}

DELETE /api/keys/{id}
→ 204 No Content
```

### Ollama

```
GET /api/ollama/status
→ {"detected":true,"url":"http://localhost:11434","models":["llama3.2:latest"]}

PUT /api/ollama/url
Body: {"url":"http://192.168.1.100:11434"}
→ {"status":"ok","models":["...remote models..."]}
```

---

## Frontend Architecture

The frontend is a **vanilla JS SPA** — no framework, no build step.

### CSS layers

| File | Responsibility |
|------|---------------|
| `tokens.css` | CSS custom properties — all colors, spacing, radii, fonts, transitions |
| `layout.css` | App shell grid, sidebar, main panel, chat area, input area |
| `components.css` | Every reusable component: buttons, selects, message bubbles, modals, chips, toasts, keys list |
| `animations.css` | All `@keyframes` declarations |

Everything uses CSS custom properties from `tokens.css`. Adding a light mode is as simple as overriding those variables under a `[data-theme="light"]` selector.

### JS modules (load order matters)

| File | Responsibility |
|------|---------------|
| `markdown.js` | `window.renderMarkdown(text)` — converts Markdown to HTML. Handles fenced code blocks, inline code, headers, bold/italic, lists, blockquotes, links, tables. Also `window.copyCode(preId)`. |
| `state.js` | `window.State` — single mutable object holding: `conversations[]`, `activeConvId`, `messages[]`, `provider`, `model`, `keyId`, `keys[]`, `streaming`, `abortController` |
| `ui.js` | All DOM manipulation functions: `renderConversationList()`, `appendMessage()`, `showTypingIndicator()`, `appendStreamBubble()`, `finalizeStreamBubble()`, `populateModels()`, `populateKeySelector()`, `renderKeysList()`, `setOllamaPill()`, `updateToolbarTitle()`, `toast()` |
| `chat.js` | `sendMessage()`, `startStreaming()`, `loadConversation()`, `createNewConversation()`, `deleteConversation()`, `startNewChat()`, `loadKeys()`, `deleteKey()`, `exportConversation()` |
| `main.js` | `DOMContentLoaded` handler — initial data load, all event listener wiring (selectors, buttons, keyboard shortcuts, modals, tabs) |

### Streaming flow (client side)

```
sendMessage()
  └─ createNewConversation() if needed
  └─ appendMessage('user', text)        ← immediate UI update
  └─ startStreaming(messages)
        └─ fetch POST /api/chat          ← SSE connection opens
        └─ showTypingIndicator()         ← animated dots
        └─ reader.read() loop
              └─ appendStreamBubble()    ← creates assistant bubble
              └─ contentEl.innerHTML = renderMarkdown(accumulated)  ← live update
        └─ finalizeStreamBubble()        ← cleans up stream IDs
        └─ State.messages.push(...)      ← update in-memory state
        └─ auto-title if first exchange
```

### Keyboard shortcuts

| Shortcut | Action |
|----------|--------|
| `Enter` | Send message |
| `Shift+Enter` | New line in textarea |
| `Ctrl+K` / `Cmd+K` | New chat |
| `Escape` | Close open modal |

---

## Database Schema

**File:** [`db/schema.sql`](file:///C:/Users/dodobird/.gemini/antigravity-ide/scratch/llm-webui/db/schema.sql)

```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    provider   TEXT NOT NULL,           -- 'openai' | 'anthropic' | 'gemini' | 'ollama'
    label      TEXT NOT NULL,           -- human-readable name
    key_value  TEXT NOT NULL,           -- stored in plaintext (local app)
    base_url   TEXT DEFAULT '',         -- optional custom endpoint
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,     -- UUID v4
    title         TEXT DEFAULT 'New Chat',
    provider      TEXT NOT NULL,
    model         TEXT NOT NULL,
    system_prompt TEXT DEFAULT '',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,      -- 'user' | 'assistant' | 'system'
    content         TEXT NOT NULL,
    tokens_used     INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

> [!NOTE]
> API keys are stored in **plaintext** in the local SQLite file. This is intentional — this is a local-first app running on your own machine. If you expose it on a network (`--host 0.0.0.0`), ensure the machine is behind a firewall.

---

## Adding a New Provider

1. **Create `providers/myprovider.go`:**

```go
package providers

import (
    "context"
    "io"
)

type MyProvider struct{}

func NewMyProvider() *MyProvider { return &MyProvider{} }

func (p *MyProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
    // Fetch live from your provider's API, or return a static list
    return []string{"my-model-v1", "my-model-v2"}, nil
}

func (p *MyProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message) (io.ReadCloser, error) {
    // Make HTTP request, return an io.ReadCloser that yields raw token text
    // (not SSE-formatted — the server handler adds the SSE framing)
    pr, pw := io.Pipe()
    go func() {
        defer pw.Close()
        // write tokens to pw...
    }()
    return pr, nil
}
```

2. **Register in `providers/registry.go`:**

```go
reg.providers = map[string]Provider{
    "ollama":     o,
    "openai":     NewOpenAIProvider(),
    "anthropic":  NewAnthropicProvider(),
    "gemini":     NewGeminiProvider(),
    "myprovider": NewMyProvider(),   // ← add here
}
```

3. **Add to the HTML provider selector in `web/static/index.html`:**

```html
<option value="myprovider">🔮 MyProvider</option>
```

That's it. No other changes needed.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Single binary** | Zero-install deployment — copy the binary and run it |
| **SQLite** | No database server needed, file is portable and swappable |
| **go:embed** | UI assets are compiled into the binary — no `static/` folder needed at runtime |
| **chi router** | Lightweight, idiomatic, excellent middleware support |
| **SSE (not WebSocket)** | Simpler — SSE is unidirectional server→client which is all streaming needs |
| **No JS framework** | Keeps the frontend simple, no build pipeline, no `node_modules` |
| **Provider interface** | Clean abstraction — adding a new LLM backend touches only one file |
| **Key redaction** | `ListKeys` never returns the raw key to the browser — only last 4 chars |
| **WAL mode** | SQLite WAL allows concurrent reads during writes — important for streaming + DB writes |
| **modernc/sqlite** | Pure-Go SQLite driver — no CGo required, so the binary is fully static |
