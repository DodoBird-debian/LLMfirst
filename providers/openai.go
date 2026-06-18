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

// OpenAIProvider implements Provider for OpenAI and compatible APIs.
type OpenAIProvider struct{}

func NewOpenAIProvider() *OpenAIProvider { return &OpenAIProvider{} }

func (p *OpenAIProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	if apiKey == "" {
		// Return a sensible default list when no key is configured yet
		return []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo"}, nil
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: ListModels status %d", resp.StatusCode)
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
		// Filter to chat-capable models only
		if strings.HasPrefix(m.ID, "gpt-") || strings.HasPrefix(m.ID, "o1") || strings.HasPrefix(m.ID, "o3") {
			names = append(names, m.ID)
		}
	}
	return names, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message) (io.ReadCloser, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	type oaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var oaiMsgs []oaiMsg
	for _, m := range messages {
		oaiMsgs = append(oaiMsgs, oaiMsg{Role: m.Role, Content: m.Content})
	}

	payload := map[string]interface{}{
		"model":    model,
		"stream":   true,
		"messages": oaiMsgs,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai: %d — %s", resp.StatusCode, string(bodyBytes))
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 {
				fmt.Fprint(pw, chunk.Choices[0].Delta.Content)
			}
		}
	}()

	return pr, nil
}
