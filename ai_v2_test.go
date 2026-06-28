package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ======= Mock AgentProvider =======

type mockProvider struct {
	streamCh    chan StreamChunk
	capabilities AgentCapabilities
	closed      bool
}

func newMockProvider(content string) *mockProvider {
	ch := make(chan StreamChunk, 10)
	ch <- StreamChunk{Content: content, Done: true}
	close(ch)
	return &mockProvider{
		streamCh: ch,
		capabilities: AgentCapabilities{
			SupportsImages:    true,
			SupportsTools:     true,
			SupportsStreaming: true,
			MaxTokens:         4096,
		},
	}
}

func newMockProviderWithToolCalls(content string, toolCalls []ToolCallRequestInput) *mockProvider {
	ch := make(chan StreamChunk, 10)
	if content != "" {
		ch <- StreamChunk{Content: content}
	}
	for _, tc := range toolCalls {
		tc := tc
		ch <- StreamChunk{ToolCall: &tc}
	}
	ch <- StreamChunk{Done: true}
	close(ch)
	return &mockProvider{
		streamCh: ch,
		capabilities: AgentCapabilities{
			SupportsTools: true,
		},
	}
}

func newMockProviderError(err error) *mockProvider {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Error: err, Done: true}
	close(ch)
	return &mockProvider{streamCh: ch}
}

func (p *mockProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	return p.streamCh, nil
}

func (p *mockProvider) Capabilities() AgentCapabilities { return p.capabilities }

func (p *mockProvider) HealthCheck(ctx context.Context) error { return nil }

func (p *mockProvider) Close() error {
	p.closed = true
	return nil
}

// ======= Mock Tool =======

type mockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, args map[string]any) (string, error)
	role        string
}

func (t *mockTool) Name() string              { return t.name }
func (t *mockTool) Description() string       { return t.description }
func (t *mockTool) RequiredRole() string      { return t.role }
func (t *mockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{"type": "string"},
		},
	}
}
func (t *mockTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, args)
	}
	return "mock result", nil
}

// ======= ProviderRegistry Tests =======

func TestProviderRegistry_AllBuiltInRegistered(t *testing.T) {
	t.Parallel()
	r := NewProviderRegistry()
	types := r.SupportedTypes()
	expected := []string{"openrouter", "local", "mimo", "webhook", "websocket", "subprocess", "mcp", "reve"}

	for _, e := range expected {
		found := false
		for _, t := range types {
			if t == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected provider %q to be registered", e)
		}
	}
}

func TestProviderRegistry_CreateUnknown(t *testing.T) {
	t.Parallel()
	r := NewProviderRegistry()
	_, err := r.Create("nonexistent_provider", nil, "key")
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
	if !strings.Contains(err.Error(), "unknown provider type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProviderRegistry_RegisterCustom(t *testing.T) {
	t.Parallel()
	r := NewProviderRegistry()
	called := false
	r.Register("custom", func(config map[string]any, apiKey string) (AgentProvider, error) {
		called = true
		return newMockProvider("custom"), nil
	})

	p, err := r.Create("custom", nil, "key")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !called {
		t.Error("custom factory was not called")
	}
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

func TestProviderRegistry_ConcurrentRegister(t *testing.T) {
	t.Parallel()
	r := NewProviderRegistry()
	for i := 0; i < 10; i++ {
		go func(n int) {
			r.Register(fmt.Sprintf("p%d", n), func(config map[string]any, apiKey string) (AgentProvider, error) {
				return newMockProvider("ok"), nil
			})
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	types := r.SupportedTypes()
	if len(types) < 8 { // at least built-in 8
		t.Errorf("expected at least 8 providers, got %d", len(types))
	}
}

// ======= ToolRegistry Tests =======

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	tool := &mockTool{name: "test_tool", description: "A test tool"}
	r.Register(tool)

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("expected to find test_tool")
	}
	if got.Name() != "test_tool" {
		t.Errorf("expected name test_tool, got %s", got.Name())
	}
}

func TestToolRegistry_GetNotFound(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestToolRegistry_GetAll(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&mockTool{name: "a"})
	r.Register(&mockTool{name: "b"})
	r.Register(&mockTool{name: "c"})

	all := r.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 tools, got %d", len(all))
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&mockTool{name: "echo", executeFunc: func(ctx context.Context, args map[string]any) (string, error) {
		return "echoed", nil
	}})

	result, err := r.Execute(context.Background(), "echo", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "echoed" {
		t.Errorf("expected 'echoed', got %q", result)
	}
}

func TestToolRegistry_Execute_NotFound(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	_, err := r.Execute(context.Background(), "missing", nil)
	if err == nil {
		t.Error("expected error for missing tool")
	}
	vartnf, ok := err.(*toolNotFoundError)
	if !ok {
		t.Errorf("expected toolNotFoundError, got %T", err)
	} else if vartnf.name != "missing" {
		t.Errorf("expected name 'missing', got %q", vartnf.name)
	}
}

func TestToolRegistry_GetDefs_Whitelist(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&mockTool{name: "tool_a"})
	r.Register(&mockTool{name: "tool_b"})
	r.Register(&mockTool{name: "tool_c"})

	agent := &AgentV2{
		ToolsEnabled:  true,
		ToolWhitelist: []string{"tool_a", "tool_c"},
	}

	defs := r.GetDefs(agent)
	if len(defs) != 2 {
		t.Errorf("expected 2 defs (whitelisted), got %d", len(defs))
	}
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	if !names["tool_a"] || !names["tool_c"] {
		t.Error("expected tool_a and tool_c in defs")
	}
	if names["tool_b"] {
		t.Error("tool_b should be filtered out by whitelist")
	}
}

func TestToolRegistry_GetDefs_NoWhitelist(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&mockTool{name: "a"})
	r.Register(&mockTool{name: "b"})

	agent := &AgentV2{ToolsEnabled: true}
	defs := r.GetDefs(agent)
	if len(defs) != 2 {
		t.Errorf("expected 2 defs (no whitelist = all), got %d", len(defs))
	}
}

func TestToolRegistry_ListInfo(t *testing.T) {
	t.Parallel()
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&mockTool{name: "alpha", description: "Alpha tool", role: "user"})
	r.Register(&mockTool{name: "beta", description: "Beta tool", role: "admin"})

	infos := r.ListInfo()
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	for _, info := range infos {
		if info.Name == "alpha" {
			if info.RequiredRole != "user" {
				t.Errorf("alpha role: got %q, want user", info.RequiredRole)
			}
		}
		if info.Name == "beta" {
			if info.RequiredRole != "admin" {
				t.Errorf("beta role: got %q, want admin", info.RequiredRole)
			}
		}
	}
}

// ======= HybridRouter Tests =======

func TestHybridRouter_BoundAgent(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{BoundAgentID: "bound-agent-1", BindUntilMsg: 5}
	agentID, err := r.Route(context.Background(), "user1", "hello", chat)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if agentID != "bound-agent-1" {
		t.Errorf("expected bound-agent-1, got %q", agentID)
	}
}

func TestHybridRouter_ExplicitAgent(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{AgentID: "explicit-agent"}
	agentID, err := r.Route(context.Background(), "user1", "hello", chat)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if agentID != "explicit-agent" {
		t.Errorf("expected explicit-agent, got %q", agentID)
	}
}

func TestHybridRouter_BoundOverExplicit(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{AgentID: "explicit", BoundAgentID: "bound", BindUntilMsg: 3}
	agentID, _ := r.Route(context.Background(), "user1", "hi", chat)
	if agentID != "bound" {
		t.Errorf("bound should take precedence, got %q", agentID)
	}
}

func TestHybridRouter_KeywordCode(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "fix this bug in my code", chat)
	if agentID != "developer" {
		t.Errorf("expected developer for code keyword, got %q", agentID)
	}
}

func TestHybridRouter_KeywordDeploy(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "deploy to server", chat)
	if agentID != "devops" {
		t.Errorf("expected devops for deploy keyword, got %q", agentID)
	}
}

func TestHybridRouter_KeywordTranslate(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "переведи этот текст", chat)
	if agentID != "translator" {
		t.Errorf("expected translator for Russian keyword, got %q", agentID)
	}
}

func TestHybridRouter_KeywordWrite(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "write me a story", chat)
	if agentID != "writer" {
		t.Errorf("expected writer, got %q", agentID)
	}
}

func TestHybridRouter_KeywordAnalyze(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "analyze the metrics", chat)
	if agentID != "analyst" {
		t.Errorf("expected analyst, got %q", agentID)
	}
}

func TestHybridRouter_DefaultAssistant(t *testing.T) {
	t.Parallel()
	r := NewHybridRouter(nil)
	chat := &AIChatV2{}
	agentID, _ := r.Route(context.Background(), "user1", "hello how are you", chat)
	if agentID != "assistant" {
		t.Errorf("expected assistant for unmatched message, got %q", agentID)
	}
}

// ======= AgentExecutor Tests =======

func TestAgentExecutor_SimpleResponse(t *testing.T) {
	t.Parallel()
	registry := NewProviderRegistry()
	// Override openrouter with mock
	registry.Register("mock", func(config map[string]any, apiKey string) (AgentProvider, error) {
		return newMockProvider("Hello from mock!"), nil
	})
	tools := &ToolRegistry{tools: make(map[string]Tool)}
	executor := NewAgentExecutor(nil, registry, tools)

	agent := &AgentV2{
		ID:           "test-agent",
		ProviderType: "mock",
		Model:        "test-model",
	}

	var chunks []string
	var finished bool
	result, err := executor.Execute(context.Background(), agent, []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil, func(token string, f bool) error {
		if !f {
			chunks = append(chunks, token)
		}
		finished = f
		return nil
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !finished {
		t.Error("expected finished=true")
	}
	if result.ModelUsed != "test-model" {
		t.Errorf("expected model test-model, got %q", result.ModelUsed)
	}
}

func TestAgentExecutor_WithToolCalls(t *testing.T) {
	t.Parallel()
	registry := NewProviderRegistry()
	registry.Register("mock", func(config map[string]any, apiKey string) (AgentProvider, error) {
		return newMockProviderWithToolCalls("Let me search...", []ToolCallRequestInput{
			{ID: "call-1", Name: "echo_tool", Arguments: `{"input":"test"}`},
		}), nil
	})
	tools := &ToolRegistry{tools: make(map[string]Tool)}
	tools.Register(&mockTool{
		name:        "echo_tool",
		executeFunc: func(ctx context.Context, args map[string]any) (string, error) {
			return "echo result", nil
		},
	})
	executor := NewAgentExecutor(nil, registry, tools)

	agent := &AgentV2{
		ID:           "test-agent",
		ProviderType: "mock",
		ToolsEnabled: true,
	}

	result, err := executor.Execute(context.Background(), agent, []AIMessageInput{
		{Role: "user", Content: "search for something"},
	}, nil, func(token string, f bool) error { return nil })

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestAgentExecutor_ModelOverride(t *testing.T) {
	t.Parallel()
	registry := NewProviderRegistry()
	var receivedModel string
	registry.Register("mock", func(config map[string]any, apiKey string) (AgentProvider, error) {
		receivedModel, _ = config["default_model"].(string)
		return newMockProvider("ok"), nil
	})
	tools := &ToolRegistry{tools: make(map[string]Tool)}
	executor := NewAgentExecutor(nil, registry, tools)

	agent := &AgentV2{
		ID:           "test-agent",
		ProviderType: "mock",
		Model:        "original-model",
	}
	settings := &AIChatSettings{ModelOverride: "override-model"}

	executor.Execute(context.Background(), agent, []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, settings, func(token string, f bool) error { return nil })

	if receivedModel != "override-model" {
		t.Errorf("expected override-model in config, got %q", receivedModel)
	}
}

func TestAgentExecutor_CloseProvider(t *testing.T) {
	t.Parallel()
	registry := NewProviderRegistry()
	mock := newMockProvider("ok")
	registry.Register("mock", func(config map[string]any, apiKey string) (AgentProvider, error) {
		return mock, nil
	})
	tools := &ToolRegistry{tools: make(map[string]Tool)}
	executor := NewAgentExecutor(nil, registry, tools)

	agent := &AgentV2{ProviderType: "mock"}
	executor.Execute(context.Background(), agent, []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil, func(token string, f bool) error { return nil })

	if !mock.closed {
		t.Error("provider should be closed after Execute")
	}
}

func TestAgentExecutor_UnknownProvider(t *testing.T) {
	t.Parallel()
	registry := NewProviderRegistry()
	tools := &ToolRegistry{tools: make(map[string]Tool)}
	executor := NewAgentExecutor(nil, registry, tools)

	agent := &AgentV2{ProviderType: "nonexistent"}
	_, err := executor.Execute(context.Background(), agent, []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil, func(token string, f bool) error { return nil })

	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

// ======= resolveAPIKey Tests =======

func TestResolveAPIKey_FromSettings(t *testing.T) {
	t.Parallel()
	agent := &AgentV2{ProviderType: "openrouter"}
	settings := &AIChatSettings{UserAPIKey: "user-key-123"}

	key := resolveAPIKey(agent, settings)
	if key != "user-key-123" {
		t.Errorf("expected user-key-123, got %q", key)
	}
}

func TestResolveAPIKey_FromAgentConfig(t *testing.T) {
	t.Parallel()
	agent := &AgentV2{
		ProviderType:   "openrouter",
		ProviderConfig: map[string]any{"api_key": "agent-key-456"},
	}

	key := resolveAPIKey(agent, nil)
	if key != "agent-key-456" {
		t.Errorf("expected agent-key-456, got %q", key)
	}
}

func TestResolveAPIKey_FromEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "env-key-789")

	agent := &AgentV2{ProviderType: "openrouter"}
	key := resolveAPIKey(agent, nil)
	if key != "env-key-789" {
		t.Errorf("expected env-key-789, got %q", key)
	}
}

func TestResolveAPIKey_PrioritySettingsOverAgent(t *testing.T) {
	t.Parallel()
	agent := &AgentV2{
		ProviderType:   "openrouter",
		ProviderConfig: map[string]any{"api_key": "agent-key"},
	}
	settings := &AIChatSettings{UserAPIKey: "settings-key"}

	key := resolveAPIKey(agent, settings)
	if key != "settings-key" {
		t.Errorf("settings should take priority, got %q", key)
	}
}

func TestResolveAPIKey_NoKey(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	agent := &AgentV2{ProviderType: "unknown_type"}
	key := resolveAPIKey(agent, nil)
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}
}

// ======= ToolCache Tests =======

func TestToolCache_SetGet(t *testing.T) {
	t.Parallel()
	cache := newToolCache(time.Minute, 100)
	cache.Set("key1", "result1")

	got, ok := cache.Get("key1")
	if !ok || got != "result1" {
		t.Errorf("expected result1, got %q (ok=%v)", got, ok)
	}
}

func TestToolCache_Expiry(t *testing.T) {
	t.Parallel()
	cache := newToolCache(50*time.Millisecond, 100)
	cache.Set("key1", "result1")

	time.Sleep(100 * time.Millisecond)

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected cache miss after TTL")
	}
}

func TestToolCache_MaxSize(t *testing.T) {
	t.Parallel()
	cache := newToolCache(time.Minute, 3)
	for i := 0; i < 5; i++ {
		cache.Set(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()

	if size > 3 {
		t.Errorf("expected max 3 entries, got %d", size)
	}
}

func TestToolCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	cache := newToolCache(time.Minute, 1000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", n%50)
			cache.Set(key, fmt.Sprintf("val%d", n))
			cache.Get(key)
		}(i)
	}
	wg.Wait()
}

// ======= CachedTool Tests =======

func TestCachedTool_CachesResult(t *testing.T) {
	t.Parallel()
	cache := newToolCache(time.Minute, 100)
	callCount := 0
	inner := &mockTool{
		name: "counting",
		executeFunc: func(ctx context.Context, args map[string]any) (string, error) {
			callCount++
			return "result", nil
		},
	}
	cached := &cachedTool{
		inner: inner,
		cache: cache,
		keyFn: func(args map[string]any) string {
			q, _ := args["query"].(string)
			return "k:" + q
		},
	}

	r1, _ := cached.Execute(context.Background(), map[string]any{"query": "test"})
	r2, _ := cached.Execute(context.Background(), map[string]any{"query": "test"})

	if r1 != "result" || r2 != "result" {
		t.Errorf("unexpected results: %q, %q", r1, r2)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (cached), got %d", callCount)
	}
}

func TestCachedTool_DifferentKeys(t *testing.T) {
	t.Parallel()
	cache := newToolCache(time.Minute, 100)
	callCount := 0
	inner := &mockTool{
		name: "counter",
		executeFunc: func(ctx context.Context, args map[string]any) (string, error) {
			callCount++
			return "ok", nil
		},
	}
	cached := &cachedTool{
		inner: inner,
		cache: cache,
		keyFn: func(args map[string]any) string {
			q, _ := args["query"].(string)
			return "k:" + q
		},
	}

	cached.Execute(context.Background(), map[string]any{"query": "a"})
	cached.Execute(context.Background(), map[string]any{"query": "b"})

	if callCount != 2 {
		t.Errorf("expected 2 calls for different keys, got %d", callCount)
	}
}

// ======= isURLSafe (SSRF) Tests =======

func TestIsURLSafe_ValidHTTPS(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("https://example.com"); err != nil {
		t.Errorf("https://example.com should be safe: %v", err)
	}
}

func TestIsURLSafe_ValidHTTP(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://example.com"); err != nil {
		t.Errorf("http://example.com should be safe: %v", err)
	}
}

func TestIsURLSafe_FTPBlocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("ftp://example.com"); err == nil {
		t.Error("ftp should be blocked")
	}
}

func TestIsURLSafe_LocalhostBlocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://localhost/admin"); err == nil {
		t.Error("localhost should be blocked")
	}
}

func TestIsURLSafe_IP127Blocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://127.0.0.1/admin"); err == nil {
		t.Error("127.0.0.1 should be blocked")
	}
}

func TestIsURLSafe_IPv6LoopbackBlocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://[::1]/admin"); err == nil {
		t.Error("::1 should be blocked")
	}
}

func TestIsURLSafe_MetadataEndpointBlocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://169.254.169.254/metadata"); err == nil {
		t.Error("cloud metadata endpoint should be blocked")
	}
}

func TestIsURLSafe_GoogleMetadataBlocked(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("http://metadata.google.internal/computeMetadata/v1/"); err == nil {
		t.Error("google metadata should be blocked")
	}
}

func TestIsURLSafe_EmptyURL(t *testing.T) {
	t.Parallel()
	if err := isURLSafe(""); err == nil {
		t.Error("empty URL should be blocked")
	}
}

func TestIsURLSafe_NoScheme(t *testing.T) {
	t.Parallel()
	if err := isURLSafe("example.com/path"); err == nil {
		t.Error("URL without scheme should be blocked")
	}
}

// ======= OpenRouter SSE Stream Parsing =======

func TestOpenRouterProvider_SSEStreamParsing(t *testing.T) {
	t.Parallel()
	// Mock OpenRouter SSE endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		// Send SSE chunks
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	provider := &openRouterProvider{
		apiKey:  "test-key",
		model:   "test-model",
		client:  mockServer.Client(),
		baseURL: mockServer.URL,
	}

	ch, err := provider.StreamChat(context.Background(), []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var content string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		if chunk.Content != "" {
			content += chunk.Content
		}
		if chunk.Done {
			break
		}
	}

	if content != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", content)
	}
}

func TestOpenRouterProvider_NoAPIKey(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	provider := &openRouterProvider{
		apiKey:  "",
		model:   "test",
		client:  &http.Client{},
		baseURL: "https://openrouter.ai/api/v1",
	}

	_, err := provider.StreamChat(context.Background(), []AIMessageInput{}, nil)
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestOpenRouterProvider_Capabilities(t *testing.T) {
	t.Parallel()
	provider := &openRouterProvider{model: "test"}
	caps := provider.Capabilities()
	if !caps.SupportsImages {
		t.Error("expected SupportsImages=true")
	}
	if !caps.SupportsTools {
		t.Error("expected SupportsTools=true")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming=true")
	}
	if caps.MaxTokens != 128000 {
		t.Errorf("expected MaxTokens=128000, got %d", caps.MaxTokens)
	}
}

func TestOpenRouterProvider_HealthCheck(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	provider := &openRouterProvider{apiKey: "test-key"}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestOpenRouterProvider_HealthCheck_NoKey(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	provider := &openRouterProvider{apiKey: ""}
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Error("expected error for missing API key")
	}
}

// ======= Helper function tests =======

func TestConvertMessages(t *testing.T) {
	t.Parallel()
	msgs := []AIMessageInput{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
	}
	result := convertMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0]["role"] != "system" || result[0]["content"] != "You are helpful" {
		t.Error("first message mismatch")
	}
	if result[1]["role"] != "user" || result[1]["content"] != "Hello" {
		t.Error("second message mismatch")
	}
}

func TestConvertToolDefs(t *testing.T) {
	t.Parallel()
	defs := []ToolDefInput{
		{
			Type: "function",
			Function: ToolDefFuncInput{
				Name:        "search",
				Description: "Search the web",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	result := convertToolDefs(defs)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(result))
	}
	fn, ok := result[0]["function"].(map[string]any)
	if !ok {
		t.Fatal("expected function map")
	}
	if fn["name"] != "search" {
		t.Errorf("expected name 'search', got %v", fn["name"])
	}
}

func TestJoinStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input []string
		sep   string
		want  string
	}{
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"a"}, ", ", "a"},
		{[]string{}, ", ", ""},
		{[]string{"x", "y"}, "\n", "x\ny"},
	}
	for _, tt := range tests {
		got := joinStrings(tt.input, tt.sep)
		if got != tt.want {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.input, tt.sep, got, tt.want)
		}
	}
}

// ======= AIGateway ChatName Tests =======

func TestAIGateway_GenerateChatName(t *testing.T) {
	t.Parallel()
	g := &AIGateway{}
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "AI Chat"},
		{"agent", "Agent Chat"},
		{"pipeline", "Pipeline Chat"},
		{"unknown", "AI Chat"},
		{"", "AI Chat"},
	}
	for _, tt := range tests {
		got := g.generateChatName(tt.input)
		if got != tt.want {
			t.Errorf("generateChatName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ======= AIGateway getUserLock Tests =======

func TestAIGateway_GetUserLock(t *testing.T) {
	t.Parallel()
	g := &AIGateway{}
	lock1 := g.getUserLock("user1")
	lock2 := g.getUserLock("user1")
	lock3 := g.getUserLock("user2")

	if lock1 != lock2 {
		t.Error("same user should get same lock")
	}
	if lock1 == lock3 {
		t.Error("different users should get different locks")
	}
}

// ======= AIGateway recordUsage Tests =======

func TestAIGateway_RecordUsage_ZeroTokens(t *testing.T) {
	t.Parallel()
	// Should not panic or do anything with 0 tokens
	g := &AIGateway{}
	g.recordUsage("user1", "agent1", 0)
	g.recordUsage("user1", "agent1", -5)
	// No assertion needed — just ensure no panic
}

// ======= queryDatabaseTool Security Tests =======

func TestQueryDatabaseTool_Security_OnlySelect(t *testing.T) {
	t.Parallel()
	tool := &queryDatabaseTool{}
	// Only test blocked queries (security validation path, no DB needed)
	blocked := []string{
		"DROP TABLE users",
		"DELETE FROM users",
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name='x'",
		"ALTER TABLE users ADD col INT",
		"TRUNCATE TABLE users",
		"CREATE TABLE evil (id INT)",
	}
	for _, q := range blocked {
		_, err := tool.Execute(context.Background(), map[string]any{"query": q})
		if err == nil {
			t.Errorf("expected error for query %q", q)
		}
	}
}

func TestQueryDatabaseTool_Security_NonSelectRejected(t *testing.T) {
	t.Parallel()
	tool := &queryDatabaseTool{}
	nonSelect := []string{
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name='x'",
		"DELETE FROM users",
		"DROP TABLE users",
	}
	for _, q := range nonSelect {
		_, err := tool.Execute(context.Background(), map[string]any{"query": q})
		if err == nil {
			t.Errorf("non-SELECT query %q should be rejected", q)
		}
	}
}

func TestQueryDatabaseTool_Security_BlockedKeywords(t *testing.T) {
	t.Parallel()
	tool := &queryDatabaseTool{}
	blocked := []string{"DROP", "DELETE", "INSERT", "UPDATE", "ALTER", "TRUNCATE", "CREATE",
		"EXEC", "GRANT", "REVOKE", "PG_READ_FILE", "WITH", "RECURSIVE"}
	for _, kw := range blocked {
		query := fmt.Sprintf("SELECT * FROM foo WHERE %s = 1", kw)
		_, err := tool.Execute(context.Background(), map[string]any{"query": query})
		if err == nil {
			t.Errorf("expected error for keyword %q in query", kw)
		}
	}
}

func TestQueryDatabaseTool_Security_BlockedTables(t *testing.T) {
	t.Parallel()
	tool := &queryDatabaseTool{}
	blocked := []string{"users", "user_tokens", "user_devices", "hermes_sessions", "ai_agents_v2"}
	for _, tbl := range blocked {
		query := fmt.Sprintf("SELECT * FROM %s", tbl)
		_, err := tool.Execute(context.Background(), map[string]any{"query": query})
		if err == nil {
			t.Errorf("expected error for blocked table %q", tbl)
		}
	}
}

func TestQueryDatabaseTool_EmptyQuery(t *testing.T) {
	t.Parallel()
	tool := &queryDatabaseTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

// ======= webFetchTool Tests =======

func TestWebFetchTool_EmptyURL(t *testing.T) {
	t.Parallel()
	tool := &webFetchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestWebFetchTool_BlockedLocalhost(t *testing.T) {
	t.Parallel()
	tool := &webFetchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"url": "http://localhost/admin"})
	if err == nil {
		t.Error("expected error for localhost URL")
	}
}

// ======= webSearchTool Tests =======

func TestWebSearchTool_EmptyQuery(t *testing.T) {
	t.Parallel()
	tool := &webSearchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestWebSearchTool_MockServer(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"Abstract": "Test abstract result",
			"Results": []map[string]string{
				{"Text": "Result 1", "FirstURL": "http://example.com/1"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Can't easily test webSearchTool with mock since it hardcodes DuckDuckGo URL
	// But we verify the tool interface
	tool := &webSearchTool{}
	if tool.Name() != "web_search" {
		t.Errorf("expected name 'web_search', got %q", tool.Name())
	}
	if tool.RequiredRole() != "user" {
		t.Errorf("expected role 'user', got %q", tool.RequiredRole())
	}
}

// ======= searchMessagesTool Tests =======

func TestSearchMessagesTool_EmptyQuery(t *testing.T) {
	t.Parallel()
	tool := &searchMessagesTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchMessagesTool_ParameterSchema(t *testing.T) {
	t.Parallel()
	tool := &searchMessagesTool{}
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Error("expected type 'object'")
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["query"]; !ok {
		t.Error("expected 'query' property")
	}
}

// ======= searchUsersTool Tests =======

func TestSearchUsersTool_EmptyQuery(t *testing.T) {
	t.Parallel()
	tool := &searchUsersTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

// ======= getChatInfoTool Tests =======

func TestGetChatInfoTool_EmptyChatID(t *testing.T) {
	t.Parallel()
	tool := &getChatInfoTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty chat_id")
	}
}

// ======= Mock OpenRouter API (SSE) =======

func TestMockOpenRouterAPI_SSE(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{"Hello", " ", "World"}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	provider := &openRouterProvider{
		apiKey:  "test-key",
		model:   "test-model",
		client:  mockServer.Client(),
		baseURL: mockServer.URL,
	}

	ch, err := provider.StreamChat(context.Background(), []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var content string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		if chunk.Content != "" {
			content += chunk.Content
		}
	}

	if content != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", content)
	}
}

func TestMockOpenRouterAPI_ToolCalls(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send text content
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Let me search\"}}]}\n\n")
		flusher.Flush()

		// Send tool call
		toolCallJSON := `{"choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"query\":\"test\"}"}}]}}]}`
		fmt.Fprintf(w, "data: %s\n\n", toolCallJSON)
		flusher.Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	provider := &openRouterProvider{
		apiKey:  "test-key",
		model:   "test-model",
		client:  mockServer.Client(),
		baseURL: mockServer.URL,
	}

	ch, err := provider.StreamChat(context.Background(), []AIMessageInput{
		{Role: "user", Content: "search for test"},
	}, []ToolDefInput{{
		Type: "function",
		Function: ToolDefFuncInput{
			Name:        "search",
			Description: "Search",
		},
	}})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var content string
	var toolCalls int
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		if chunk.Content != "" {
			content += chunk.Content
		}
		if chunk.ToolCall != nil {
			toolCalls++
			if chunk.ToolCall.Name != "search" {
				t.Errorf("expected tool name 'search', got %q", chunk.ToolCall.Name)
			}
		}
	}

	if content != "Let me search" {
		t.Errorf("expected 'Let me search', got %q", content)
	}
	if toolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", toolCalls)
	}
}

func TestMockOpenRouterAPI_HTTPError(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer mockServer.Close()

	provider := &openRouterProvider{
		apiKey:  "test-key",
		model:   "test-model",
		client:  mockServer.Client(),
		baseURL: mockServer.URL,
	}

	_, err := provider.StreamChat(context.Background(), []AIMessageInput{
		{Role: "user", Content: "Hi"},
	}, nil)
	if err == nil {
		t.Error("expected error for 429 response")
	}
}
