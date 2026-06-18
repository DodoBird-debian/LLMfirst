package providers

import (
	"database/sql"

	appdb "github.com/dodobird/llm-webui/db"
	"github.com/dodobird/llm-webui/config"
)

// Registry holds all available provider implementations.
type Registry struct {
	providers map[string]Provider
	ollama    *OllamaProvider
}

// NewRegistry initializes providers using the given config.
func NewRegistry(db *sql.DB, cfg config.Config) *Registry {
	// Determine Ollama URL from settings or flag
	ollamaURL := cfg.OllamaURL
	if stored, err := appdb.GetSetting(db, "ollama_url"); err == nil && stored != "" {
		ollamaURL = stored
	}
	o := NewOllamaProvider(db, ollamaURL)

	reg := &Registry{ollama: o}
	reg.providers = map[string]Provider{
		"ollama":    o,
		"openai":    NewOpenAIProvider(),
		"anthropic": NewAnthropicProvider(),
		"gemini":    NewGeminiProvider(),
	}
	return reg
}

// Get returns the provider for the given name.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, errUnknownProvider(name)
	}
	return p, nil
}

// Ollama returns the Ollama provider directly.
func (r *Registry) Ollama() *OllamaProvider {
	return r.ollama
}

type unknownProviderError string

func errUnknownProvider(name string) error {
	return unknownProviderError(name)
}

func (e unknownProviderError) Error() string {
	return "unknown provider: " + string(e)
}
