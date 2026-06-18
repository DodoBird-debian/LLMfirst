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
- **`schema.sql`**: Contains `CREATE TABLE IF NOT EXISTS` for four tables:
  1. `api_keys`: `id` (PK), `provider`, `label`, `key_value`, `base_url`, `created_at`.
  2. `conversations`: `id` (UUID PK), `title`, `provider`, `model`, `system_prompt`, `created_at`, `updated_at`.
  3. `messages`: `id` (PK AUTOINCREMENT), `conversation_id` (FK cascading), `role`, `content`, `token_count`, `created_at`.
  4. `settings`: `key` (PK), `value`.
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
Contains storage logic for API keys and global settings.
- **`type APIKey struct`**: Holds the credentials. Includes `base_url` for routing requests to proxy APIs (like LM Studio).
- **`func CreateKey(db *sql.DB, provider, label, keyValue, baseURL string) (*APIKey, error)`**: Basic `INSERT`.
- **`func ListKeys(db *sql.DB) ([]APIKey, error)`**: **CRITICAL SECURITY MEASURE**: When listing keys for the UI to display, this function manually overwrites the `KeyValue` struct property to `"****" + kv[len(kv)-4:]` (or just `"****"` if too short). This guarantees raw keys are never sent in bulk payload to the browser.
- **`func GetKeyValue(db *sql.DB, id int64) (string, string, error)`**: Fetches the **unredacted** raw key and `baseURL` for server-side API requests.
- **`func UpdateKey(db *sql.DB, id int64, label, baseURL string) error`**: Edits the non-secret metadata.
- **`func DeleteKey(db *sql.DB, id int64) error`**: Removes the key row entirely.
- **`func GetSetting(db *sql.DB, key string) (string, error)`**: Queries the `settings` table. Ignores `sql.ErrNoRows` returning `""`.
- **`func SetSetting(db *sql.DB, key, value string) error`**: UPSERT operation using SQLite's `ON CONFLICT(key) DO UPDATE SET value=excluded.value`.

---

## 4. REST API Handlers

### 4.1 `server/server.go`
- **`func NewRouter(db *sql.DB, cfg config.Config) http.Handler`**
  - **Middlewares**: Applies `chi.Logger`, `chi.Recoverer` (prevents crashing on panic), and `chi.RequestID`.
  - **Static Serving**: Maps `/*` to `web.Handler()`.
  - **Routing Table**: Maps all `/api/*` endpoints to their respective handler functions below.

### 4.2 `server/handlers_chat.go`
The primary operational logic of the proxy.
- **`type chatRequest struct`**: Deserializes JSON payloads containing `conversationId`, `provider`, `model`, `keyId`, and `messages`.
- **`func handleChat(db *sql.DB, reg *providers.Registry) http.HandlerFunc`**
  - **Edge Case handling**: Returns `400 Bad Request` if JSON is malformed.
  - **Persistence**: Grabs the last element of `req.Messages` (the user input) and passes it to `appdb.SaveMessage`.
  - **Headers**: Sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`. Casts the `ResponseWriter` to `http.Flusher`.
  - **Provider Lookup**: Grabs the provider from `reg.Get()`. Fetches unredacted key from `appdb.GetKeyValue()`.
  - **Streaming Loop**:
    1. Calls `ChatStream()`. Receives an `io.ReadCloser`.
    2. Opens a `bufio.Scanner` to read the token chunks.
    3. Prints `data: {token}\n\n` to the stream and immediately calls `flusher.Flush()`.
    4. Accumulates tokens into a `full` string variable.
  - **Post-Stream Processing**: Saves `full` string into DB as `assistant` message. Prints `data: [DONE]\n\n`.
- **CRUD Wrappers**:
  - `handleListConversations`: Calls `ListConversations`, writes JSON.
  - `handleCreateConversation`: Reads `provider`, `model`, `system_prompt`, calls `CreateConversation`. Returns `201 Created`.
  - `handleGetConversation`: Reads `chi.URLParam("id")`. Calls `GetConversation` and `GetMessages`. Merges them into a dictionary response. Returns `404` if not found.
  - `handleUpdateConversation`: Decodes `title` and `system_prompt`. Calls `UpdateConversation`.
  - `handleDeleteConversation`: Deletes conversation, returns `204 No Content`.
  - `handleAddMessage`: Appends message manually, returns `201`.
  - `handleDeleteMessage`: Deletes message manually, returns `204`.

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
- **`func writeJSON` / `func decodeJSON`**: Universal marshaling helpers to DRY up handlers.

---

## 5. LLM Provider Registry

The plugin architecture for backend LLM bridging.

### 5.1 `providers/provider.go`
- **`type Message struct`**: `{Role, Content}`.
- **`type Provider interface`**:
  1. `ChatStream(ctx, model, apiKey, baseURL, messages) (io.ReadCloser, error)`
  2. `ListModels(ctx, apiKey, baseURL) ([]string, error)`

### 5.2 `providers/registry.go`
- **`type Registry struct`**: Contains a map `map[string]Provider` and a specific pointer to `*OllamaProvider`.
- **`func NewRegistry`**: Fetches `ollama_url` from DB fallback to CLI. Instantiates all providers.
- **`func Get`**: Map lookup. Returns custom `errUnknownProvider` on miss.

### 5.3 `providers/ollama.go`
- **`SetBaseURL`**: Mutates the struct variable and triggers DB upsert.
- **`ListModels`**: `GET /api/tags`. Parses response `{"models": [{"name": "llama3"}]}` into string slice.
- **`ChatStream`**: 
  - Constructs `{"model": "x", "stream": true, "messages": [...]}`.
  - Submits POST to `/api/chat`.
  - **Edge Case**: Ollama sends NDJSON (Newline Delimited JSON). The scanner unpacks JSON strings line-by-line, reading `chunk.Message.Content`. It formats it into raw text and writes it to an `io.PipeWriter`, converting NDJSON into a standard byte stream for the caller.

### 5.4 `providers/openai.go`
- **`ListModels`**: Requires API key. Hits `/v1/models`. **Edge Case**: Specifically filters the `result.Data` array for strings prefixed with `"gpt-"`, `"o1"`, or `"o3"` to filter out irrelevant embedding or audio models.
- **`ChatStream`**: Hits `/v1/chat/completions`. Scans lines for `data: ` prefix. Unmarshals `chunk.Choices[0].Delta.Content`. Breaks on `[DONE]`.

### 5.5 `providers/anthropic.go`
- **`ListModels`**: Uses hardcoded defaults (`claude-opus-4-5`, etc.) if no key is present. Hits `/v1/models` passing `x-api-key` and `anthropic-version: 2023-06-01`.
- **`ChatStream`**: 
  - Iterates messages. **Edge Case**: Extracts the `system` message out of the array and promotes it to a top-level JSON field `{"system": "..."}`, as required by Claude's API design.
  - Submits to `/v1/messages`. Filters for `chunk.Type == "content_block_delta"`.

### 5.6 `providers/gemini.go`
- **`ListModels`**: Appends `?key=` to URL parameters. Filters `result.Models` for entities supporting `"generateContent"` or `"streamGenerateContent"`. Strips the redundant `"models/"` prefix from the names.
- **`ChatStream`**:
  - Translates role `"assistant"` to `"model"`.
  - Translates payload into `{ "contents": [ { "role": "x", "parts": [ {"text": "..."} ] } ] }`.
  - Targets `/models/{model}:streamGenerateContent?alt=sse`. Parses array `chunk.Candidates[0].Content.Parts[0].Text`.

---

## 6. Frontend Assets & Server

### 6.1 `web/web.go`
- Uses `//go:embed all:static` to compile the filesystem into binary.
- Extracts sub-filesystem via `fs.Sub(staticFiles, "static")` to strip directory prefixes. Returns `http.FileServer`.

### 6.2 `web/static/index.html`
The entire DOM hierarchy:
- **`#sidebar`**: Sidebar container.
  - `.logo`, `#btn-new-chat`
  - `#conversation-list`: Scrollable div.
  - `.sidebar-footer`: Contains `#btn-settings` and `#ollama-status`.
- **`#main`**: Main body content grid.
  - **`#toolbar`**: `#btn-sidebar-toggle`, Select dropdowns (`#sel-provider`, `#sel-model`, `#sel-key`), and dynamic Title display.
  - **`#chat-area`**: 
    - **`#welcome`**: The blank-slate view with four preset "chips".
    - **`#messages`**: The scrollable bubble container.
  - **`#input-area`**: The floating textbox textarea `#msg-input`, `#btn-send`, `#btn-stop`, character counts.
- **Modals**: `#modal-settings` and `#modal-rename`. Hidden by default using `style="display:none"`. Fixed overlays with blur backdrops.
- **Toasts**: `#toast-container` floating bottom-right.

---

## 7. CSS Styling Layers

The CSS architecture is split into purely logical responsibilities.

### 7.1 `web/static/css/tokens.css`
Declares CSS custom properties under `:root`.
- **Palette**: `--col-bg` (`#0d0f14`), surface shades, text layers (`--col-text`).
- **Accent**: `--col-accent` (`#7c6af7`), used across glows and active states.
- **Bubbles**: Defines specific background and border variables for user (`--col-user-bg`) and assistant.
- **Sizing**: Uses a scalable `--sp-` system (`0.25rem` up to `2rem`). Radius uses `--r-sm` to `--r-full`.

### 7.2 `web/static/css/layout.css`
Applies Grid and Flexbox rules for structural placement.
- `*, *::before, *::after`: Standard box-sizing resets.
- `#app`: `display: flex; height: 100vh; overflow: hidden;` prevents body scroll entirely.
- Sidebar collapses via `width: 0; min-width: 0; opacity: 0; pointer-events: none;` allowing GPU accelerated transitions.
- The `#chat-area` uses `overflow-y: auto; scrollbar-width: thin;` ensuring internal scrollbars are styled and constrained cleanly. The message box uses max-width constraints (`900px`) centered with `margin: 0 auto`.

### 7.3 `web/static/css/components.css`
Component-specific scopes mapping to UI widgets.
- **Buttons**: `.btn-new-chat`, `.btn-icon`, `.btn-primary`, `.btn-send`. Utilizes `:hover` pseudo-classes to invoke translation transforms (`translateY(-1px)`) and `--col-accent-glow` box-shadows.
- **Form Controls**: Select wrappers use an absolute positioned `▾` pseudo-element. Textareas are `resize: none`. Focus states trigger `box-shadow: 0 0 0 3px var(--col-accent-muted)`.
- **Conversation Items**: Nested structures with `.conv-actions` initially set to `opacity: 0`, and revealed upon `:hover` of parent `.conv-item`.
- **Chat Bubbles**: `.msg.user` uses `align-items: flex-end`, shifting content right. `.msg.assistant` shifts left. Uses distinct bottom-left or bottom-right border radii stripping.
- **Markdown Specifics**: Targets tags inside `.msg-bubble` (e.g. `p`, `h1`, `code`, `pre`). Blocks use specific syntax padding and copy button hover triggers.
- **Pills & Status Dots**: `.ollama-pill` shifts colors based on `.online` / `.offline` states.

### 7.4 `web/static/css/animations.css`
Defines keyframes:
- `@keyframes fadeIn`, `fadeOut`, `slideUp`, `msgIn`.
- `@keyframes typing`: Scaled pulse effect targeting `0%, 80%, 100%`.
- `@keyframes shimmer`: Animates the `background-position` of `.skeleton` loading states (for future component integrations).

---

## 8. Client-Side JavaScript

A masterclass in Vanilla JS state management.

### 8.1 `web/static/js/state.js`
The global `window.State` singleton.
- Variables: `conversations` array, `activeConvId` (string), `messages` array, `provider`, `model`, `keyId`, `keys` array, `streaming` bool, `abortController`.
- Helper methods: `setProvider()`, `setModel()`, `setKeyId()`, `getActiveConv()` filters the list.

### 8.2 `web/static/js/ui.js`
DOM modification logic.
- **`toast(msg, type)`**: Appends a `div.toast` to the DOM. Sets a `setTimeout` for 3200ms to invoke `.remove()`.
- **`renderConversationList()`**: Wipes and maps `State.conversations` into `.conv-item` elements. Injects event listeners for loading.
- **`renderMessages()`**: Clears DOM. Loops `State.messages`, calls `appendMessage`.
- **`appendMessage()`**: Generates user/assistant HTML. Modifies avatar icon (`U` or `⬡`). Drops to `scrollTop = scrollHeight`.
- **`showTypingIndicator()` / `removeTypingIndicator()`**: Injects the pulsing CSS dot block.
- **`appendStreamBubble()`**: Initializes an empty bubble assigned with `id="stream-bubble"`.
- **`finalizeStreamBubble()`**: Removes the unique DOM `id` identifiers so the next stream creates fresh DOM targets.
- **`populateModels()`**: Determines if the provider requires a key. Executes `fetch(/api/models)`. Maps result array into `<option>` nodes. Handles server errors aggressively via fallback `<option>` displays.
- **`populateKeySelector()`**: Filters `State.keys` by current `State.provider` and repopulates the dropdown.
- **`renderKeysList()`**: UI mapping for settings modal.
- **`openRenameModal()`**: Loads modal input. Overwrites `onclick` behavior dynamically to capture context `convId`.

### 8.3 `web/static/js/chat.js`
Business logic bridging UI to API.
- **`sendMessage()`**: 
  - Prevents send if `streaming == true` or text is blank.
  - Automatically triggers `createNewConversation()` if `activeConvId` is null.
  - Pushes message to `State.messages`. Resets `<textarea>` height. Invokes `startStreaming()`.
- **`startStreaming()`**:
  - Initializes `new AbortController()`.
  - Submits POST to `/api/chat`.
  - **Edge Case Parsing**: Extracts the response reader (`res.body.getReader()`). Implements an infinite `while(true)` reading loop. 
  - Splits chunks by `\n`. Processes `data: ` lines. Breaks on `[DONE]`. 
  - Progressively overwrites `contentEl.innerHTML` using `renderMarkdown()`.
  - Executes auto-titling logic via PUT request if conversation has less than 3 messages.
- **`loadConversation(id)`**: Fetches history. Edits `State`. Updates provider/model `<select>` elements to perfectly match the loaded conversation history. Swaps Welcome screen for Messages container.
- **`deleteConversation()`**: Prompt confirms. DELETEs API. Resets state to welcome screen if deleted item was active.
- **`startNewChat()`**: Resets DOM. Focuses textarea.
- **`exportConversation()`**: Loops `State.messages`, builds raw markdown strings, instantiates a `Blob`, creates a dynamic `<a>` object URL, and automatically clicks it to invoke browser download dialog.

### 8.4 `web/static/js/markdown.js`
The zero-dependency Markdown parser `window.renderMarkdown(text)`.
- Replaces HTML entities (`<`, `>`, `&`).
- Replaces Fenced code blocks (` ``` `). Injects randomly generated `id` attributes to pre blocks to uniquely map Copy buttons.
- Standard Regex mapping for inline code, `h1-h4`, blockquotes (`>`), italics (`*`, `_`), and lists (`-`, `*`, `+`, `1.`).
- Paragraph wrapping ensures dangling text is correctly spaced using negative lookaheads `^(?!<[a-z]|$)`.
- `copyCode(preId)`: Selects `textContent`, executes `navigator.clipboard.writeText()`, and provides temporary UI feedback.

### 8.5 `web/static/js/main.js`
The Application Bootstrapper bound to `DOMContentLoaded`.
- Evaluates `Promise.all` for `loadConversations`, `loadKeys`, `checkOllama`.
- Binds global `change` event listeners to dropdowns.
- Binds keyboard events:
  - Textarea captures `Enter` (without Shift) and executes `sendMessage()`.
  - Tracks `input` to fire `autoResize()` preventing vertical scroll overflow within bounds.
  - Global `keydown` traps `Ctrl+K` for new chats.
- Configures Modal click-outside-to-close behavior utilizing `e.target` delegation vs explicit modal boundaries.
- Loops UI tabs and sets `.active` states based on `data-tab` parameters.
- Triggers Welcome Chip clicks, dropping template prompts directly into the textarea and calling `focus()`.

---
*End of Comprehensive Documentation. Every file, interface, network constraint, and styling pipeline behaves strictly according to this blueprint.*
