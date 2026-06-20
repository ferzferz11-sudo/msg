package main

// ai_provider_mcp.go — MCP (Model Context Protocol) provider
// Supports stdio transport with JSON-RPC 2.0

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type mcpProvider struct {
	command   string
	args      []string
	transport string // "stdio" or "sse"
	timeout   time.Duration
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	mu        sync.Mutex
	nextID    int
	pending   map[int]chan json.RawMessage
}

func newMCPProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	command, _ := config["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("MCP server command is required")
	}
	var args []string
	if a, ok := config["args"].([]any); ok {
		for _, v := range a {
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	transport, _ := config["transport"].(string)
	if transport == "" {
		transport = "stdio"
	}
	timeoutSec := 10
	if t, ok := config["timeout_seconds"].(float64); ok {
		timeoutSec = int(t)
	}

	return &mcpProvider{
		command:   command,
		args:      args,
		transport: transport,
		timeout:   time.Duration(timeoutSec) * time.Second,
		pending:   make(map[int]chan json.RawMessage),
	}, nil
}

func (p *mcpProvider) connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return nil // already connected
	}

	p.cmd = exec.CommandContext(ctx, p.command, p.args...)
	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.stdin = stdin
	p.stdout = bufio.NewReader(stdout)

	if err := p.cmd.Start(); err != nil {
		return err
	}

	// Start response reader goroutine
	go p.readResponses()

	// Initialize MCP session
	_, err = p.sendRequest(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "lavender-server",
			"version": "1.3.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("MCP initialize failed: %w", err)
	}

	return nil
}

func (p *mcpProvider) readResponses() {
	for {
		line, err := p.stdout.ReadString('\n')
		if err != nil {
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		p.mu.Lock()
		ch, ok := p.pending[msg.ID]
		if ok {
			delete(p.pending, msg.ID)
		}
		p.mu.Unlock()
		if ok {
			ch <- msg.Result
			close(ch)
		}
	}
}

func (p *mcpProvider) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	ch := make(chan json.RawMessage, 1)
	p.pending[id] = ch
	p.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	p.mu.Lock()
	_, err := p.stdin.Write(data)
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(p.timeout):
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("MCP request timeout")
	}
}

func (p *mcpProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	if err := p.connect(ctx); err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk, 64)

	go func() {
		defer close(ch)

		// Call the LLM with the MCP context
		result, err := p.sendRequest(ctx, "tools/call", map[string]any{
			"name":      "chat",
			"arguments": map[string]any{"messages": convertMessages(messages)},
		})
		if err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}

		var response struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(result, &response); err == nil {
			for _, c := range response.Content {
				if c.Type == "text" && c.Text != "" {
					ch <- StreamChunk{Content: c.Text}
				}
			}
		}

		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

func (p *mcpProvider) listTools(ctx context.Context) ([]ToolDefInput, error) {
	result, err := p.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	var defs []ToolDefInput
	for _, t := range resp.Tools {
		defs = append(defs, ToolDefInput{
			Type: "function",
			Function: ToolDefFuncInput{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return defs, nil
}

func (p *mcpProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     true,
		SupportsStreaming: false,
		MaxTokens:         4096,
	}
}

func (p *mcpProvider) HealthCheck(ctx context.Context) error {
	if p.command == "" {
		return fmt.Errorf("MCP server command not configured")
	}
	return nil
}

func (p *mcpProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		p.stdin.Close()
		p.cmd.Process.Kill()
	}
	return nil
}
