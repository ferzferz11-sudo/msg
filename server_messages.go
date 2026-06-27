package main

// server_messages.go — v1 RPC handlers (DEPRECATED)
// These handlers are kept for backward compatibility with old clients.
// Internally they read/write messages_v2 and convert to v1 proto format.
// New clients should use GetHistoryV2, SendMessageV2, EditMessageV2, DeleteMessageV2, SetReactionV2.

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetHistory returns messages for a room. DEPRECATED: use GetHistoryV2.
// Reads from messages_v2 and converts to v1 proto format.
func (s *server) GetHistory(_ context.Context, req *gen.GetHistoryRequest) (*gen.GetHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	roomID := req.Room
	if roomID == "" {
		return &gen.GetHistoryResponse{Messages: nil}, nil
	}

	// Read from messages_v2 using cursor-based pagination (latest messages)
	rows, _, err := s.db.GetMessagesV2Cursor(roomID, limit, "")
	if err != nil {
		logger.Errorf("GetHistory(v2): %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get history: %v", err)
	}

	var messages []*gen.Message
	// rows are already in DESC order from cursor, reverse to get ASC for v1 compat
	for i := len(rows) - 1; i >= 0; i-- {
		m := &rows[i]

		// Resolve sender username
		username := m.SenderName
		if username == "" {
			_ = s.db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid`, m.SenderID).Scan(&username)
			if username == "" {
				username = m.SenderID
			}
		}

		// Convert reactions JSONB to v1 format
		var reactions []*gen.Reaction
		if m.Reactions != "" && m.Reactions != "{}" {
			var reactionMap map[string]string
			if json.Unmarshal([]byte(m.Reactions), &reactionMap) == nil {
				for uid, emoji := range reactionMap {
					reactions = append(reactions, &gen.Reaction{User: uid, Emoji: emoji})
				}
			}
		}

		// Parse media URLs
		var imageURLs []string
		if m.MediaURLs != "" && m.MediaURLs != "[]" {
			json.Unmarshal([]byte(m.MediaURLs), &imageURLs)
		}

		// Map content_type to v1 fields
		text := m.Text
		imageURL := ""
		voiceURL := ""
		duration := m.Duration

		switch m.ContentType {
		case "image":
			imageURL = m.MediaURL
			text = ""
		case "voice":
			voiceURL = m.MediaURL
			text = ""
		case "file":
			imageURL = m.MediaURL
			text = ""
		case "deleted":
			text = "[deleted]"
		}

		// Handle reply
		repliedToID := ""
		repliedToUser := ""
		repliedToText := ""
		if m.ReplyToID.Valid {
			repliedToID = m.ReplyToID.String
			repliedToText = m.ReplyPreview.String
		}

		// E2EE
		e2eePayload := ""
		if m.IsE2EE {
			e2eePayload = string(m.E2EEPayload)
		}

		messages = append(messages, &gen.Message{
			Id:                 m.ID,
			User:               username,
			Text:               text,
			CreatedAt:          timestamppb.New(m.CreatedAt),
			Reactions:          reactions,
			RepliedToMessageId: repliedToID,
			RepliedToUser:      repliedToUser,
			RepliedToText:      repliedToText,
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			ImageUrl:           imageURL,
			ImageUrls:          imageURLs,
			Edited:             m.Edited,
			VoiceUrl:           voiceURL,
			Duration:           duration,
			IsE2Ee:             m.IsE2EE,
			E2EePayload:        e2eePayload,
		})
	}

	return &gen.GetHistoryResponse{Messages: messages}, nil
}

// SetReaction sets or removes a reaction. DEPRECATED: use SetReactionV2.
// Uses messages_v2.reactions JSONB internally.
func (s *server) SetReaction(_ context.Context, req *gen.ReactionRequest) (*gen.ReactionResponse, error) {
	if req.MessageId == "" || req.Reaction == nil {
		return &gen.ReactionResponse{Success: false}, nil
	}

	logger.Infof("[Reaction] %s on %s by %s", req.Reaction.Emoji, req.MessageId, req.Reaction.User)

	// Use v2 internally
	reactionsJSON, err := s.db.SetReactionV2(req.MessageId, req.Reaction.User, req.Reaction.Emoji)
	if err != nil {
		logger.Infof("Failed to set reaction: %v", err)
		return &gen.ReactionResponse{Success: false}, err
	}

	// Get message for broadcast
	msg, err := s.db.GetMessageV2ByUUID(req.MessageId)
	if err == nil {
		// Resolve username
		username := msg.SenderName
		if username == "" {
			_ = s.db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid`, msg.SenderID).Scan(&username)
			if username == "" {
				username = msg.SenderID
			}
		}

		// Convert reactions to v1 format
		var reactions []*gen.Reaction
		if reactionsJSON != "" && reactionsJSON != "{}" {
			var reactionMap map[string]string
			if json.Unmarshal([]byte(reactionsJSON), &reactionMap) == nil {
				for uid, emoji := range reactionMap {
					reactions = append(reactions, &gen.Reaction{User: uid, Emoji: emoji})
				}
			}
		}

		// Map content
		text := msg.Text
		imageURL := ""
		voiceURL := ""
		switch msg.ContentType {
		case "image":
			imageURL = msg.MediaURL
		case "voice":
			voiceURL = msg.MediaURL
		}

		s.hub.Broadcast(&gen.Message{
			Id:          msg.ID,
			User:        username,
			Text:        text,
			CreatedAt:   timestamppb.New(msg.CreatedAt),
			Reactions:   reactions,
			RoomId:      msg.RoomID,
			IsRead:      msg.IsRead,
			ImageUrl:    imageURL,
			VoiceUrl:    voiceURL,
			Duration:    msg.Duration,
			Edited:      msg.Edited,
		})
	}

	return &gen.ReactionResponse{Success: true}, nil
}

// DeleteMessages deletes messages. DEPRECATED: use DeleteMessageV2.
// Uses soft delete in messages_v2 internally.
func (s *server) DeleteMessages(_ context.Context, req *gen.DeleteMessagesRequest) (*gen.DeleteMessagesResponse, error) {
	var messageIDs []string

	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}

		// Try to find by ID first
		if msg.Id != "" {
			// Verify the message exists in v2
			v2Msg, err := s.db.GetMessageV2ByUUID(msg.Id)
			if err == nil {
				// Permission check
				canDelete := false
				if v2Msg.SenderID == req.RequesterUsername {
					canDelete = true
				} else if msg.RoomId != "" {
					chat, chatErr := s.db.GetChat(msg.RoomId)
					if chatErr == nil && chat.CreatorUsername == req.RequesterUsername {
						canDelete = true
					}
				}
				if canDelete {
					messageIDs = append(messageIDs, msg.Id)
				}
			}
		}
	}

	if len(messageIDs) == 0 {
		return &gen.DeleteMessagesResponse{Success: false}, nil
	}

	// Soft delete via v2
	if err := s.db.DeleteMessageV2(messageIDs); err != nil {
		return &gen.DeleteMessagesResponse{Success: false}, nil
	}

	// Broadcast deletions
	for _, id := range messageIDs {
		v2Msg, err := s.db.GetMessageV2ByUUID(id)
		if err == nil {
			_ = s.db.IncrementParticipantsChatListVersion(v2Msg.RoomID)
			s.hub.Broadcast(&gen.Message{
				User:   "SYSTEM",
				Text:   "DELETE_MESSAGE:" + id,
				RoomId: v2Msg.RoomID,
			})
		}
	}

	return &gen.DeleteMessagesResponse{Success: true}, nil
}

// EditMessage edits a message. DEPRECATED: use EditMessageV2.
// Uses messages_v2.text internally.
func (s *server) EditMessage(_ context.Context, req *gen.EditMessageRequest) (*gen.EditMessageResponse, error) {
	if req.MessageId == "" {
		return &gen.EditMessageResponse{Success: false, Message: "Message ID is required"}, nil
	}

	// Use v2 internally
	if err := s.db.EditMessageV2(req.MessageId, req.Text); err != nil {
		logger.Infof("Failed to edit message %s: %v", req.MessageId, err)
		return &gen.EditMessageResponse{Success: false, Message: err.Error()}, nil
	}

	// Get updated message for broadcast
	msg, err := s.db.GetMessageV2ByUUID(req.MessageId)
	if err == nil {
		_ = s.db.IncrementParticipantsChatListVersion(msg.RoomID)

		// Resolve username
		username := msg.SenderName
		if username == "" {
			_ = s.db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid`, msg.SenderID).Scan(&username)
			if username == "" {
				username = msg.SenderID
			}
		}

		// Convert reactions
		var reactions []*gen.Reaction
		if msg.Reactions != "" && msg.Reactions != "{}" {
			var reactionMap map[string]string
			if json.Unmarshal([]byte(msg.Reactions), &reactionMap) == nil {
				for uid, emoji := range reactionMap {
					reactions = append(reactions, &gen.Reaction{User: uid, Emoji: emoji})
				}
			}
		}

		// Map content
		text := msg.Text
		imageURL := ""
		voiceURL := ""
		switch msg.ContentType {
		case "image":
			imageURL = msg.MediaURL
		case "voice":
			voiceURL = msg.MediaURL
		}

		// Handle reply
		repliedToID := ""
		repliedToText := ""
		if msg.ReplyToID.Valid {
			repliedToID = msg.ReplyToID.String
			repliedToText = msg.ReplyPreview.String
		}

		s.hub.Broadcast(&gen.Message{
			Id:                 msg.ID,
			User:               username,
			Text:               text,
			CreatedAt:          timestamppb.New(msg.CreatedAt),
			Reactions:          reactions,
			RepliedToMessageId: repliedToID,
			RepliedToText:      repliedToText,
			RoomId:             msg.RoomID,
			IsRead:             msg.IsRead,
			ImageUrl:           imageURL,
			Edited:             true,
			VoiceUrl:           voiceURL,
			Duration:           msg.Duration,
		})
	}

	logger.Infof("Edited message %s", req.MessageId)
	return &gen.EditMessageResponse{Success: true, Message: "Message edited successfully"}, nil
}

// trimString trims a string to maxLen characters.
func trimString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen]) + "..."
}
