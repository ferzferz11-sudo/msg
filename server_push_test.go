package main

import (
	"LavenderMessenger/gen"
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockChatStream implements gen.ChatService_ChatServer for testing
type mockChatStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockChatStream) Send(*gen.Message) error          { return nil }
func (m *mockChatStream) Context() context.Context          { return m.ctx }
func (m *mockChatStream) SendHeader(metadata.MD) error      { return nil }
func (m *mockChatStream) SetHeader(metadata.MD) error       { return nil }
func (m *mockChatStream) SendMsg(interface{}) error         { return nil }
func (m *mockChatStream) RecvMsg(interface{}) error         { return nil }
func (m *mockChatStream) SetTrailer(metadata.MD)            {}
func (m *mockChatStream) Recv() (*gen.Message, error)       { return nil, nil }

// ======= Hub.IsUserOnline tests =======

func TestIsUserOnline_UserPresent(t *testing.T) {
	h := NewHub(nil)
	stream := &mockChatStream{ctx: context.Background()}

	// Register client with userId (v2 JWT auth)
	h.Register(stream)
	h.SetUserId(stream, "user-uuid-123")
	h.UpdateName(stream, "testuser")
	h.SetAuthenticated(stream, true)

	// Should be online by userId
	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online (found by userId)")
	}
}

func TestIsUserOnline_UserNotPresent(t *testing.T) {
	h := NewHub(nil)

	// No clients registered
	if h.IsUserOnline("user-uuid-999", "unknown") {
		t.Error("User should not be online")
	}
}

func TestIsUserOnline_FallbackToUsername(t *testing.T) {
	h := NewHub(nil)
	stream := &mockChatStream{ctx: context.Background()}

	// v1 client: only username set, no userId
	h.Register(stream)
	h.UpdateName(stream, "testuser")
	h.SetAuthenticated(stream, true)

	// Should be online by username fallback
	if !h.IsUserOnline("", "testuser") {
		t.Error("User should be online (found by username fallback)")
	}
}

func TestIsUserOnline_BothUserIdAndUsername(t *testing.T) {
	h := NewHub(nil)
	stream := &mockChatStream{ctx: context.Background()}

	// v2 client: both userId and username set
	h.Register(stream)
	h.SetUserId(stream, "user-uuid-123")
	h.UpdateName(stream, "testuser")
	h.SetAuthenticated(stream, true)

	// Check by userId (primary)
	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online (found by userId)")
	}

	// Check by username fallback (when userId is empty)
	if !h.IsUserOnline("", "testuser") {
		t.Error("User should be online (found by username fallback)")
	}

	// Different userId but same username should still match via username
	if !h.IsUserOnline("different-uuid", "testuser") {
		t.Error("User should be online (username match)")
	}
}

func TestIsUserOnline_MultipleStreams(t *testing.T) {
	h := NewHub(nil)
	stream1 := &mockChatStream{ctx: context.Background()}
	stream2 := &mockChatStream{ctx: context.Background()}

	h.Register(stream1)
	h.SetUserId(stream1, "user-uuid-1")
	h.UpdateName(stream1, "user1")

	h.Register(stream2)
	h.SetUserId(stream2, "user-uuid-2")
	h.UpdateName(stream2, "user2")

	// Both users should be online
	if !h.IsUserOnline("user-uuid-1", "user1") {
		t.Error("User1 should be online")
	}
	if !h.IsUserOnline("user-uuid-2", "user2") {
		t.Error("User2 should be online")
	}

	// Non-existent user should not be online
	if h.IsUserOnline("user-uuid-999", "user999") {
		t.Error("Non-existent user should not be online")
	}
}

func TestIsUserOnline_AfterUnregister(t *testing.T) {
	h := NewHub(nil)
	stream := &mockChatStream{ctx: context.Background()}

	// Register a client
	h.Register(stream)
	h.SetUserId(stream, "user-uuid-123")
	h.UpdateName(stream, "testuser")

	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online before unregister")
	}

	// Unregister
	h.Unregister(stream)

	// Immediately after unregister, grace period is active (30s)
	// So user should still appear online
	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online during grace period after unregister")
	}
}

func TestIsUserOnline_GracePeriod(t *testing.T) {
	h := NewHub(nil)
	stream := &mockChatStream{ctx: context.Background()}

	// Register and unregister
	h.Register(stream)
	h.SetUserId(stream, "user-uuid-123")
	h.UpdateName(stream, "testtest")

	h.Unregister(stream)

	// During grace period, user should still appear online
	// (grace period is 30 seconds, so immediately after unregister it should be online)
	if !h.IsUserOnline("user-uuid-123", "testtest") {
		t.Error("User should be online during grace period")
	}
}
