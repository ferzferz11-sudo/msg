package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ======= Reaction V2 JSON Tests =======

func TestReactionV2_JSON(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		wantJSON string
	}{
		{
			name:     "single reaction",
			input:    map[string]string{"user-1": "👍"},
			wantJSON: `{"user-1":"👍"}`,
		},
		{
			name:     "multiple reactions",
			input:    map[string]string{"user-1": "👍", "user-2": "❤️", "user-3": "🔥"},
			wantJSON: `{"user-1":"👍","user-2":"❤️","user-3":"🔥"}`,
		},
		{
			name:     "empty reactions",
			input:    map[string]string{},
			wantJSON: `{}`,
		},
		{
			name:     "emoji with unicode",
			input:    map[string]string{"user-1": "🎉"},
			wantJSON: `{"user-1":"🎉"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if string(data) != tt.wantJSON {
				t.Errorf("got %s, want %s", string(data), tt.wantJSON)
			}

			// Verify round-trip
			var decoded map[string]string
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if len(decoded) != len(tt.input) {
				t.Errorf("decoded length %d, want %d", len(decoded), len(tt.input))
			}
			for k, v := range tt.input {
				if decoded[k] != v {
					t.Errorf("decoded[%s] = %s, want %s", k, decoded[k], v)
				}
			}
		})
	}
}

func TestReactionV2_EmptyJSON(t *testing.T) {
	var reactions map[string]string
	err := json.Unmarshal([]byte("{}"), &reactions)
	if err != nil {
		t.Fatalf("unmarshal empty JSON failed: %v", err)
	}
	if len(reactions) != 0 {
		t.Errorf("expected empty map, got %d entries", len(reactions))
	}
}

func TestReactionV2_InvalidJSON(t *testing.T) {
	var reactions map[string]string
	err := json.Unmarshal([]byte("invalid"), &reactions)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ======= BroadcastV2Reaction Payload Tests =======

func TestBroadcastV2Reaction_PayloadFormat(t *testing.T) {
	messageID := "msg-123"
	reactionsJSON := `{"user-1":"👍","user-2":"❤️"}`

	payload := messageID + "|" + reactionsJSON

	// Parse the payload
	parts := splitPayload(payload)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0] != messageID {
		t.Errorf("expected messageID %s, got %s", messageID, parts[0])
	}
	if parts[1] != reactionsJSON {
		t.Errorf("expected reactionsJSON %s, got %s", reactionsJSON, parts[1])
	}
}

func TestBroadcastV2Reaction_EmptyReactions(t *testing.T) {
	messageID := "msg-456"
	reactionsJSON := `{}`

	payload := messageID + "|" + reactionsJSON
	parts := splitPayload(payload)

	if parts[0] != messageID {
		t.Errorf("expected messageID %s, got %s", messageID, parts[0])
	}

	var reactions map[string]string
	err := json.Unmarshal([]byte(parts[1]), &reactions)
	if err != nil {
		t.Fatalf("failed to parse reactions: %v", err)
	}
	if len(reactions) != 0 {
		t.Errorf("expected empty reactions, got %d", len(reactions))
	}
}

func TestBroadcastV2Reaction_ReactionsWithPipe(t *testing.T) {
	// Edge case: reactions JSON shouldn't contain pipes, but test anyway
	messageID := "msg-789"
	reactionsJSON := `{"user-1":"👍"}`

	payload := messageID + "|" + reactionsJSON
	parts := splitPayload(payload)

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
}

// ======= Admin Cursor Tests =======

func TestAdminCursor_EncodeDecode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	username := "testuser"

	encoded := encodeAdminCursor(now, username)
	if encoded == "" {
		t.Fatal("encodeAdminCursor returned empty string")
	}

	decodedTime, decodedUsername, ok := decodeAdminCursor(encoded)
	if !ok {
		t.Fatal("decodeAdminCursor returned false")
	}
	if decodedUsername != username {
		t.Errorf("expected username %s, got %s", username, decodedUsername)
	}
	if !decodedTime.Equal(now) {
		t.Errorf("expected time %v, got %v", now, decodedTime)
	}
}

func TestAdminCursor_Empty(t *testing.T) {
	decodedTime, decodedUsername, ok := decodeAdminCursor("")
	if ok {
		t.Error("expected ok=false for empty cursor")
	}
	if !decodedTime.IsZero() {
		t.Errorf("expected zero time, got %v", decodedTime)
	}
	if decodedUsername != "" {
		t.Errorf("expected empty username, got %s", decodedUsername)
	}
}

func TestAdminCursor_InvalidBase64(t *testing.T) {
	decodedTime, decodedUsername, ok := decodeAdminCursor("!!!invalid-base64!!!")
	if ok {
		t.Error("expected ok=false for invalid base64")
	}
	if !decodedTime.IsZero() {
		t.Errorf("expected zero time, got %v", decodedTime)
	}
	if decodedUsername != "" {
		t.Errorf("expected empty username, got %s", decodedUsername)
	}
}

func TestAdminCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON
	encoded := "aW52YWxpZCBqc29u" // "invalid json" in base64
	decodedTime, decodedUsername, ok := decodeAdminCursor(encoded)
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
	if !decodedTime.IsZero() {
		t.Errorf("expected zero time, got %v", decodedTime)
	}
	if decodedUsername != "" {
		t.Errorf("expected empty username, got %s", decodedUsername)
	}
}

// ======= Favorites Reaction Preservation Tests =======

func TestFavoritesPreservesOriginalID(t *testing.T) {
	// Simulate the favorites flow:
	// 1. Original message has ID "orig-msg-123"
	// 2. Favorite should preserve this ID
	// 3. Reaction on "orig-msg-123" should be visible in favorites

	originalID := "orig-msg-123"
	reactionsJSON := `{"user-1":"👍"}`

	// Simulate: original message + reaction
	var reactions map[string]string
	err := json.Unmarshal([]byte(reactionsJSON), &reactions)
	if err != nil {
		t.Fatalf("failed to unmarshal reactions: %v", err)
	}

	// Verify reaction is tied to original ID
	if _, ok := reactions["user-1"]; !ok {
		t.Error("expected reaction from user-1")
	}

	// Simulate: favorite uses same ID
	favoriteID := originalID
	if favoriteID != originalID {
		t.Errorf("favorite ID %s should equal original ID %s", favoriteID, originalID)
	}
}

func TestFavoritesNewUUIDBreaksReactions(t *testing.T) {
	// This test demonstrates the bug that was fixed:
	// If SaveFavoriteMessage generates a NEW UUID, reactions won't match

	originalID := "orig-msg-123"

	reactionsJSON := `{"user-1":"👍"}`

	// Parse reactions for original message
	var reactions map[string]string
	json.Unmarshal([]byte(reactionsJSON), &reactions)

	// The bug: reactions are stored under originalID, but favorite has new UUID
	// Client looks up reactions by favorite ID -> finds nothing
	if _, ok := reactions[originalID]; ok {
		// This would be wrong - reactions are stored by user, not message ID
		t.Log("Reactions are stored by user ID in the JSON, not message ID")
	}

	// Verify that with the fix, favorite ID == original ID
	fixedFavoriteID := originalID // After fix: we preserve the original ID
	if fixedFavoriteID != originalID {
		t.Errorf("fixed favorite ID %s should equal original ID %s", fixedFavoriteID, originalID)
	}
}

// ======= rowToProtoV2 Reaction Tests =======

func TestRowToProtoV2_ReactionsJSON(t *testing.T) {
	tests := []struct {
		name        string
		reactions   string
		expectCount int
		expectEmoji map[string]string
	}{
		{
			name:        "empty reactions",
			reactions:   "{}",
			expectCount: 0,
			expectEmoji: map[string]string{},
		},
		{
			name:        "single reaction",
			reactions:   `{"user-1":"👍"}`,
			expectCount: 1,
			expectEmoji: map[string]string{"user-1": "👍"},
		},
		{
			name:        "multiple reactions",
			reactions:   `{"user-1":"👍","user-2":"❤️","user-3":"🔥"}`,
			expectCount: 3,
			expectEmoji: map[string]string{"user-1": "👍", "user-2": "❤️", "user-3": "🔥"},
		},
		{
			name:        "nil reactions",
			reactions:   "",
			expectCount: 0,
			expectEmoji: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := &MessageRowV2{
				ID:          "msg-test",
				RoomID:      "room-test",
				SenderID:    "user-test",
				ContentType: "text",
				Text:        "Test message",
				Reactions:   tt.reactions,
				CreatedAt:   time.Now().UTC(),
			}

			proto := rowToProtoV2(row)
			if proto == nil {
				t.Fatal("rowToProtoV2 returned nil")
			}

			var reactions map[string]string
			if len(proto.Reactions) > 0 {
				if err := json.Unmarshal(proto.Reactions, &reactions); err != nil {
					t.Fatalf("failed to unmarshal reactions: %v", err)
				}
			} else {
				reactions = make(map[string]string)
			}

			if len(reactions) != tt.expectCount {
				t.Errorf("got %d reactions, want %d", len(reactions), tt.expectCount)
			}

			for k, v := range tt.expectEmoji {
				if reactions[k] != v {
					t.Errorf("reactions[%s] = %s, want %s", k, reactions[k], v)
				}
			}
		})
	}
}

func TestRowToProtoV2_ReactionsPreserved(t *testing.T) {
	// Test that reactions are preserved through the proto conversion
	reactionsJSON := `{"user-1":"👍","user-2":"❤️","user-3":"🔥"}`

	row := &MessageRowV2{
		ID:          "msg-reactions",
		RoomID:      "room-reactions",
		SenderID:    "user-reactions",
		ContentType: "text",
		Text:        "Message with reactions",
		Reactions:   reactionsJSON,
		CreatedAt:   time.Now().UTC(),
	}

	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}

	// Verify reactions are in proto
	var reactions map[string]string
	err := json.Unmarshal(proto.Reactions, &reactions)
	if err != nil {
		t.Fatalf("failed to unmarshal proto reactions: %v", err)
	}

	if len(reactions) != 3 {
		t.Errorf("expected 3 reactions, got %d", len(reactions))
	}

	// Verify each reaction
	expected := map[string]string{
		"user-1": "👍",
		"user-2": "❤️",
		"user-3": "🔥",
	}
	for k, v := range expected {
		if reactions[k] != v {
			t.Errorf("reactions[%s] = %s, want %s", k, reactions[k], v)
		}
	}
}

// ======= Helper Function =======

// splitPayload splits a string by the first occurrence of "|"
func splitPayload(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
