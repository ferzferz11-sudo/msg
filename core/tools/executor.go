package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"LavenderMessenger/core/llm"
	"LavenderMessenger/core/pipeline"
)

// DefaultToolExecutor — реализация ToolExecutor с набором инструментов
type DefaultToolExecutor struct {
	db *sql.DB
}

func NewDefaultToolExecutor(db *sql.DB) pipeline.ToolExecutor {
	return &DefaultToolExecutor{db: db}
}

func (e *DefaultToolExecutor) GetToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "search_messages",
			Description: "Search messages in the database by keyword. Use when user asks about previous messages or conversations.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search keyword or phrase",
					},
					"chat_id": map[string]any{
						"type":        "string",
						"description": "Optional chat ID to limit search scope",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Max results (default 10)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "search_users",
			Description: "Search users by name, username, or phone number.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query (name, username, or phone)",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Max results (default 5)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "web_search",
			Description: "Search the web for current information. Use for facts, news, or any information not in the database.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query",
					},
					"max_results": map[string]any{
						"type":        "integer",
						"description": "Max results (default 5)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "get_chat_info",
			Description: "Get information about a specific chat (name, type, member count).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"chat_id": map[string]any{
						"type":        "string",
						"description": "Chat ID to look up",
					},
				},
				"required": []string{"chat_id"},
			},
		},
	}
}

func (e *DefaultToolExecutor) Execute(ctx context.Context, call llm.ToolCall) string {
	log.Printf("[ToolExecutor] executing tool=%s id=%s", call.Function.Name, call.Function.Name)

	var params map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &params); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	switch call.Function.Name {
	case "search_messages":
		return e.searchMessages(ctx, params)
	case "search_users":
		return e.searchUsers(ctx, params)
	case "web_search":
		return e.webSearch(ctx, params)
	case "get_chat_info":
		return e.getChatInfo(ctx, params)
	default:
		return fmt.Sprintf("Unknown tool: %s", call.Function.Name)
	}
}

func (e *DefaultToolExecutor) searchMessages(ctx context.Context, params map[string]any) string {
	query, _ := params["query"].(string)
	if query == "" {
		return "Error: query is required"
	}

	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	var chatID string
	if c, ok := params["chat_id"].(string); ok {
		chatID = c
	}

	if e.db == nil {
		return "Error: database not available"
	}

	var rows *sql.Rows
	var err error

	if chatID != "" {
		rows, err = e.db.QueryContext(ctx,
			`SELECT m.id, m.chat_id, m.sender_id, m.content, m.created_at 
			 FROM messages m 
			 WHERE m.chat_id = $1 AND m.content ILIKE $2 
			 ORDER BY m.created_at DESC LIMIT $3`,
			chatID, "%"+query+"%", limit,
		)
	} else {
		rows, err = e.db.QueryContext(ctx,
			`SELECT m.id, m.chat_id, m.sender_id, m.content, m.created_at 
			 FROM messages m 
			 WHERE m.content ILIKE $1 
			 ORDER BY m.created_at DESC LIMIT $2`,
			"%"+query+"%", limit,
		)
	}

	if err != nil {
		return fmt.Sprintf("Search error: %v", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var id, chatID, senderID, content string
		var createdAt time.Time
		if err := rows.Scan(&id, &chatID, &senderID, &content, &createdAt); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s] chat=%s sender=%s: %s",
			createdAt.Format("2006-01-02 15:04"), chatID, senderID, truncate(content, 100)))
	}

	if len(results) == 0 {
		return "No messages found matching the query."
	}

	return fmt.Sprintf("Found %d messages:\n%s", len(results), strings.Join(results, "\n"))
}

func (e *DefaultToolExecutor) searchUsers(ctx context.Context, params map[string]any) string {
	query, _ := params["query"].(string)
	if query == "" {
		return "Error: query is required"
	}

	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	if e.db == nil {
		return "Error: database not available"
	}

	rows, err := e.db.QueryContext(ctx,
		`SELECT id, username, display_name, phone 
		 FROM users 
		 WHERE username ILIKE $1 OR display_name ILIKE $2 OR phone ILIKE $3
		 LIMIT $4`,
		"%"+query+"%", "%"+query+"%", "%"+query+"%", limit,
	)
	if err != nil {
		return fmt.Sprintf("Search error: %v", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var id, username, displayName, phone string
		if err := rows.Scan(&id, &username, &displayName, &phone); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("id=%s username=%s name=%s phone=%s",
			id, username, displayName, phone))
	}

	if len(results) == 0 {
		return "No users found matching the query."
	}

	return fmt.Sprintf("Found %d users:\n%s", len(results), strings.Join(results, "\n"))
}

func (e *DefaultToolExecutor) webSearch(ctx context.Context, params map[string]any) string {
	query, _ := params["query"].(string)
	if query == "" {
		return "Error: query is required"
	}

	maxResults := 5
	if m, ok := params["max_results"].(float64); ok {
		maxResults = int(m)
	}

	// DuckDuckGo Instant Answer API (free, no auth)
	ddgURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", ddgURL, nil)
	if err != nil {
		return fmt.Sprintf("Web search error: %v", err)
	}
	req.Header.Set("User-Agent", "LavenderBot/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Web search error: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Results []struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Web search parse error: %v", err)
	}

	var parts []string
	if result.AbstractText != "" {
		parts = append(parts, fmt.Sprintf("Summary: %s\nSource: %s", result.AbstractText, result.AbstractURL))
	}

	count := 0
	for _, r := range result.Results {
		if count >= maxResults {
			break
		}
		if r.Text != "" {
			parts = append(parts, fmt.Sprintf("- %s\n  %s", r.Text, r.URL))
			count++
		}
	}

	// RelatedTopics тоже могут быть полезны
	for _, r := range result.RelatedTopics {
		if count >= maxResults {
			break
		}
		if r.Text != "" {
			parts = append(parts, fmt.Sprintf("- %s\n  %s", r.Text, r.URL))
			count++
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("No web results found for: %s", query)
	}

	return strings.Join(parts, "\n\n")
}

func (e *DefaultToolExecutor) getChatInfo(ctx context.Context, params map[string]any) string {
	chatID, _ := params["chat_id"].(string)
	if chatID == "" {
		return "Error: chat_id is required"
	}

	if e.db == nil {
		return "Error: database not available"
	}

	var name, chatType string
	var memberCount int

	err := e.db.QueryRowContext(ctx,
		`SELECT c.name, c.type, COUNT(cm.user_id) 
		 FROM chats c 
		 LEFT JOIN chat_members cm ON cm.chat_id = c.id 
		 WHERE c.id = $1 
		 GROUP BY c.name, c.type`,
		chatID,
	).Scan(&name, &chatType, &memberCount)

	if err != nil {
		return fmt.Sprintf("Chat not found or error: %v", err)
	}

	return fmt.Sprintf("Chat: %s\nType: %s\nMembers: %d", name, chatType, memberCount)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
