package main

// ai_tool_web_fetch.go — Fetch URL content tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type webFetchTool struct{}

func (t *webFetchTool) Name() string         { return "web_fetch" }
func (t *webFetchTool) Description() string  { return "Fetch content from a URL" }
func (t *webFetchTool) RequiredRole() string { return "user" }

func (t *webFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to fetch",
			},
			"max_chars": map[string]any{
				"type":        "integer",
				"description": "Max characters to return (default 5000)",
			},
		},
		"required": []string{"url"},
	}
}

func (t *webFetchTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	u, _ := args["url"].(string)
	if u == "" {
		return "", fmt.Errorf("url is required")
	}
	maxChars := 5000
	if m, ok := args["max_chars"].(float64); ok {
		maxChars = int(m)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LavenderMessenger/1.3")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxChars*2)))
	if err != nil {
		return "", err
	}

	content := string(body)
	if len(content) > maxChars {
		content = content[:maxChars] + "\n...[truncated]"
	}

	return fmt.Sprintf("URL: %s\nStatus: %d\nContent:\n%s", u, resp.StatusCode, content), nil
}
