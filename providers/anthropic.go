package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider implements Provider for Anthropic Claude.
type AnthropicProvider struct{}

func NewAnthropicProvider() *AnthropicProvider { return &AnthropicProvider{} }

func (p *AnthropicProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	if apiKey == "" {
		// Sensible defaults when no key configured
		return []string{"claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-3-5"}, nil
	}
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: ListModels status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range result.Data {
		names = append(names, m.ID)
	}
	return names, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message, opts Options) (io.ReadCloser, error) {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	var systemPrompt string
	var chatMsgs []map[string]string
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		chatMsgs = append(chatMsgs, map[string]string{"role": m.Role, "content": m.Content})
	}

	maxTokens := 8192
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}

	payload := map[string]interface{}{
		"model":      model,
		"stream":     true,
		"max_tokens": maxTokens,
		"messages":   chatMsgs,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	if opts.Temperature > 0 {
		payload["temperature"] = opts.Temperature
	}
	if opts.TopP > 0 {
		payload["top_p"] = opts.TopP
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic: %d — %s", resp.StatusCode, string(bodyBytes))
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if ctx.Err() != nil {
				break
			}
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Type == "content_block_delta" && chunk.Delta.Type == "text_delta" {
				fmt.Fprint(pw, chunk.Delta.Text)
			}
		}
	}()

	return pr, nil
}
