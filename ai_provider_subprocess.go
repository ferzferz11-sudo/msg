package main

// ai_provider_subprocess.go — Subprocess provider (runs external process)

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type subprocessProvider struct {
	command   string
	args      []string
	env       map[string]string
	timeout   time.Duration
	streaming bool
}

func newSubprocessProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	command, _ := config["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("subprocess command is required")
	}
	var args []string
	if a, ok := config["args"].([]any); ok {
		for _, v := range a {
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	env := make(map[string]string)
	if e, ok := config["env"].(map[string]any); ok {
		for k, v := range e {
			env[k] = fmt.Sprintf("%v", v)
		}
	}
	timeoutSec := 60
	if t, ok := config["timeout_seconds"].(float64); ok {
		timeoutSec = int(t)
	}
	streaming, _ := config["streaming"].(bool)

	return &subprocessProvider{
		command:   command,
		args:      args,
		env:       env,
		timeout:   time.Duration(timeoutSec) * time.Second,
		streaming: streaming,
	}, nil
}

func (p *subprocessProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages")
	}

	// Build input payload
	payload := map[string]any{
		"messages": convertMessages(messages),
	}
	inputData, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	cmd := exec.CommandContext(ctx, p.command, p.args...)
	cmd.Stdin = bytes.NewReader(inputData)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range p.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer cancel()

		if p.streaming {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				ch <- StreamChunk{Content: scanner.Text() + "\n"}
			}
		} else {
			var result struct {
				Content string `json:"content"`
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			if err := json.Unmarshal(buf.Bytes(), &result); err == nil && result.Content != "" {
				ch <- StreamChunk{Content: result.Content}
			} else {
				ch <- StreamChunk{Content: buf.String()}
			}
		}

		err := cmd.Wait()
		if err != nil {
			ch <- StreamChunk{Error: err}
		}
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

func (p *subprocessProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: p.streaming,
		MaxTokens:         4096,
	}
}

func (p *subprocessProvider) HealthCheck(ctx context.Context) error {
	if p.command == "" {
		return fmt.Errorf("subprocess command not configured")
	}
	return nil
}

func (p *subprocessProvider) Close() error { return nil }
