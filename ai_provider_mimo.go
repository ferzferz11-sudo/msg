package main

// ai_provider_mimo.go — MiMo LLM provider (HTTP API)

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type miMoProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

func newMiMoProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	model, _ := config["model"].(string)
	if model == "" {
		model = os.Getenv("MIMO_MODEL")
	}
	if model == "" {
		model = "mimo-auto"
	}
	baseURL, _ := config["base_url"].(string)
	if baseURL == "" {
		baseURL = os.Getenv("MIMO_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.mimo.ai/v1"
	}
	return &miMoProvider{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: baseURL,
	}, nil
}

func (p *miMoProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	apiKey := p.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("MIMO_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("MiMo API key not configured")
	}

	payload := map[string]any{
		"model":    p.model,
		"messages": convertMessages(messages),
		"stream":   true,
	}
	if len(tools) > 0 {
		payload["tools"] = convertToolDefs(tools)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MiMo request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("MiMo returned %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		readSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

func (p *miMoProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
		MaxTokens:         128000,
	}
}

func (p *miMoProvider) HealthCheck(ctx context.Context) error {
	if p.apiKey == "" && os.Getenv("MIMO_API_KEY") == "" {
		return fmt.Errorf("MiMo API key not configured")
	}
	return nil
}

func (p *miMoProvider) Close() error { return nil }

// readSSEStream is shared by OpenRouter and MiMo providers
func readSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	reader := bufio.NewReader(body)
	for {
		if ctx.Err() != nil {
			ch <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				ch <- StreamChunk{Done: true}
				return
			}
			ch <- StreamChunk{Error: err, Done: true}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- StreamChunk{Done: true}
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						ID   string `json:"id"`
						Type string `json:"type"`
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
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			ch <- StreamChunk{Content: delta.Content}
		}
		for _, tc := range delta.ToolCalls {
			ch <- StreamChunk{
				ToolCall: &ToolCallRequestInput{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
}
