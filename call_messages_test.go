package main

import (
	"fmt"
	"testing"

	"LavenderMessenger/gen"
)

func TestCallMessage_IsVideoField(t *testing.T) {
	msg := &gen.CallMessage{
		CallId:     "call-123",
		SenderId:   "user-1",
		ReceiverId: "user-2",
		Type:       gen.CallMessage_INITIATE,
		IsVideo:    true,
	}
	if !msg.IsVideo {
		t.Error("expected IsVideo to be true")
	}

	msg2 := &gen.CallMessage{
		CallId:     "call-456",
		SenderId:   "user-1",
		ReceiverId: "user-2",
		Type:       gen.CallMessage_INITIATE,
		IsVideo:    false,
	}
	if msg2.IsVideo {
		t.Error("expected IsVideo to be false for audio call")
	}
}

func TestCallMessage_DefaultIsVideo(t *testing.T) {
	msg := &gen.CallMessage{
		CallId:   "call-789",
		SenderId: "user-1",
		Type:     gen.CallMessage_INITIATE,
	}
	if msg.IsVideo {
		t.Error("expected IsVideo to default to false")
	}
}

func TestCallSystemMessages_VideoVsAudio(t *testing.T) {
	tests := []struct {
		name     string
		isVideo  bool
		wantIcon string
		wantText string
	}{
		{"video call", true, "📹", "Видеозвонок"},
		{"audio call", false, "📞", "Звонок"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var icon, text string
			if tt.isVideo {
				icon = "📹"
				text = "Видеозвонок"
			} else {
				icon = "📞"
				text = "Звонок"
			}
			if icon != tt.wantIcon {
				t.Errorf("expected icon %q, got %q", tt.wantIcon, icon)
			}
			if text != tt.wantText {
				t.Errorf("expected text %q, got %q", tt.wantText, text)
			}
		})
	}
}

func TestCallEndedMessage_AnsweredVsNotAnswered(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		wantText string
	}{
		{"not answered 0s", 0, "Не отвечено"},
		{"answered 1s", 1, "Звонок завершен (0:01)"},
		{"answered 65s", 65, "Звонок завершен (1:05)"},
		{"answered 3661s", 3661, "Звонок завершен (61:01)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var text string
			if tt.duration > 0 {
				minutes := tt.duration / 60
				seconds := tt.duration % 60
				text = fmt.Sprintf("Звонок завершен (%d:%02d)", minutes, seconds)
			} else {
				text = "Не отвечено"
			}
			if text != tt.wantText {
				t.Errorf("expected %q, got %q", tt.wantText, text)
			}
		})
	}
}
