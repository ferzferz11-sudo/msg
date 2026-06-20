package main

// ai_tool_registry.go — Tool registry for AI agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

// ToolRegistry holds all registered tools
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates a registry and registers built-in tools
func NewToolRegistry(db *sql.DB) *ToolRegistry {
	r := &ToolRegistry{
		tools: make(map[string]Tool),
	}
	// Shared cache for tool results (1min TTL, max 500 entries)
	cache := newToolCache(time.Minute, 500)
	// Register built-in tools with caching for DB-backed tools
	r.Register(newCachedSearchMessagesTool(db, cache))
	r.Register(newCachedSearchUsersTool(db, cache))
	r.Register(&webSearchTool{})
	r.Register(&webFetchTool{})
	r.Register(newCachedGetChatInfoTool(db, cache))
	r.Register(&queryDatabaseTool{db: db})
	return r
}

// Register adds a tool
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// GetAll returns all registered tools
func (r *ToolRegistry) GetAll() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// GetDefs returns tool definitions filtered by agent's whitelist
func (r *ToolRegistry) GetDefs(agent *AgentV2) []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []ToolDef
	for _, t := range r.tools {
		// Check whitelist
		if agent.ToolWhitelist != nil && len(agent.ToolWhitelist) > 0 {
			allowed := false
			for _, w := range agent.ToolWhitelist {
				if w == t.Name() {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		defs = append(defs, ToolDef{
			Type: "function",
			Function: ToolDefFunc{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", &toolNotFoundError{name: name}
	}
	return tool.Execute(ctx, args)
}

// ListInfo returns tool info for the ListAITools RPC
func (r *ToolRegistry) ListInfo() []toolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var infos []toolInfo
	for _, t := range r.tools {
		paramsJSON, _ := json.Marshal(t.Parameters())
		infos = append(infos, toolInfo{
			Name:             t.Name(),
			Description:      t.Description(),
			ParametersSchema: string(paramsJSON),
			RequiredRole:     t.RequiredRole(),
		})
	}
	return infos
}

type toolInfo struct {
	Name             string
	Description      string
	ParametersSchema string
	RequiredRole     string
}

type toolNotFoundError struct {
	name string
}

func (e *toolNotFoundError) Error() string {
	return "tool not found: " + e.name
}
