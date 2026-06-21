package main

import (
	"LavenderMessenger/gen"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ======= Rate Limiter Tests =======

func TestBotRateLimiter_Allow(t *testing.T) {
	rl := NewRedisRateLimiter(3, time.Minute, "rl:bot:test:")
	userID := "test-user-1"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !rl.allow(userID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be blocked
	if rl.allow(userID) {
		t.Error("Request 4 should be blocked (rate limit exceeded)")
	}
}

func TestBotRateLimiter_DifferentUsers(t *testing.T) {
	rl := NewRedisRateLimiter(2, time.Minute, "rl:bot:test2:")

	// User 1: 2 requests allowed
	if !rl.allow("user1") {
		t.Error("User1 request 1 should be allowed")
	}
	if !rl.allow("user1") {
		t.Error("User1 request 2 should be allowed")
	}
	// User 1: 3rd blocked
	if rl.allow("user1") {
		t.Error("User1 request 3 should be blocked")
	}

	// User 2: still allowed (separate counter)
	if !rl.allow("user2") {
		t.Error("User2 request 1 should be allowed")
	}
}

func TestBotRateLimiter_WindowReset(t *testing.T) {
	rl := NewRedisRateLimiter(2, 100*time.Millisecond, "rl:bot:test3:")
	userID := "test-user-window"

	// Exhaust limit
	rl.allow(userID)
	rl.allow(userID)
	if rl.allow(userID) {
		t.Error("Should be blocked after 2 requests")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !rl.allow(userID) {
		t.Error("Should be allowed after window reset")
	}
}

func TestOwlRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(10, time.Minute)
	userID := "owl-user-1"

	// All 10 requests should be allowed
	for i := 0; i < 10; i++ {
		if !rl.allow(userID) {
			t.Errorf("OWL request %d should be allowed", i+1)
		}
	}

	// 11th should be blocked
	if rl.allow(userID) {
		t.Error("OWL request 11 should be blocked")
	}
}

// ======= Bot Command Handler Tests =======

func TestHandleBotStatus(t *testing.T) {
	s := &server{}
	req := &gen.BotCommandRequest{
		UserId:   "test-user",
		Username: "testuser",
		Command:  "/status",
	}

	resp := handleBotStatus(s, req)
	if !resp.Success {
		t.Error("handleBotStatus should return success")
	}
	if resp.ResponseText == "" {
		t.Error("handleBotStatus should return non-empty response")
	}
	if !strings.Contains(resp.ResponseText, "Статус сервера") {
		t.Error("handleBotStatus should contain status header")
	}
}

func TestHandleBotVersion(t *testing.T) {
	s := &server{}
	req := &gen.BotCommandRequest{
		UserId:   "test-user",
		Username: "testuser",
		Command:  "/version",
	}

	resp := handleBotVersion(s, req)
	if !resp.Success {
		t.Error("handleBotVersion should return success")
	}
	if !strings.Contains(resp.ResponseText, ServerVersion) {
		t.Errorf("handleBotVersion should contain version %s", ServerVersion)
	}
}

func TestHandleBotHelp(t *testing.T) {
	s := &server{}
	req := &gen.BotCommandRequest{
		UserId:   "test-user",
		Username: "testuser",
		Command:  "/help",
	}

	resp := handleBotHelp(s, req)
	if !resp.Success {
		t.Error("handleBotHelp should return success")
	}
	if !strings.Contains(resp.ResponseText, "Доступные команды") {
		t.Error("handleBotHelp should contain help header")
	}
	// Check that all 7 commands are listed
	expectedCommands := []string{"/status", "/deploy", "/logs", "/restart", "/ai", "/help", "/version"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(resp.ResponseText, cmd) {
			t.Errorf("handleBotHelp should contain command %s", cmd)
		}
	}
}

func TestHandleBotDeploy_NonAdmin(t *testing.T) {
	// Skip: requires a real DB connection for IsSuperAdmin check
	// The handler calls s.db.IsSuperAdmin() which panics on nil DB
	// In production, the server always has a valid DB connection
	t.Skip("Requires DB connection")
}

func TestHandleBotDeploy_InvalidTarget(t *testing.T) {
	// Skip: requires a real DB connection
	t.Skip("Requires DB connection")
}

func TestHandleBotRestart_NonAdmin(t *testing.T) {
	// Skip: requires a real DB connection for IsSuperAdmin check
	t.Skip("Requires DB connection")
}

func TestHandleBotAI_NoMessage(t *testing.T) {
	s := &server{}
	req := &gen.BotCommandRequest{
		UserId:   "test-user",
		Username: "testuser",
		Command:  "/ai",
		Args:     []string{},
	}

	resp := handleBotAI(s, req)
	if resp.Success {
		t.Error("handleBotAI should fail with no message")
	}
	if !resp.IsError {
		t.Error("handleBotAI should set IsError with no message")
	}
}

// ======= Bot Command Dispatcher Tests =======

func TestDispatchBotCommand_UnknownCommand(t *testing.T) {
	s := &server{}
	req := &gen.BotCommandRequest{
		UserId:   "test-user",
		Username: "testuser",
		Command:  "/unknown",
	}

	resp := dispatchBotCommand(s, req)
	if resp.Success {
		t.Error("dispatchBotCommand should fail for unknown command")
	}
	if !resp.IsError {
		t.Error("dispatchBotCommand should set IsError for unknown command")
	}
	if !strings.Contains(resp.ErrorMessage, "Неизвестная команда") {
		t.Error("Should return 'unknown command' error message")
	}
}

func TestDispatchBotCommand_KnownCommands(t *testing.T) {
	// Only test commands that don't require DB access
	s := &server{}

	tests := []struct {
		command string
		wantOk  bool
	}{
		{"/status", true},
		{"/version", true},
		{"/help", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			req := &gen.BotCommandRequest{
				UserId:   "test-user",
				Username: "testuser",
				Command:  tt.command,
			}
			resp := dispatchBotCommand(s, req)
			if resp.Success != tt.wantOk {
				t.Errorf("Command %s: success=%v (expected %v)", tt.command, resp.Success, tt.wantOk)
			}
		})
	}
}

func TestDispatchBotCommand_RateLimit(t *testing.T) {
	// Create a fresh rate limiter for this test
	originalLimiter := botCmdRateLimiter
	botCmdRateLimiter = NewRedisRateLimiter(2, time.Minute, "rl:bot:test:")
	defer func() { botCmdRateLimiter = originalLimiter }()

	s := &server{}
	userID := "rate-limit-test-user"

	// First 2 should succeed (or at least not be rate-limited)
	for i := 0; i < 2; i++ {
		req := &gen.BotCommandRequest{
			UserId:   userID,
			Username: "testuser",
			Command:  "/status",
		}
		resp := dispatchBotCommand(s, req)
		if !resp.Success {
			// Might fail for other reasons, but not rate limit
			if strings.Contains(resp.ErrorMessage, "Rate limit") {
				t.Errorf("Request %d should not be rate limited", i+1)
			}
		}
	}

	// 3rd should be rate limited
	req := &gen.BotCommandRequest{
		UserId:   userID,
		Username: "testuser",
		Command:  "/status",
	}
	resp := dispatchBotCommand(s, req)
	if resp.Success {
		t.Error("Request 3 should be rate limited")
	}
	if !strings.Contains(resp.ErrorMessage, "Rate limit") {
		t.Error("Should return rate limit error message")
	}
}

// ======= Bot Command Registry Tests =======

func TestBotCommandList_AllPresent(t *testing.T) {
	expected := map[string]bool{
		"/status":  false,
		"/deploy":  false,
		"/logs":    false,
		"/restart": false,
		"/ai":      false,
		"/help":    false,
		"/version": false,
	}

	for _, cmd := range botCommandList {
		if _, ok := expected[cmd.Command]; ok {
			expected[cmd.Command] = true
		}
	}

	for cmd, found := range expected {
		if !found {
			t.Errorf("Bot command %s not found in registry", cmd)
		}
	}
}

func TestBotCommandList_HasRequiredFields(t *testing.T) {
	for _, cmd := range botCommandList {
		if cmd.Command == "" {
			t.Error("Bot command should have non-empty Command")
		}
		if cmd.Description == "" {
			t.Errorf("Bot command %s should have non-empty Description", cmd.Command)
		}
		if cmd.Category == "" {
			t.Errorf("Bot command %s should have non-empty Category", cmd.Command)
		}
	}
}

// ======= Notification Service Tests =======

func TestNotificationService_Broadcast(t *testing.T) {
	ns := &notificationService{
		subscribers: make(map[string]map[chan *gen.ServerNotification]bool),
		maxHistory:  10,
	}

	ch := make(chan *gen.ServerNotification, 5)
	ns.subscribe("user1", ch)

	ns.broadcast(&gen.ServerNotification{
		Id:      "test-1",
		Type:    "info",
		Title:   "Test",
		Message: "Test message",
	})

	select {
	case notif := <-ch:
		if notif.Id != "test-1" {
			t.Errorf("Expected notification id 'test-1', got '%s'", notif.Id)
		}
		if notif.Title != "Test" {
			t.Errorf("Expected title 'Test', got '%s'", notif.Title)
		}
	default:
		t.Error("Notification should have been received")
	}
}

func TestNotificationService_History(t *testing.T) {
	ns := &notificationService{
		subscribers: make(map[string]map[chan *gen.ServerNotification]bool),
		maxHistory:  5,
	}

	// Add 10 notifications
	for i := 0; i < 10; i++ {
		ns.broadcast(&gen.ServerNotification{
			Id:   fmt.Sprintf("notif-%d", i),
			Type: "info",
		})
	}

	// History should be capped at maxHistory
	history := ns.getHistory("user1", 100)
	if len(history) != 5 {
		t.Errorf("Expected history length 5, got %d", len(history))
	}

	// getHistory returns last N entries in chronological order
	// After 10 inserts with maxHistory=5, we get notif-5..notif-9
	if history[0].Id != "notif-5" {
		t.Errorf("Expected oldest in history 'notif-5', got '%s'", history[0].Id)
	}
	if history[len(history)-1].Id != "notif-9" {
		t.Errorf("Expected most recent notification 'notif-9', got '%s'", history[len(history)-1].Id)
	}
}

func TestNotificationService_SubscribeUnsubscribe(t *testing.T) {
	ns := &notificationService{
		subscribers: make(map[string]map[chan *gen.ServerNotification]bool),
		maxHistory:  10,
	}

	ch := make(chan *gen.ServerNotification, 5)
	ns.subscribe("user1", ch)

	// Should have 1 subscriber
	ns.mu.Lock()
	count := len(ns.subscribers["user1"])
	ns.mu.Unlock()
	if count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}

	ns.unsubscribe("user1", ch)

	// Should have 0 subscribers
	ns.mu.Lock()
	count = len(ns.subscribers["user1"])
	ns.mu.Unlock()
	if count != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", count)
	}
}

// ======= Utility Function Tests =======

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{30 * time.Minute, "30м"},
		{2 * time.Hour, "2ч 0м"},
		{25 * time.Hour, "1д 1ч 0м"},
		{90 * time.Minute, "1ч 30м"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, result, tt.expected)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"hello", 5, "hello"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
