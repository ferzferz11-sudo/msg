package main

// ai_provider_webhook.go — HTTP webhook provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type webhookProvider struct {
	url       string
	method    string
	headers   map[string]string
	timeout   time.Duration
	streaming bool
	client    *http.Client
}

func newWebhookProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	url, _ := config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}
	method, _ := config["method"].(string)
	if method == "" {
		method = "POST"
	}
	timeoutSec := 30
	if t, ok := config["timeout_seconds"].(float64); ok {
		timeoutSec = int(t)
	}
	streaming, _ := config["streaming"].(bool)

	headers := make(map[string]string)
	if h, ok := config["headers"].(map[string]any); ok {
		for k, v := range h {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}

	return &webhookProvider{
		url:       url,
		method:    method,
		headers:   headers,
		timeout:   time.Duration(timeoutSec) * time.Second,
		streaming: streaming,
		client:    &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}, nil
}

func (p *webhookProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	payload := map[string]any{
		"messages": convertMessages(messages),
	}
	if len(tools) > 0 {
		payload["tools"] = convertToolDefs(tools)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, p.method, p.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}

	ch := make(chan StreamChunk, 64)

	if p.streaming {
		go func() {
			defer close(ch)
			defer resp.Body.Close()
			readSSEStream(ctx, resp.Body, ch)
		}()
	} else {
		go func() {
			defer close(ch)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}
			var result struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(body, &result); err == nil && result.Content != "" {
				ch <- StreamChunk{Content: result.Content}
			} else {
				ch <- StreamChunk{Content: string(body)}
			}
			ch <- StreamChunk{Done: true}
		}()
	}

	return ch, nil
}

func (p *webhookProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: p.streaming,
		MaxTokens:         4096,
	}
}

func (p *webhookProvider) HealthCheck(ctx context.Context) error {
	if p.url == "" {
		return fmt.Errorf("webhook URL not configured")
	}
	return nil
}

func (p *webhookProvider) Close() error { return nil }
