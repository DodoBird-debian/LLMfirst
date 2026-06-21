package providers

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	appdb "github.com/dodobird/llm-webui/db"
)

// OllamaProvider implements Provider for local Ollama instances.
type OllamaProvider struct {
	db      *sql.DB
	baseURL string
}

func NewOllamaProvider(db *sql.DB, baseURL string) *OllamaProvider {
	return &OllamaProvider{db: db, baseURL: baseURL}
}

func (o *OllamaProvider) BaseURL() string { return o.baseURL }

// SetBaseURL persists the new Ollama URL to settings and updates in memory.
func (o *OllamaProvider) SetBaseURL(db *sql.DB, url string) error {
	if err := appdb.SetSetting(db, "ollama_url", url); err != nil {
		return err
	}
	o.baseURL = url
	return nil
}

func (o *OllamaProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	url := o.baseURL
	if baseURL != "" {
		url = baseURL
	}
	resp, err := http.Get(url + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range result.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message, opts Options) (io.ReadCloser, error) {
	url := o.baseURL
	if baseURL != "" {
		url = baseURL
	}

	type ollamaMsg struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images,omitempty"`
	}
	var oMsgs []ollamaMsg
	for _, m := range messages {
		oMsg := ollamaMsg{Role: m.Role, Content: m.Content}
		for _, img := range m.Images {
			if strings.HasPrefix(img, "data:") {
				parts := strings.SplitN(img, ";base64,", 2)
				if len(parts) == 2 {
					oMsg.Images = append(oMsg.Images, parts[1])
				}
			} else {
				oMsg.Images = append(oMsg.Images, img)
			}
		}
		oMsgs = append(oMsgs, oMsg)
	}

	payload := map[string]interface{}{
		"model":    model,
		"stream":   true,
		"messages": oMsgs,
	}

	options := map[string]interface{}{}
	if opts.Temperature > 0 {
		options["temperature"] = opts.Temperature
	}
	if opts.TopP > 0 {
		options["top_p"] = opts.TopP
	}
	if opts.MaxTokens > 0 {
		options["num_predict"] = opts.MaxTokens
	}
	if len(options) > 0 {
		payload["options"] = options
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama: status %d", resp.StatusCode)
	}

	// Transform Ollama NDJSON into token lines
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				continue
			}
			if chunk.Done {
				break
			}
			token := chunk.Message.Content
			if strings.TrimSpace(token) != "" {
				fmt.Fprintf(pw, "%s", token)
			}
		}
	}()

	return pr, nil
}
