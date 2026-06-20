package main

// ai_tool_query_db.go — Database query tool for AI agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type queryDatabaseTool struct {
	db *sql.DB
}

func (t *queryDatabaseTool) Name() string         { return "query_database" }
func (t *queryDatabaseTool) RequiredRole() string { return "admin" }

func (t *queryDatabaseTool) Description() string {
	return "Execute a read-only SQL query against the database. Returns results as JSON. Only SELECT queries are allowed."
}

func (t *queryDatabaseTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The SQL SELECT query to execute",
			},
			"max_rows": map[string]any{
				"type":        "integer",
				"description": "Maximum number of rows to return (default 100, max 1000)",
				"default":     100,
			},
		},
		"required": []string{"query"},
	}
}

func (t *queryDatabaseTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Security: only allow SELECT queries
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(trimmed, "SELECT") {
		return "", fmt.Errorf("only SELECT queries are allowed")
	}

	// Block dangerous patterns — expanded blocklist
	dangerous := []string{
		"DROP", "DELETE", "INSERT", "UPDATE", "ALTER", "TRUNCATE", "CREATE",
		"EXEC", "EXECUTE", "INTO", "GRANT", "REVOKE",
		"PG_READ_FILE", "PG_WRITE_FILE", "DBLINK", "COPY",
		"pg_", "information_schema",
	}
	for _, d := range dangerous {
		if strings.Contains(trimmed, d) {
			return "", fmt.Errorf("query contains forbidden keyword: %s", d)
		}
	}

	// Block access to sensitive tables
	blockedTables := []string{
		"users", "user_tokens", "user_devices", "user_chat_metadata",
		"password_reset_tokens", "ai_usage_stats",
	}
	for _, t := range blockedTables {
		if strings.Contains(trimmed, strings.ToUpper(t)) {
			return "", fmt.Errorf("access to table '%s' is not allowed", t)
		}
	}

	maxRows := 100
	if m, ok := args["max_rows"].(float64); ok && m > 0 {
		maxRows = int(m)
		if maxRows > 1000 {
			maxRows = 1000
		}
	}

	// Add LIMIT if not present
	if !strings.Contains(trimmed, "LIMIT") {
		query = strings.TrimRight(query, ";") + fmt.Sprintf(" LIMIT %d", maxRows)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Execute in a read-only transaction for defense-in-depth
	tx, err := t.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", fmt.Errorf("failed to start read-only transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("row iteration error: %w", err)
	}

	if results == nil {
		results = []map[string]any{}
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return string(data), nil
}
