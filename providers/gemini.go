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

	req, err := http.NewRequestWithContext(ctx, "GET", base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	
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

func (p *GeminiProvider) ChatStream(ctx context.Context, model, apiKey, baseURL string, messages []Message, opts Options) (io.ReadCloser, error) {
	if baseURL == "" || strings.Contains(baseURL, "streamGenerateContent") || strings.Contains(baseURL, "/models/") {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	type inlineData struct {
		MimeType string `json:"mime_type"`
		Data     string `json:"data"`
	}
	type part struct {
		Text       string      `json:"text,omitempty"`
		InlineData *inlineData `json:"inline_data,omitempty"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	var contents []content
	var systemInstruction *content
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			systemInstruction = &content{Role: "system", Parts: []part{{Text: m.Content}}}
			continue
		}

		var parts []part
		if m.Content != "" {
			parts = append(parts, part{Text: m.Content})
		}
		for _, img := range m.Images {
			if strings.HasPrefix(img, "data:") {
				partsArr := strings.SplitN(img, ";base64,", 2)
				if len(partsArr) == 2 {
					mimeType := strings.TrimPrefix(partsArr[0], "data:")
					parts = append(parts, part{InlineData: &inlineData{MimeType: mimeType, Data: partsArr[1]}})
				}
			}
		}
		// If empty, ensure at least empty text to prevent API error
		if len(parts) == 0 {
			parts = append(parts, part{Text: " "})
		}

		contents = append(contents, content{Role: role, Parts: parts})
	}

	payload := map[string]interface{}{"contents": contents}
	if systemInstruction != nil {
		payload["system_instruction"] = map[string]interface{}{
			"parts": systemInstruction.Parts,
		}
	}

	genConfig := map[string]interface{}{}
	if opts.Temperature > 0 {
		genConfig["temperature"] = opts.Temperature
	}
	if opts.TopP > 0 {
		genConfig["topP"] = opts.TopP
	}
	if opts.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = opts.MaxTokens
	}
	if len(genConfig) > 0 {
		payload["generationConfig"] = genConfig
	}
	body, _ := json.Marshal(payload)

	streamURL, err := neturl.Parse(fmt.Sprintf("%s/models/%s:streamGenerateContent", baseURL, model))
	if err != nil {
		return nil, err
	}
	sq := streamURL.Query()
	sq.Set("alt", "sse")
	streamURL.RawQuery = sq.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", streamURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))

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
			if ctx.Err() != nil {
				break
			}
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
