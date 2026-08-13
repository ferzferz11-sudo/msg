package main

// ai_tool_search_users.go — Search users tool

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type searchUsersTool struct {
	db *sql.DB
}

func (t *searchUsersTool) Name() string         { return "search_users" }
func (t *searchUsersTool) Description() string  { return "Search users by name, username, or phone" }
func (t *searchUsersTool) RequiredRole() string { return "user" }

func (t *searchUsersTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search term (name, username, or phone)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results (default 10)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *searchUsersTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	rows, err := t.db.QueryContext(ctx,
		`SELECT id, username, COALESCE(display_name,''), COALESCE(email,'')
		FROM users WHERE username ILIKE $1 OR display_name ILIKE $1 OR email ILIKE $1
		LIMIT $2`,
		"%"+query+"%", limit)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var id, username, displayName, email string
		if err := rows.Scan(&id, &username, &displayName, &email); err != nil {
			continue
		}
		entry := username
		if displayName != "" {
			entry = displayName + " (@" + username + ")"
		}
		if email != "" {
			entry += " <" + email + ">"
		}
		results = append(results, entry)
	}

	if len(results) == 0 {
		return "No users found matching: " + query, nil
	}
	return strings.Join(results, "\n"), nil
}
