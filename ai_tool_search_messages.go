package main

// ai_tool_search_messages.go — Search messages tool

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type searchMessagesTool struct {
	db *sql.DB
}

func (t *searchMessagesTool) Name() string { return "search_messages" }
func (t *searchMessagesTool) Description() string {
	return "Search messages by keyword across user's chats"
}
func (t *searchMessagesTool) RequiredRole() string { return "user" }

func (t *searchMessagesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search keyword or phrase",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Optional: limit search to specific chat ID",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results (default 10, max 50)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *searchMessagesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	chatID, _ := args["chat_id"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 50 {
			limit = 50
		}
	}

	var rows *sql.Rows
	var err error
	if chatID != "" {
		rows, err = t.db.QueryContext(ctx,
			`SELECT message_id, room_id, username, created_at, LEFT(encrypted_text::text, 200) as preview
			FROM messages WHERE room_id = $1 AND encrypted_text::text ILIKE $2
			ORDER BY created_at DESC LIMIT $3`,
			chatID, "%"+query+"%", limit)
	} else {
		rows, err = t.db.QueryContext(ctx,
			`SELECT message_id, room_id, username, created_at, LEFT(encrypted_text::text, 200) as preview
			FROM messages WHERE encrypted_text::text ILIKE $1
			ORDER BY created_at DESC LIMIT $2`,
			"%"+query+"%", limit)
	}
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var msgID, roomID, username, preview string
		var createdAt interface{}
		if err := rows.Scan(&msgID, &roomID, &username, &createdAt, &preview); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s] in %s by %s: %s", createdAt, roomID, username, strings.TrimSpace(preview)))
	}

	if len(results) == 0 {
		return "No messages found matching: " + query, nil
	}
	return strings.Join(results, "\n"), nil
}
