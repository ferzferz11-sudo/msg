package main

// ai_tool_get_chat_info.go — Get chat info tool

import (
	"context"
	"database/sql"
	"fmt"
)

type getChatInfoTool struct {
	db *sql.DB
}

func (t *getChatInfoTool) Name() string        { return "get_chat_info" }
func (t *getChatInfoTool) Description() string  { return "Get metadata about a chat (name, type, member count)" }
func (t *getChatInfoTool) RequiredRole() string { return "user" }

func (t *getChatInfoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Chat ID to get info about",
			},
		},
		"required": []string{"chat_id"},
	}
}

func (t *getChatInfoTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatID, _ := args["chat_id"].(string)
	if chatID == "" {
		return "", fmt.Errorf("chat_id is required")
	}

	var name, chatType string
	err := t.db.QueryRowContext(ctx,
		`SELECT name, type FROM chats WHERE id = $1`, chatID).Scan(&name, &chatType)
	if err != nil {
		return "", fmt.Errorf("chat not found: %v", err)
	}

	var memberCount int
	t.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chats WHERE id = $1 AND participants IS NOT NULL`, chatID).Scan(&memberCount)

	return fmt.Sprintf("Chat: %s\nType: %s\nMembers: %d", name, chatType, memberCount), nil
}
