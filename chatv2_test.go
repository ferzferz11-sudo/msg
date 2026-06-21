package main

import (
	"LavenderMessenger/gen"
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockChatV2Stream implements gen.ChatService_ChatV2Server for testing
type mockChatV2Stream struct {
	grpc.ServerStream
	ctx     context.Context
	sent    []*gen.ChatV2Message
	recvCh  chan *gen.ChatV2Message
	room    string
	closeCh chan struct{}
}

func newMockChatV2Stream(ctx context.Context) *mockChatV2Stream {
	return &mockChatV2Stream{
		ctx:     ctx,
		sent:    make([]*gen.ChatV2Message, 0),
		recvCh:  make(chan *gen.ChatV2Message, 10),
		closeCh: make(chan struct{}),
	}
}

func (m *mockChatV2Stream) Send(msg *gen.ChatV2Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockChatV2Stream) Context() context.Context {
	return m.ctx
}

func (m *mockChatV2Stream) SendHeader(metadata.MD) error { return nil }
func (m *mockChatV2Stream) SetHeader(metadata.MD) error  { return nil }
func (m *mockChatV2Stream) SendMsg(interface{}) error     { return nil }
func (m *mockChatV2Stream) RecvMsg(interface{}) error     { return nil }
func (m *mockChatV2Stream) SetTrailer(metadata.MD)        {}

func (m *mockChatV2Stream) Recv() (*gen.ChatV2Message, error) {
	msg, ok := <-m.recvCh
	if !ok {
		return nil, context.Canceled
	}
	return msg, nil
}

func (m *mockChatV2Stream) send(msg *gen.ChatV2Message) {
	m.recvCh <- msg
}

func (m *mockChatV2Stream) close() {
	close(m.recvCh)
}

// ======= Hub V2 methods tests =======

func TestHubV2_RegisterUnregister(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)

	h.mu.RLock()
	if len(h.v2Clients) != 1 {
		t.Errorf("expected 1 v2 client, got %d", len(h.v2Clients))
	}
	h.mu.RUnlock()

	h.UnregisterV2(stream)

	h.mu.RLock()
	if len(h.v2Clients) != 0 {
		t.Errorf("expected 0 v2 clients after unregister, got %d", len(h.v2Clients))
	}
	h.mu.RUnlock()
}

func TestHubV2_SetRoomAndSnapshot(t *testing.T) {
	h := NewHub(nil)
	stream1 := newMockChatV2Stream(context.Background())
	stream2 := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream1)
	h.RegisterV2(stream2)
	h.SetV2Room(stream1, "room-a")
	h.SetV2Room(stream2, "room-a")

	targets := h.SnapshotRoomStreams("room-a")
	if len(targets) != 2 {
		t.Errorf("expected 2 streams in room-a, got %d", len(targets))
	}

	targets = h.SnapshotRoomStreams("room-b")
	if len(targets) != 0 {
		t.Errorf("expected 0 streams in room-b, got %d", len(targets))
	}

	h.UnregisterV2(stream1)
	h.UnregisterV2(stream2)
}

func TestHubV2_SetUserId(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2UserId(stream, "uuid-123")

	h.mu.RLock()
	if h.v2UserIds[stream] != "uuid-123" {
		t.Errorf("expected userId uuid-123, got %s", h.v2UserIds[stream])
	}
	if !h.userIdSet["uuid-123"] {
		t.Error("expected uuid-123 in userIdSet")
	}
	h.mu.RUnlock()

	h.UnregisterV2(stream)

	h.mu.RLock()
	if h.userIdSet["uuid-123"] {
		t.Error("expected uuid-123 removed from userIdSet after unregister")
	}
	h.mu.RUnlock()
}

func TestHubV2_SetUsername(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2Username(stream, "alice")

	h.mu.RLock()
	if h.v2Clients[stream] != "alice" {
		t.Errorf("expected username alice, got %s", h.v2Clients[stream])
	}
	if !h.usernameSet["alice"] {
		t.Error("expected alice in usernameSet")
	}
	h.mu.RUnlock()

	h.UnregisterV2(stream)

	h.mu.RLock()
	if h.usernameSet["alice"] {
		t.Error("expected alice removed from usernameSet after unregister")
	}
	h.mu.RUnlock()
}

func TestHubV2_SnapshotRoomStreams_Empty(t *testing.T) {
	h := NewHub(nil)
	targets := h.SnapshotRoomStreams("nonexistent")
	if len(targets) != 0 {
		t.Errorf("expected 0, got %d", len(targets))
	}
}

func TestHubV2_MultipleRooms(t *testing.T) {
	h := NewHub(nil)
	s1 := newMockChatV2Stream(context.Background())
	s2 := newMockChatV2Stream(context.Background())
	s3 := newMockChatV2Stream(context.Background())

	h.RegisterV2(s1)
	h.RegisterV2(s2)
	h.RegisterV2(s3)
	h.SetV2Room(s1, "room-1")
	h.SetV2Room(s2, "room-1")
	h.SetV2Room(s3, "room-2")

	if len(h.SnapshotRoomStreams("room-1")) != 2 {
		t.Errorf("expected 2 in room-1, got %d", len(h.SnapshotRoomStreams("room-1")))
	}
	if len(h.SnapshotRoomStreams("room-2")) != 1 {
		t.Errorf("expected 1 in room-2, got %d", len(h.SnapshotRoomStreams("room-2")))
	}

	h.UnregisterV2(s1)
	h.UnregisterV2(s2)
	h.UnregisterV2(s3)
}

func TestHubV2_GracePeriod(t *testing.T) {
	h := NewHub(nil)
	stream := newMockChatV2Stream(context.Background())

	h.RegisterV2(stream)
	h.SetV2Username(stream, "bob")
	h.UnregisterV2(stream)

	// Bob should be in grace period
	h.graceMu.Lock()
	_, inGrace := h.gracePeriods["bob"]
	h.graceMu.Unlock()

	if !inGrace {
		t.Error("expected bob to be in grace period after unregister")
	}
}

func TestHubV2_OnlineUsers(t *testing.T) {
	h := NewHub(nil)
	s1 := newMockChatV2Stream(context.Background())
	s2 := newMockChatV2Stream(context.Background())

	h.RegisterV2(s1)
	h.RegisterV2(s2)
	h.SetV2Username(s1, "alice")
	h.SetV2Username(s2, "bob")

	users := h.GetOnlineUsers()
	found := make(map[string]bool)
	for _, u := range users {
		found[u] = true
	}
	if !found["alice"] || !found["bob"] {
		t.Errorf("expected both alice and bob online, got %v", users)
	}

	h.UnregisterV2(s1)
	h.UnregisterV2(s2)
}
