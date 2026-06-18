package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"strings"
)

// GeminiProvider implements Provider for Google Gemini.
type GeminiProvider struct{}

func NewGeminiProvider() *GeminiProvider { return &GeminiProvider{} }

func (p *GeminiProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: API key required to list models")
	}
	// Fall back to default if blank or if user pasted a full endpoint URL
	if baseURL == "" || strings.Contains(baseURL, "streamGenerateContent") || strings.Contains(baseURL, "/models/") {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	base, err := neturl.Parse(baseURL + "/models")
	if err != nil {
		return nil, err
	}
	q := base.Query()
	q.Set("key", strings.TrimSpace(apiKey))
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", base.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("gemini ListModels %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Models []struct {
			Name               string   `json:"name"`
			SupportedMethods   []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range result.Models {
		// Only include models that support generateContent / streamGenerateContent
		for _, method := range m.SupportedMethods {
			if method == "generateContent" || method == "streamGenerateContent" {
				// Strip "models/" prefix
				name := strings.TrimPrefix(m.Name, "models/")
				names = append(names, name)
				break
			}
		}
	}
	return names, nil
}

func (p *GeminiProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message) (io.ReadCloser, error) {
	if baseURL == "" || strings.Contains(baseURL, "streamGenerateContent") || strings.Contains(baseURL, "/models/") {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	var contents []content
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			continue
		}
		contents = append(contents, content{Role: role, Parts: []part{{Text: m.Content}}})
	}

	payload := map[string]interface{}{"contents": contents}
	body, _ := json.Marshal(payload)

	streamURL, err := neturl.Parse(fmt.Sprintf("%s/models/%s:streamGenerateContent", baseURL, model))
	if err != nil {
		return nil, err
	}
	sq := streamURL.Query()
	sq.Set("key", strings.TrimSpace(apiKey))
	sq.Set("alt", "sse")
	streamURL.RawQuery = sq.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", streamURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("gemini: %d — %s", resp.StatusCode, string(bodyBytes))
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
			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				fmt.Fprint(pw, chunk.Candidates[0].Content.Parts[0].Text)
			}
		}
	}()

	return pr, nil
}
