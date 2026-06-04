package hermes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/core/llm"
)

// Provider — реализация LLMProvider для локального Hermes (stdin/stdout JSON-RPC)
// Запускает hermes как дочерний процесс и общается через JSON построчно
type Provider struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	mu        sync.Mutex
	modelID   string
	hermesPath string
	sessionID string
}

// hermesRequest — запрос к Hermes процессу
type hermesRequest struct {
	Type      string        `json:"type"` // "chat"
	Messages  []llm.Message `json:"messages"`
	Tools     []llm.ToolDef `json:"tools,omitempty"`
	Stream    bool          `json:"stream"`
	SessionID string        `json:"session_id,omitempty"`
}

// hermesResponse — ответ от Hermes процесса
type hermesResponse struct {
	Type     string `json:"type"` // "chunk" | "tool_call" | "done" | "error"
	Content  string `json:"content,omitempty"`
	ToolCall *struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_call,omitempty"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

// NewProvider создаёт Hermes provider
// hermesPath — путь к бинарнику hermes (например, "/usr/local/bin/hermes")
func NewProvider(hermesPath string) (*Provider, error) {
	if hermesPath == "" {
		hermesPath = findHermesBinary()
	}
	if hermesPath == "" {
		return nil, fmt.Errorf("hermes binary not found — set HERMES_PATH env or pass path explicitly")
	}

	p := &Provider{
		modelID:    "hermes-local",
		hermesPath: hermesPath,
	}

	if err := p.start(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Provider) start() error {
	// Запускаем hermes в JSON-RPC режиме
	// Ожидаем, что hermes поддерживает: hermes --json-rpc --stream
	cmd := exec.Command(p.hermesPath, "--json-rpc", "--stream")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// stderr — в лог сервера
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start hermes: %w", err)
	}

	p.cmd = cmd
	p.stdin = stdin
	p.stdout = bufio.NewScanner(stdout)

	// Увеличиваем буфер сканера для длинных строк
	p.stdout.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	log.Printf("[HermesProvider] started pid=%d path=%s", cmd.Process.Pid, p.hermesPath)
	return nil
}

func (p *Provider) ModelID() string {
	return p.modelID
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		return p.cmd.Wait()
	}
	return nil
}

func (p *Provider) StreamChat(
	ctx context.Context,
	messages []llm.Message,
	tools []llm.ToolDef,
) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk, 64)

	req := hermesRequest{
		Type:      "chat",
		Messages:  messages,
		Tools:     tools,
		Stream:    true,
		SessionID: p.sessionID,
	}

	p.mu.Lock()
	data, err := json.Marshal(req)
	if err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if _, err := p.stdin.Write(data); err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("write: %w", err)
	}
	p.mu.Unlock()

	go func() {
		defer close(out)

		for p.stdout.Scan() {
			select {
			case <-ctx.Done():
				out <- llm.StreamChunk{Error: "context cancelled", Done: true}
				return
			default:
			}

			line := p.stdout.Bytes()
			if len(line) == 0 {
				continue
			}

			var resp hermesResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				// Пропускаем не-JSON строки (логи, debug и т.д.)
				continue
			}

			switch resp.Type {
			case "chunk":
				if resp.Content != "" {
					out <- llm.StreamChunk{Content: resp.Content}
				}
			case "tool_call":
				if resp.ToolCall != nil {
					out <- llm.StreamChunk{
						ToolCall: &llm.ToolCall{
							ID: resp.ToolCall.ID,
							Function: llm.ToolCallFunc{
								Name:      resp.ToolCall.Function.Name,
								Arguments: resp.ToolCall.Function.Arguments,
							},
						},
					}
				}
			case "done":
				out <- llm.StreamChunk{Done: true}
				return
			case "error":
				out <- llm.StreamChunk{Error: resp.Error, Done: true}
				return
			}
		}

		// Scanner завершился — процесс умер
		if err := p.stdout.Err(); err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("stdout: %v", err), Done: true}
		} else {
			out <- llm.StreamChunk{Error: "hermes process exited", Done: true}
		}
	}()

	return out, nil
}

// SetSessionID — устанавливает ID сессии для контекста
func (p *Provider) SetSessionID(id string) {
	p.sessionID = id
}

// Restart — перезапускает hermes процесс
func (p *Provider) Restart() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}

	return p.start()
}

// findHermesBinary — ищет hermes в стандартных местах
func findHermesBinary() string {
	paths := []string{
		os.Getenv("HERMES_PATH"),
		"/usr/local/bin/hermes",
		"/usr/bin/hermes",
		"/root/hermes/bin/hermes",
		"/root/.hermes/bin/hermes",
		"/root/go/bin/hermes",
	}

	for _, p := range paths {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	// Пробуем which
	cmd := exec.Command("which", "hermes")
	out, err := cmd.Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return p
		}
	}

	return ""
}

// Используем time для heartbeat
var _ = time.Now
