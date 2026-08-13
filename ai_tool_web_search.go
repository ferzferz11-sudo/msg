package main

// ai_tool_web_search.go — Web search tool (DuckDuckGo)

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type webSearchTool struct{}

func (t *webSearchTool) Name() string         { return "web_search" }
func (t *webSearchTool) Description() string  { return "Search the web for information" }
func (t *webSearchTool) RequiredRole() string { return "user" }

func (t *webSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
		},
		"required": []string{"query"},
	}
}

func (t *webSearchTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) + "&format=json&no_html=1"
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var result struct {
		Abstract    string `json:"Abstract"`
		AbstractURL string `json:"AbstractURL"`
		Results     []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text string `json:"Text"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	var output []string
	if result.Abstract != "" {
		output = append(output, result.Abstract)
	}
	for i, r := range result.Results {
		if i >= 5 {
			break
		}
		if r.Text != "" {
			output = append(output, "- "+r.Text)
		}
	}
	for i, r := range result.RelatedTopics {
		if i >= 3 {
			break
		}
		if r.Text != "" {
			output = append(output, "- "+r.Text)
		}
	}

	if len(output) == 0 {
		return "No results found for: " + query, nil
	}
	return "Search results for: " + query + "\n\n" + joinStrings(output, "\n"), nil
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
