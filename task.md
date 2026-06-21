# Phase 2 Task List: Vision & Modularity

## Modular Architecture
- `[x]` Create `pkg/fileprocessor/fileprocessor.go`.
- `[x]` Implement `ProcessFile(filepath, mimeType) (text string, base64 string, err error)`
- `[x]` Refactor `server/handlers_files.go` to use `fileprocessor`.
- `[x]` Update `db.go` and `attachments.go` to store `base64_data`. Wait, I will just store Base64 in `extracted_text` to avoid schema migrations for now, or add `image_base64` column to SQLite schema. (Let's add a column to `schema.sql`).

## Core Structs
- `[x]` Update `providers.Message` in `providers/registry.go` to include `Images []string`.

## Backend Injection
- `[x]` Update `server/handlers_chat.go` to inject Base64 images into the user's last `Message.Images` array, ONLY if provider is `gemini` or `ollama`.

## Provider Modifications
- `[x]` Update `providers/gemini.go` to support `inlineData`.
- `[x]` Update `providers/ollama.go` to support `images` array.

## Verification
- `[ ]` Compile and run.
- `[ ]` Test uploading an image.
- `[ ]` Test querying the image with Gemini and Ollama.
