package hermes

import (
	"log"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/core/llm"
)

// Provider — реализация LLMProvider для локального Hermes Agent
// Запускает `hermes chat -q` как дочерний процесс, стримит ответ построчно
type Provider struct {
	mu         sync.Mutex
	modelID    string
	hermesPath string
	sessionID  string
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

	log.Printf("[HermesProvider] initialized path=%s", hermesPath)
	return p, nil
}

func (p *Provider) ModelID() string {
	return p.modelID
}

func (p *Provider) Close() error {
	return nil // stateless — каждый запуск отдельный процесс
}

// SetSessionID — устанавливает ID сессии для контекста (передаётся через --resume)
func (p *Provider) SetSessionID(id string) {
	p.sessionID = id
}

func (p *Provider) StreamChat(
	ctx context.Context,
	messages []llm.Message,
	tools []llm.ToolDef,
) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk, 64)

	// Собираем весь контекст в один промпт
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString("[System] ")
		case "user":
			sb.WriteString("[User] ")
		case "assistant":
			sb.WriteString("[Assistant] ")
		case "tool":
			sb.WriteString("[Tool] ")
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}
	query := strings.TrimSpace(sb.String())
	if query == "" {
		query = " "
	}

	go func() {
		defer close(out)

		// Собираем команду
		args := []string{"chat", "-q", query, "--quiet"}

		// Если есть сессия — возобновляем
		if p.sessionID != "" {
			args = append(args, "--resume", p.sessionID)
		}

		// Картинки — передаём через --image (только первая для простоты)
		for _, msg := range messages {
			for _, img := range msg.Images {
				// Сохраняем во временный файл
				tmpFile := fmt.Sprintf("/tmp/hermes_img_%d.jpg", time.Now().UnixNano())
				if err := os.WriteFile(tmpFile, img, 0644); err == nil {
					args = append(args, "--image", tmpFile)
				}
				break // только первая картинка
			}
			break
		}

		cmd := exec.CommandContext(ctx, p.hermesPath, args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("stdout pipe: %v", err), Done: true}
			return
		}

		if err := cmd.Start(); err != nil {
			out <- llm.StreamChunk{Error: fmt.Sprintf("start hermes: %v", err), Done: true}
			return
		}

		// Читаем stdout построчно и стримим
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		sessionID := ""
		fullResponse := strings.Builder{}

		for scanner.Scan() {
			line := scanner.Text()

			// Пропускаем служебные строки
			if strings.HasPrefix(line, "session_id:") {
				sessionID = strings.TrimSpace(strings.TrimPrefix(line, "session_id:"))
				continue
			}

			// Пропускаем пустые строки в начале
			if fullResponse.Len() == 0 && strings.TrimSpace(line) == "" {
				continue
			}

			fullResponse.WriteString(line)
			fullResponse.WriteString("\n")

			// Стримим по строкам (эмуляция стриминга)
			out <- llm.StreamChunk{Content: line + "\n"}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() == context.Canceled {
				out <- llm.StreamChunk{Error: "context cancelled", Done: true}
			} else {
				out <- llm.StreamChunk{Error: fmt.Sprintf("hermes exit: %v", err), Done: true}
			}
			return
		}

		// Сохраняем session ID для следующего запроса
		if sessionID != "" {
			p.mu.Lock()
			p.sessionID = sessionID
			p.mu.Unlock()
		}

		// Если ничего не прочитали — отправляем пустой done
		if fullResponse.Len() == 0 {
			log.Printf("[HermesProvider] empty response for query: %q", query[:min(len(query), 80)])
		}

		out <- llm.StreamChunk{Done: true}
	}()

	return out, nil
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
