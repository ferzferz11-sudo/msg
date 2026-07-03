package main

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"LavenderMessenger/gen"
)

func TestEncodeDecodeMessageCursor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := "test-msg-123"

	encoded := encodeMessageCursor(now, id)
	if encoded == "" {
		t.Fatal("encodeMessageCursor returned empty string")
	}

	decodedTime, decodedID := decodeMessageCursor(encoded)
	if decodedID != id {
		t.Errorf("expected id %q, got %q", id, decodedID)
	}
	if !decodedTime.Equal(now) {
		t.Errorf("expected time %v, got %v", now, decodedTime)
	}
}

func TestEncodeMessageCursorEmpty(t *testing.T) {
	decodedTime, decodedID := decodeMessageCursor("")
	if decodedID != "" {
		t.Errorf("expected empty id, got %q", decodedID)
	}
	if !decodedTime.IsZero() {
		t.Errorf("expected zero time, got %v", decodedTime)
	}
}

func TestRowToProtoV2_Text(t *testing.T) {
	now := time.Now().UTC()
	row := &MessageRowV2{
		ID:          "msg-123",
		RoomID:      "room-1",
		SenderID:    "user-uuid-123",
		ContentType: "text",
		Text:        "Hello, world!",
		IsRead:      false,
		CreatedAt:   now,
		Reactions:   `{"user-1":"👍","user-2":"❤️"}`,
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}
	if proto.Id != "msg-123" {
		t.Errorf("expected id msg-123, got %s", proto.Id)
	}
	if proto.RoomId != "room-1" {
		t.Errorf("expected room_id room-1, got %s", proto.RoomId)
	}
	if proto.SenderId != "user-uuid-123" {
		t.Errorf("expected sender_id user-uuid-123, got %s", proto.SenderId)
	}

	textContent, ok := proto.Content.(*gen.MessageV2_Text)
	if !ok {
		t.Fatal("expected text content")
	}
	if textContent.Text != "Hello, world!" {
		t.Errorf("expected text Hello, world!, got %s", textContent.Text)
	}

	var reactions map[string]string
	if err := json.Unmarshal(proto.Reactions, &reactions); err != nil {
		t.Fatalf("failed to unmarshal reactions: %v", err)
	}
	if reactions["user-1"] != "👍" {
		t.Errorf("expected 👍, got %s", reactions["user-1"])
	}
}

func TestRowToProtoV2_Media(t *testing.T) {
	row := &MessageRowV2{
		ID:          "msg-456",
		RoomID:      "room-2",
		SenderID:    "user-uuid-456",
		ContentType: "image",
		MediaURL:    "http://host:8082/images/test.jpg",
		MediaURLs:   `["http://host:8082/images/a.jpg","http://host:8082/images/b.jpg"]`,
		CreatedAt:   time.Now().UTC(),
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}

	mediaContent, ok := proto.Content.(*gen.MessageV2_Media)
	if !ok {
		t.Fatal("expected media content")
	}
	if mediaContent.Media.Type != "image" {
		t.Errorf("expected type image, got %s", mediaContent.Media.Type)
	}
	if mediaContent.Media.Url != "http://host:8082/images/test.jpg" {
		t.Errorf("expected url, got %s", mediaContent.Media.Url)
	}
	if len(mediaContent.Media.Urls) != 2 {
		t.Errorf("expected 2 urls, got %d", len(mediaContent.Media.Urls))
	}
}

func TestRowToProtoV2_E2EE(t *testing.T) {
	row := &MessageRowV2{
		ID:           "msg-e2ee",
		RoomID:       "room-3",
		SenderID:     "user-uuid-789",
		ContentType:  "text",
		IsE2EE:       true,
		E2EEPayload:  []byte("encrypted-data-here"),
		CreatedAt:    time.Now().UTC(),
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}
	if !proto.IsE2Ee {
		t.Error("expected is_e2ee to be true")
	}
	if proto.E2EePayload == "" {
		t.Error("expected e2ee_payload to be set")
	}
}

func TestRowToProtoV2_Nil(t *testing.T) {
	proto := rowToProtoV2(nil)
	if proto != nil {
		t.Error("expected nil for nil input")
	}
}

func TestRowToProtoV2_Reply(t *testing.T) {
	row := &MessageRowV2{
		ID:           "msg-reply",
		RoomID:       "room-4",
		SenderID:     "user-uuid-reply",
		ContentType:  "text",
		Text:         "This is a reply",
		ReplyToID:    sql.NullString{String: "orig-msg-123", Valid: true},
		ReplyPreview: sql.NullString{String: "Original message text...", Valid: true},
		CreatedAt:    time.Now().UTC(),
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}

	if proto.Reply == nil {
		t.Fatal("expected reply to be set")
	}
	if proto.Reply.MessageId != "orig-msg-123" {
		t.Errorf("expected reply message_id orig-msg-123, got %s", proto.Reply.MessageId)
	}
}

func TestRowToProtoV2_Deleted(t *testing.T) {
	row := &MessageRowV2{
		ID:          "msg-deleted",
		RoomID:      "room-5",
		SenderID:    "user-uuid-del",
		ContentType: "deleted",
		CreatedAt:   time.Now().UTC(),
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}

	textContent, ok := proto.Content.(*gen.MessageV2_Text)
	if !ok {
		t.Fatal("expected text content for deleted message")
	}
	if textContent.Text != "[deleted]" {
		t.Errorf("expected [deleted], got %s", textContent.Text)
	}
}
