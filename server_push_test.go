package main

import (
	"context"
	"testing"
)

// ======= Hub.IsUserOnline tests =======

func TestIsUserOnline_UserPresent(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2UserId(stream, "user-uuid-123")
	h.SetV2Username(stream, "testuser")

	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online (found by userId)")
	}
}

func TestIsUserOnline_UserNotPresent(t *testing.T) {
	h := NewHub(nil)

	if h.IsUserOnline("user-uuid-999", "unknown") {
		t.Error("User should not be online")
	}
}

func TestIsUserOnline_UsernameOnly(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2Username(stream, "testuser")

	if !h.IsUserOnline("", "testuser") {
		t.Error("User should be online (found by username)")
	}
}

func TestIsUserOnline_BothUserIdAndUsername(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2UserId(stream, "user-uuid-123")
	h.SetV2Username(stream, "testuser")

	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online (found by userId)")
	}

	if !h.IsUserOnline("", "testuser") {
		t.Error("User should be online (found by username fallback)")
	}

	if !h.IsUserOnline("different-uuid", "testuser") {
		t.Error("User should be online (username match)")
	}
}

func TestIsUserOnline_MultipleStreams(t *testing.T) {
	h := NewHub(nil)
	stream1 := newMockChatV2Stream(context.Background())
	stream2 := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream1)
	h.SetV2UserId(stream1, "user-uuid-1")
	h.SetV2Username(stream1, "user1")

	h.RegisterV2(stream2)
	h.SetV2UserId(stream2, "user-uuid-2")
	h.SetV2Username(stream2, "user2")

	if !h.IsUserOnline("user-uuid-1", "user1") {
		t.Error("User1 should be online")
	}
	if !h.IsUserOnline("user-uuid-2", "user2") {
		t.Error("User2 should be online")
	}

	if h.IsUserOnline("user-uuid-999", "user999") {
		t.Error("Non-existent user should not be online")
	}
}

func TestIsUserOnline_AfterUnregister(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2UserId(stream, "user-uuid-123")
	h.SetV2Username(stream, "testuser")

	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online before unregister")
	}

	h.UnregisterV2(stream)

	if !h.IsUserOnline("user-uuid-123", "testuser") {
		t.Error("User should be online during grace period after unregister")
	}
}

func TestIsUserOnline_GracePeriod(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2UserId(stream, "user-uuid-123")
	h.SetV2Username(stream, "testtest")

	h.UnregisterV2(stream)

	if !h.IsUserOnline("user-uuid-123", "testtest") {
		t.Error("User should be online during grace period")
	}
}
