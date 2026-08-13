package main

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"LavenderMessenger/gen"
)

func TestAllowedTimerValues(t *testing.T) {
	valid := []int32{0, 30, 60, 300, 3600, 86400}
	for _, v := range valid {
		if !allowedTimerValues[v] {
			t.Errorf("expected %d to be allowed", v)
		}
	}

	invalid := []int32{1, 15, 120, 7200, -1, 99999}
	for _, v := range invalid {
		if allowedTimerValues[v] {
			t.Errorf("expected %d to be rejected", v)
		}
	}
}

func TestChatV2RowToProto_SelfDestructTimer(t *testing.T) {
	c := ChatV2Row{
		ID:                "chat-1",
		Name:              "Test Chat",
		Type:              "group",
		SelfDestructTimer: 3600,
	}

	proto := chatV2RowToProto(c)
	if proto.SelfDestructTimer != 3600 {
		t.Errorf("expected self_destruct_timer 3600, got %d", proto.SelfDestructTimer)
	}
}

func TestChatV2RowToProto_SelfDestructTimerZero(t *testing.T) {
	c := ChatV2Row{
		ID:   "chat-2",
		Name: "Normal Chat",
		Type: "direct",
	}

	proto := chatV2RowToProto(c)
	if proto.SelfDestructTimer != 0 {
		t.Errorf("expected self_destruct_timer 0, got %d", proto.SelfDestructTimer)
	}
}

func TestRowToProtoV2_ForwardedFrom(t *testing.T) {
	row := &MessageRowV2{
		ID:            "msg-fwd",
		RoomID:        "room-1",
		SenderID:      "user-1",
		ContentType:   "text",
		Text:          "Forwarded message",
		ForwardedFrom: "original_sender",
	}

	proto := rowToProtoV2(row)
	if proto.ForwardedFrom != "original_sender" {
		t.Errorf("expected forwarded_from 'original_sender', got %q", proto.ForwardedFrom)
	}
}

func TestRowToProtoV2_Mentions(t *testing.T) {
	row := &MessageRowV2{
		ID:          "msg-mention",
		RoomID:      "room-1",
		SenderID:    "user-1",
		ContentType: "text",
		Text:        "Hello @user2",
		Mentions:    sql.NullString{String: `["user2","user3"]`, Valid: true},
	}

	proto := rowToProtoV2(row)
	if len(proto.Mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %d", len(proto.Mentions))
	}
	if proto.Mentions[0] != "user2" || proto.Mentions[1] != "user3" {
		t.Errorf("expected mentions [user2, user3], got %v", proto.Mentions)
	}
}

func TestSetSelfDestructTimerResponse_Proto(t *testing.T) {
	resp := &gen.SetSelfDestructTimerResponse{
		Success: true,
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}

	resp2 := &gen.SetSelfDestructTimerResponse{
		Success: false,
		Error:   "invalid timer value",
	}
	if resp2.Success {
		t.Error("expected success to be false")
	}
	if resp2.Error != "invalid timer value" {
		t.Errorf("expected error 'invalid timer value', got %q", resp2.Error)
	}
}

func TestTimerChangeMessage(t *testing.T) {
	tests := []struct {
		seconds int32
		wantRu  string
		wantEn  string
	}{
		{0, "Авто-удаление отключено", "Auto-delete disabled"},
		{30, "Авто-удаление: 30 сек", "Auto-delete: 30 sec"},
		{60, "Авто-удаление: 1 мин", "Auto-delete: 1 min"},
		{300, "Авто-удаление: 5 мин", "Auto-delete: 5 min"},
		{3600, "Авто-удаление: 1 час", "Auto-delete: 1 hour"},
		{86400, "Авто-удаление: 24 часа", "Auto-delete: 24 hours"},
		{120, "Авто-удаление: 120 сек", "Auto-delete: 120 sec"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("timer_%d", tt.seconds), func(t *testing.T) {
			got := timerChangeMessage(tt.seconds)
			parts := strings.SplitN(got, "\n", 2)
			if len(parts) != 2 {
				t.Fatalf("expected 2 lines, got %q", got)
			}
			if parts[0] != tt.wantRu {
				t.Errorf("ru = %q, want %q", parts[0], tt.wantRu)
			}
			if parts[1] != tt.wantEn {
				t.Errorf("en = %q, want %q", parts[1], tt.wantEn)
			}
		})
	}
}

func TestRowToProtoV2_SystemMessage(t *testing.T) {
	row := &MessageRowV2{
		ID:          "msg-system",
		RoomID:      "room-1",
		SenderID:    "00000000-0000-0000-0000-000000000000",
		ContentType: "system",
		Text:        "Авто-удаление: 5 мин\nAuto-delete: 5 min",
		CreatedAt:   time.Now().UTC(),
	}
	proto := rowToProtoV2(row)
	if proto == nil {
		t.Fatal("rowToProtoV2 returned nil")
	}
	if proto.SenderId != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected system sender_id, got %q", proto.SenderId)
	}
}
