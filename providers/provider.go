package providers

import (
	"context"
	"io"
)

// Message is a single chat message sent to a provider.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Options contains LLM generation parameters
type Options struct {
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// Provider is the interface every LLM backend implements.
type Provider interface {
	// ChatStream sends messages and returns a streaming reader of tokens.
	ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message, opts Options) (io.ReadCloser, error)
	// ListModels returns available model IDs. apiKey and baseURL may be empty for local providers.
	ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error)
}
