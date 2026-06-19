package main

// ai_agent_executor.go — Agent execution + tool calling loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	_ "database/sql" // will be used when ToolRegistry is implemented
)

// AgentExecutor handles agent execution with provider dispatch and tool calling loop
type AgentExecutor struct {
	db       *sql.DB
	registry *ProviderRegistry
	tools    *ToolRegistry
}

// NewAgentExecutor creates a new executor
func NewAgentExecutor(db *sql.DB, registry *ProviderRegistry, tools *ToolRegistry) *AgentExecutor {
	return &AgentExecutor{
		db:       db,
		registry: registry,
		tools:    tools,
	}
}

// Execute runs an agent with messages, streaming chunks via onChunk callback
func (e *AgentExecutor) Execute(ctx context.Context, agent *AgentV2, messages []AIMessageInput, settings *AIChatSettings, onChunk func(token string, finished bool) error) error {
	// 1. Get provider from registry
	provider, err := e.registry.Create(agent.ProviderType, agent.ProviderConfig, resolveAPIKey(agent, settings))
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}
	defer provider.Close()

	// 2. Get tool definitions if enabled
	var toolDefs []ToolDefInput
	if agent.ToolsEnabled && e.tools != nil {
		defs := e.tools.GetDefs(agent)
		for _, d := range defs {
			toolDefs = append(toolDefs, ToolDefInput{
				Type: d.Type,
				Function: ToolDefFuncInput{
					Name:        d.Function.Name,
					Description: d.Function.Description,
					Parameters:  d.Function.Parameters,
				},
			})
		}
	}

	// 3. Tool calling loop (max 10 iterations)
	maxIterations := 10
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Stream from provider
		ch, err := provider.StreamChat(ctx, messages, toolDefs)
		if err != nil {
			return err
		}

		// Collect response and tool calls
		var fullContent string
		var toolCalls []ToolCallRequestInput
		for chunk := range ch {
			if chunk.Error != nil {
				return chunk.Error
			}
			if chunk.Content != "" {
				fullContent += chunk.Content
				onChunk(chunk.Content, false)
			}
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, *chunk.ToolCall)
			}
			if chunk.Done {
				break
			}
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			onChunk("", true)
			return nil
		}

		// Execute tools and build results
		messages = append(messages, AIMessageInput{Role: "assistant", Content: fullContent})
		for _, tc := range toolCalls {
			result := e.executeTool(ctx, tc.Name, tc.Arguments)
			messages = append(messages, AIMessageInput{
				Role:    "tool",
				Content: fmt.Sprintf("Tool %s result: %s", tc.Name, result),
			})
		}
	}

	onChunk("", true)
	return nil
}

func (e *AgentExecutor) executeTool(ctx context.Context, name string, argsJSON string) string {
	var args map[string]any
	if argsJSON != "" {
		json.Unmarshal([]byte(argsJSON), &args)
	}
	result, err := e.tools.Execute(ctx, name, args)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return result
}

func resolveAPIKey(agent *AgentV2, settings *AIChatSettings) string {
	// Priority: settings user key → agent config key → env var
	if settings != nil && settings.UserAPIKey != "" {
		return settings.UserAPIKey
	}
	if agent.ProviderConfig != nil {
		if key, ok := agent.ProviderConfig["api_key"].(string); ok && key != "" {
			return key
		}
	}
	// Environment fallbacks
	switch agent.ProviderType {
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "mimo":
		return os.Getenv("MIMO_API_KEY")
	}
	return ""
}
