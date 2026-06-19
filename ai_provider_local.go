package main

// ai_provider_local.go — Local Hermes LLM provider (subprocess)

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type localProvider struct {
	binaryPath string
}

func newLocalProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	binary := findHermesBinary()
	if binary == "" {
		return nil, fmt.Errorf("hermes binary not found")
	}
	return &localProvider{binaryPath: binary}, nil
}

func findHermesBinary() string {
	if p := os.Getenv("HERMES_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, p := range []string{"/usr/local/bin/hermes", "/usr/bin/hermes"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (p *localProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages")
	}

	lastMsg := messages[len(messages)-1].Content
	args := []string{"chat", "-q", lastMsg, "--quiet"}

	ch := make(chan StreamChunk, 64)
	cmd := exec.CommandContext(ctx, p.binaryPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(ch)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		close(ch)
		return nil, err
	}

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "session_id:") {
				continue
			}
			ch <- StreamChunk{Content: line + "\n"}
		}
		ch <- StreamChunk{Done: true}
		cmd.Wait()
	}()

	return ch, nil
}

func (p *localProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
		MaxTokens:         32000,
	}
}

func (p *localProvider) HealthCheck(ctx context.Context) error {
	if p.binaryPath == "" {
		return fmt.Errorf("hermes binary not found")
	}
	return nil
}

func (p *localProvider) Close() error { return nil }
