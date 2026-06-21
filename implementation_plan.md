# Phase 2: Multimodal Vision & Modular File Processing

Based on user feedback, we will keep the integration simple but modular. Image support will be strictly limited to **Gemini** and **Ollama**. We will also introduce a modular file processor to handle file extraction, paving the way for future offloading or heavy (10k LOC) PDF analysis modules.

## Proposed Architecture

### 1. Modular File Processor (`pkg/fileprocessor/`)
Create an independent module responsible for analyzing files:
- **Input**: File path and MIME type.
- **Output**: Extracted Text (string) AND/OR Base64 Image Data (string).
- **Why**: Keeps `handlers_files.go` incredibly thin. If you want to add heavy PDF OCR or offload file processing to a separate microservice later, you only change this module.

### 2. Core Data Structures
Modify the `Message` struct in `providers/registry.go` to hold images:
```go
type Message struct {
    Role    string   `json:"role"`
    Content string   `json:"content"`
    Images  []string `json:"images,omitempty"` // Base64 encoded image data without data URI prefix
}
```

### 3. Backend Handlers (`handlers_chat.go`)
- Before sending a message to a provider, retrieve attachments.
- **Text Attachments**: Prepend to the system prompt (works for all providers).
- **Image Attachments**: 
  - If provider == `gemini` or `ollama`: Append to the `Images` slice of the latest user `Message`.
  - If provider == `openai` or `anthropic`: Ignore image attachments (or warn the user).

### 4. Provider Updates
Update only two providers to handle the new `Images` array:

#### [MODIFY] `providers/gemini.go`
Gemini expects images as `inlineData` in its `parts` array. We will parse the Base64 string, guess the MIME type, and build the correct JSON payload.

#### [MODIFY] `providers/ollama.go`
Ollama expects images as a simple array of Base64 strings in the `images` property of the message object.

---

## Execution Ready
This plan has been approved by the user and execution is currently underway in the task list.
