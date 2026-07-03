package main

// ai_provider_hermes_acp.go — Hermes Agent ACP provider
// Launches `hermes acp` as a child process
// Communicates via stdin/stdout in JSON-RPC 2.0 format
// Each user gets a persistent session (sync.Map, auto-cleanup)

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ======= JSON-RPC 2.0 Structures =======

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonrpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type sessionCreateResult struct {
	SessionID string `json:"session_id"`
}

type submitPromptParams struct {
	Prompt string `json:"prompt"`
}

// ======= HermesACPProvider =======

// HermesACPProvider — ACP provider for Hermes Agent
// Launches `hermes acp` as a child process
// Communicates via stdin/stdout in JSON-RPC 2.0 format
type HermesACPProvider struct {
	hermesPath string
	sessions   sync.Map // userID → *hermesSession
	mu         sync.RWMutex
}

type hermesSession struct {
	sessionID string
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	userID    string
	createdAt time.Time
	msgID     int // JSON-RPC request ID counter
}

// NewHermesACPProvider creates a new ACP provider
func NewHermesACPProvider(hermesPath string) (*HermesACPProvider, error) {
	if hermesPath == "" {
		hermesPath = findHermesBinary()
	}
	if hermesPath == "" {
		return nil, fmt.Errorf("hermes binary not found — set HERMES_ACP_PATH or HERMES_PATH env")
	}

	p := &HermesACPProvider{
		hermesPath: hermesPath,
	}

	log.Printf("[HermesACP] initialized path=%s", hermesPath)
	return p, nil
}

// StreamChat implements AgentProvider — main entry point
func (p *HermesACPProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages")
	}

	// Extract user ID from context (stored in first message or context)
	userID := extractUserID(ctx, messages)

	// Get or create persistent session
	sess, err := p.getOrCreateSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("hermes session: %w", err)
	}

	// Build prompt from messages
	prompt := buildPrompt(messages)

	ch := make(chan StreamChunk, 64)

	go func() {
		defer close(ch)

		sess.mu.Lock()
		defer sess.mu.Unlock()

		// Send session.submit_prompt
		sess.msgID++
		req := jsonrpcRequest{
			JSONRPC: "2.0",
			ID:      sess.msgID,
			Method:  "session.submit_prompt",
			Params:  submitPromptParams{Prompt: prompt},
		}

		reqBytes, err := json.Marshal(req)
		if err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("marshal request: %w", err), Done: true}
			return
		}

		reqBytes = append(reqBytes, '\n')
		if _, err := sess.stdin.Write(reqBytes); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("write to hermes stdin: %w", err), Done: true}
			return
		}

		// Read responses line by line
		for sess.stdout.Scan() {
			line := sess.stdout.Text()
			if line == "" {
				continue
			}

			// Try to parse as JSON-RPC notification
			var notif jsonrpcNotification
			if err := json.Unmarshal([]byte(line), &notif); err == nil && notif.Method != "" {
				switch notif.Method {
				case "session.prompt":
					// Extract token from params
					if params, ok := notif.Params.(map[string]any); ok {
						if token, ok := params["token"].(string); ok {
							ch <- StreamChunk{Content: token}
						}
					}
				case "session.done":
					ch <- StreamChunk{Done: true}
					return
				case "session.error":
					errMsg := "hermes error"
					if params, ok := notif.Params.(map[string]any); ok {
						if msg, ok := params["message"].(string); ok {
							errMsg = msg
						}
					}
					ch <- StreamChunk{Error: fmt.Errorf("hermes error: %s", errMsg), Done: true}
					return
				}
				continue
			}

			// Try to parse as JSON-RPC response (to our request)
			var resp jsonrpcResponse
			if err := json.Unmarshal([]byte(line), &resp); err == nil && resp.ID != 0 {
				if resp.Error != nil {
					ch <- StreamChunk{Error: fmt.Errorf("hermes rpc error %d: %s", resp.Error.Code, resp.Error.Message), Done: true}
					return
				}
				// Result for session.submit_prompt — just continue reading stream
				continue
			}

			// Fallback: treat as raw text content
			ch <- StreamChunk{Content: line + "\n"}
		}

		// Scanner ended — check for errors
		if err := sess.stdout.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("hermes stdout read: %w", err), Done: true}
			return
		}

		// Process exited normally
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

// Capabilities implements AgentProvider
func (p *HermesACPProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
		MaxTokens:         8192,
	}
}

// HealthCheck implements AgentProvider
func (p *HermesACPProvider) HealthCheck(ctx context.Context) error {
	if p.hermesPath == "" {
		return fmt.Errorf("hermes binary not found")
	}
	// Check binary exists and is executable
	info, err := os.Stat(p.hermesPath)
	if err != nil {
		return fmt.Errorf("hermes binary: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hermes binary is not executable: %s", p.hermesPath)
	}
	return nil
}

// Close implements AgentProvider — stop all sessions
func (p *HermesACPProvider) Close() error {
	p.sessions.Range(func(key, value any) bool {
		s := value.(*hermesSession)
		s.cancel()
		return true
	})
	p.sessions.Clear()
	return nil
}

// ======= Session Management =======

func (p *HermesACPProvider) getOrCreateSession(ctx context.Context, userID string) (*hermesSession, error) {
	// Check existing session
	if v, ok := p.sessions.Load(userID); ok {
		s := v.(*hermesSession)
		// Verify process is still alive
		if s.cmd.Process != nil && s.cmd.ProcessState == nil {
			return s, nil
		}
		// Process dead — cleanup and recreate
		p.sessions.Delete(userID)
	}

	// Create new session
	return p.createSession(ctx, userID)
}

func (p *HermesACPProvider) createSession(ctx context.Context, userID string) (*hermesSession, error) {
	sessCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(sessCtx, p.hermesPath, "acp")
	cmd.Stderr = nil // discard stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start hermes: %w", err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	sess := &hermesSession{
		stdin:     stdin,
		stdout:    scanner,
		cmd:       cmd,
		ctx:       sessCtx,
		cancel:    cancel,
		userID:    userID,
		createdAt: time.Now(),
		msgID:     0,
	}

	// Send session.create
	sess.msgID++
	createReq := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      sess.msgID,
		Method:  "session.create",
		Params:  struct{}{},
	}

	reqBytes, err := json.Marshal(createReq)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("marshal session.create: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	if _, err := stdin.Write(reqBytes); err != nil {
		cancel()
		return nil, fmt.Errorf("write session.create: %w", err)
	}

	// Wait for session.created notification or response
	if scanner.Scan() {
		line := scanner.Text()
		var notif jsonrpcNotification
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &notif); err == nil && notif.Method == "session.created" {
			if params, ok := notif.Params.(map[string]any); ok {
				if sid, ok := params["session_id"].(string); ok {
					sess.sessionID = sid
				}
			}
		} else if err := json.Unmarshal([]byte(line), &resp); err == nil && resp.Result != nil {
			var result sessionCreateResult
			json.Unmarshal(resp.Result, &result)
			sess.sessionID = result.SessionID
		}
	}

	p.sessions.Store(userID, sess)
	log.Printf("[HermesACP] session created user=%s session=%s", userID, sess.sessionID)

	// Start cleanup goroutine for this session
	go p.sessionWatchdog(userID, sess)

	return sess, nil
}

func (p *HermesACPProvider) sessionWatchdog(userID string, sess *hermesSession) {
	// Wait for process to exit
	err := sess.cmd.Wait()
	if err != nil && sess.ctx.Err() == nil {
		log.Printf("[HermesACP] session ended user=%s err=%v", userID, err)
	}
	// Cleanup
	p.sessions.Delete(userID)
	sess.cancel()
}

// CleanupInactiveSessions removes sessions inactive for > 30 minutes
func (p *HermesACPProvider) CleanupInactiveSessions() {
	now := time.Now()
	p.sessions.Range(func(key, value any) bool {
		s := value.(*hermesSession)
		if now.Sub(s.createdAt) > 30*time.Minute {
			log.Printf("[HermesACP] cleanup inactive session user=%s session=%s", key, s.sessionID)
			s.cancel()
			p.sessions.Delete(key)
		}
		return true
	})
}

// StopSession stops a specific user's session
func (p *HermesACPProvider) StopSession(userID string) {
	if v, ok := p.sessions.LoadAndDelete(userID); ok {
		s := v.(*hermesSession)
		log.Printf("[HermesACP] stopping session user=%s session=%s", userID, s.sessionID)
		s.cancel()
	}
}

// ActiveSessions returns the number of active sessions
func (p *HermesACPProvider) ActiveSessions() int {
	count := 0
	p.sessions.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// ======= Helpers =======

func extractUserID(ctx context.Context, messages []AIMessageInput) string {
	// Try to extract from first message content (pattern: "[User] userID: ...")
	if len(messages) > 0 {
		content := messages[0].Content
		if len(content) > 8 && content[:6] == "user:" {
			return content[6:]
		}
	}
	// Fallback: use a hash of the conversation
	return "default"
}

func buildPrompt(messages []AIMessageInput) string {
	var result string
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			result += "[System] "
		case "user":
			result += "[User] "
		case "assistant":
			result += "[Assistant] "
		case "tool":
			result += "[Tool] "
		}
		result += msg.Content + "\n\n"
	}
	if result == "" {
		result = " "
	}
	return result
}

// newHermesACPProvider is the factory function for registry
func newHermesACPProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	hermesPath, _ := config["hermes_path"].(string)
	if hermesPath == "" {
		hermesPath = os.Getenv("HERMES_ACP_PATH")
	}
	if hermesPath == "" {
		hermesPath = findHermesBinary()
	}
	return NewHermesACPProvider(hermesPath)
}
