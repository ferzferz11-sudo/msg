package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"LavenderMessenger/core/llm"
)

// Provider — реализация LLMProvider для OpenRouter
// Полностью самостоятельная реализация, не зависит от owl.go
type Provider struct {
	apiKey string
	model  string
}

func NewProvider(apiKey, model string) *Provider {
	if model == "" {
		model = "openrouter/auto"
	}
	return &Provider{
		apiKey: apiKey,
		model:  model,
	}
}

func (p *Provider) ModelID() string {
	return "openrouter/" + p.model
}

func (p *Provider) Close() error {
	return nil // HTTP stateless
}

func (p *Provider) StreamChat(
	ctx context.Context,
	messages []llm.Message,
	tools []llm.ToolDef,
) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk, 64)

	// Конвертируем messages в OpenRouter API format
	ormsgs := make([]map[string]interface{}, 0, len(messages))
	for _, m := range messages {
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}
		// Если есть картинки — форматируем как multimodal content
		if len(m.Images) > 0 {
			contentParts := make([]map[string]interface{}, 0)
			contentParts = append(contentParts, map[string]interface{}{
				"type": "text",
				"text": m.Content,
			})
			for _, img := range m.Images {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:image/jpeg;base64," + base64Encode(img),
					},
				})
			}
			msg["content"] = contentParts
		}
		ormsgs = append(ormsgs, msg)
	}

	go func() {
		defer close(out)

		if p.apiKey == "" {
			p.apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if p.apiKey == "" {
			out <- llm.StreamChunk{Error: "OpenRouter API key not configured", Done: true}
			return
		}

		model := p.model
		if model == "" {
			model = os.Getenv("OPENROUTER_MODEL")
			if model == "" {
				model = "openrouter/auto"
			}
		}

		payload := map[string]interface{}{
			"model":    model,
			"messages": ormsgs,
			"stream":   true,
		}

		// Добавляем tools если есть
		if len(tools) > 0 {
			orTools := make([]map[string]interface{}, 0, len(tools))
			for _, t := range tools {
				orTools = append(orTools, map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  t.Parameters,
					},
				})
			}
			payload["tools"] = orTools
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("marshal: %v", err), Done: true}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST",
			"https://openrouter.ai/api/v1/chat/completions",
			bytes.NewBuffer(jsonData))
		if err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("request: %v", err), Done: true}
			return
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HTTP-Referer", "https://lavender-messenger.com")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("http: %v", err), Done: true}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			out <- llm.StreamChunk{Error: fmt.Sprintf("OpenRouter %d: %s", resp.StatusCode, string(body)), Done: true}
			return
		}

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					out <- llm.StreamChunk{Done: true}
					return
				}
				out <- llm.StreamChunk{Error: fmt.Sprintf("read: %v", err), Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				out <- llm.StreamChunk{Done: true}
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
						ToolCalls []struct {
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta

				// Content chunk
				if delta.Content != "" {
					out <- llm.StreamChunk{Content: delta.Content}
				}

				// Tool call chunk
				for _, tc := range delta.ToolCalls {
					if tc.Function.Name != "" {
						out <- llm.StreamChunk{
							ToolCall: &llm.ToolCall{
								ID: tc.ID,
								Function: llm.ToolCallFunc{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							},
						}
					}
				}
			}
		}
	}()

	return out, nil
}

func (p *Provider) SetModel(model string) {
	p.model = model
}

func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var buf strings.Builder
	buf.Grow((len(data) + 2) / 3 * 4)
	for i := 0; i < len(data); i += 3 {
		b := []byte{0, 0, 0}
		b[0] = data[i]
		if i+1 < len(data) {
			b[1] = data[i+1]
		}
		if i+2 < len(data) {
			b[2] = data[i+2]
		}
		buf.WriteByte(alphabet[b[0]>>2])
		buf.WriteByte(alphabet[((b[0]&0x03)<<4)|(b[1]>>4)])
		if i+1 < len(data) {
			buf.WriteByte(alphabet[((b[1]&0x0f)<<2)|(b[2]>>6)])
		} else {
			buf.WriteByte('=')
		}
		if i+2 < len(data) {
			buf.WriteByte(alphabet[b[2]&0x3f])
		} else {
			buf.WriteByte('=')
		}
	}
	return buf.String()
}

// Используем log для отладки
var _ = log.Printf
