package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetHistoryV2 returns messages for a room with cursor-based pagination.
func (s *server) GetHistoryV2(_ context.Context, req *gen.GetHistoryV2Request) (*gen.GetHistoryV2Response, error) {
	if req.RoomId == "" {
		return &gen.GetHistoryV2Response{}, nil
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	rows, nextCursor, err := s.db.GetMessagesV2Cursor(req.RoomId, limit, req.Cursor)
	if err != nil {
		logger.Errorf("GetHistoryV2: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get history: %v", err)
	}

	var messages []*gen.MessageV2
	for _, r := range rows {
		m := rowToProtoV2(&r)
		if m != nil {
			messages = append(messages, m)
		}
	}

	return &gen.GetHistoryV2Response{
		Messages:   messages,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// SendMessageV2 creates a new message and broadcasts it.
func (s *server) SendMessageV2(ctx context.Context, req *gen.SendMessageV2Request) (*gen.SendMessageV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if req.RoomId == "" {
		return &gen.SendMessageV2Response{Success: false, Error: "room_id required"}, nil
	}

	msgID := uuid.New().String()
	now := time.Now().UTC()

	row := &MessageRowV2{
		ID:          msgID,
		RoomID:      req.RoomId,
		SenderID:    userID,
		ContentType: "text",
		IsRead:      false,
		CreatedAt:   now,
	}

	switch c := req.Content.(type) {
	case *gen.SendMessageV2Request_Text:
		row.Text = c.Text
		row.ContentType = "text"
	case *gen.SendMessageV2Request_Media:
		row.MediaURL = c.Media.Url
		if len(c.Media.Urls) > 0 {
			b, _ := json.Marshal(c.Media.Urls)
			row.MediaURLs = string(b)
		}
		row.Duration = c.Media.Duration
		row.ContentType = c.Media.Type
	default:
		row.ContentType = "text"
	}

	if req.IsE2Ee {
		row.IsE2EE = true
		row.E2EEPayload = []byte(req.E2EePayload)
	}

	if req.ReplyToId != "" {
		row.ReplyToID = sql.NullString{String: req.ReplyToId, Valid: true}
		orig, err := s.db.GetMessageV2ByUUID(req.ReplyToId)
		if err == nil {
			preview := orig.Text
			if len(preview) > 100 {
				preview = preview[:100]
			}
			if orig.ContentType == "image" {
				preview = "[image]"
			} else if orig.ContentType == "voice" {
				preview = "[voice]"
			}
			row.ReplyPreview = sql.NullString{String: preview, Valid: true}
		}
	}

	if len(req.Mentions) > 0 {
		b, _ := json.Marshal(req.Mentions)
		row.Mentions = sql.NullString{String: string(b), Valid: true}
	}

	if err := s.db.SaveMessageV2(row); err != nil {
		logger.Errorf("SendMessageV2: %v", err)
		return &gen.SendMessageV2Response{Success: false, Error: err.Error()}, nil
	}

	// Update last_seen_at when user sends a message via unary RPC
	if username := GetUsername(ctx); username != "" {
		_ = s.db.UpdateLastSeen(username)
	}

	// Update chat last message
	preview := row.Text
	if len(preview) > 500 {
		preview = preview[:500]
	}
	if row.ContentType == "image" {
		preview = "Image"
	} else if row.ContentType == "voice" {
		preview = "Voice message"
	}
	_, _ = s.db.Exec(`UPDATE chats SET last_message_text=$1, last_message_time=$2, last_message_username=(SELECT username FROM users WHERE id=$3::uuid), last_message_has_image=$4 WHERE id=$5`,
		preview, now, userID, row.ContentType == "image", req.RoomId)

	// Increment chat list version
	_ = s.db.IncrementParticipantsChatListVersion(req.RoomId)

	// Broadcast to room via system message (client fetches via GetHistoryV2)
	s.hub.Broadcast(&gen.Message{
		User:   "SYSTEM",
		Text:   "NEW_MESSAGE_V2:" + msgID,
		RoomId: req.RoomId,
	})

	// Broadcast actual message via ChatV2 stream (real-time delivery)
	v2Msg := rowToProtoV2(row)
	wrappedMsg := &gen.ChatV2Message{
		Payload: &gen.ChatV2Message_Message{Message: v2Msg},
	}
	for _, target := range s.hub.SnapshotRoomStreams(req.RoomId) {
		_ = target.Send(wrappedMsg)
	}

	return &gen.SendMessageV2Response{
		Message: rowToProtoV2(row),
		Success: true,
	}, nil
}

// EditMessageV2 edits a message text.
func (s *server) EditMessageV2(ctx context.Context, req *gen.EditMessageV2Request) (*gen.EditMessageV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if req.MessageId == "" {
		return &gen.EditMessageV2Response{Success: false, Message: "message_id required"}, nil
	}

	// Verify ownership
	msg, err := s.db.GetMessageV2ByUUID(req.MessageId)
	if err != nil {
		return &gen.EditMessageV2Response{Success: false, Message: "message not found"}, nil
	}
	if msg.SenderID != userID {
		return &gen.EditMessageV2Response{Success: false, Message: "not authorized"}, nil
	}

	if err := s.db.EditMessageV2(req.MessageId, req.Text); err != nil {
		logger.Errorf("EditMessageV2: %v", err)
		return &gen.EditMessageV2Response{Success: false, Message: err.Error()}, nil
	}

	if username := GetUsername(ctx); username != "" {
		_ = s.db.UpdateLastSeen(username)
	}

	// Broadcast edit notification
	updated, err := s.db.GetMessageV2ByUUID(req.MessageId)
	if err == nil {
		s.hub.Broadcast(&gen.Message{
			User:   "SYSTEM",
			Text:   "EDIT_MESSAGE_V2:" + req.MessageId,
			RoomId: updated.RoomID,
		})

		// Update last message in chat if this was the last message
		var lastMsgID string
		_ = s.db.QueryRow(`SELECT id FROM messages_v2 WHERE room_id = $1 ORDER BY created_at DESC LIMIT 1`, updated.RoomID).Scan(&lastMsgID)
		if lastMsgID == req.MessageId {
			s.db.UpdateChatLastMessage(updated.RoomID)
		}
	}

	return &gen.EditMessageV2Response{Success: true, Message: "edited"}, nil
}

// DeleteMessageV2 permanently deletes messages from the database.
func (s *server) DeleteMessageV2(ctx context.Context, req *gen.DeleteMessageV2Request) (*gen.DeleteMessageV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if len(req.MessageIds) == 0 {
		return &gen.DeleteMessageV2Response{Success: false}, nil
	}

	// Verify ownership for each message
	var canDelete []string
	for _, id := range req.MessageIds {
		msg, err := s.db.GetMessageV2ByUUID(id)
		if err != nil {
			continue
		}
		// Owner or admin can delete
		if msg.SenderID == userID {
			canDelete = append(canDelete, id)
		} else {
			// Check if user is admin
			var isAdmin bool
			_ = s.db.QueryRow(`SELECT is_super_admin FROM users WHERE id = $1::uuid`, userID).Scan(&isAdmin)
			if isAdmin {
				canDelete = append(canDelete, id)
			}
		}
	}

	if len(canDelete) == 0 {
		return &gen.DeleteMessageV2Response{Success: false}, nil
	}

	// Collect room IDs before deletion
	updatedRooms := make(map[string]bool)
	for _, id := range canDelete {
		msg, err := s.db.GetMessageV2ByUUID(id)
		if err == nil {
			updatedRooms[msg.RoomID] = true
		}
	}

	if err := s.db.DeleteMessageV2(canDelete); err != nil {
		return &gen.DeleteMessageV2Response{Success: false}, nil
	}

	if username := GetUsername(ctx); username != "" {
		_ = s.db.UpdateLastSeen(username)
	}

	// Update last message in chat after deletion
	for roomID := range updatedRooms {
		s.db.UpdateChatLastMessage(roomID)
	}

	// Broadcast delete notifications
	for _, id := range canDelete {
		msg, err := s.db.GetMessageV2ByUUID(id)
		if err == nil {
			_ = s.db.IncrementParticipantsChatListVersion(msg.RoomID)
			s.hub.Broadcast(&gen.Message{
				User:   "SYSTEM",
				Text:   "DELETE_MESSAGE_V2:" + id,
				RoomId: msg.RoomID,
			})
		}
	}

	return &gen.DeleteMessageV2Response{Success: true}, nil
}

// SetReactionV2 sets or removes a reaction.
func (s *server) SetReactionV2(ctx context.Context, req *gen.SetReactionV2Request) (*gen.SetReactionV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if req.MessageId == "" {
		return &gen.SetReactionV2Response{Success: false}, nil
	}

	reactionsJSON, err := s.db.SetReactionV2(req.MessageId, userID, req.Emoji)
	if err != nil {
		log.Printf("[SetReactionV2] FAILED: messageId=%s userID=%s emoji=%q err=%v", req.MessageId, userID, req.Emoji, err)
		return &gen.SetReactionV2Response{Success: false}, nil
	}

	if username := GetUsername(ctx); username != "" {
		_ = s.db.UpdateLastSeen(username)
	}

	// Broadcast reaction update via system message
	msg, err := s.db.GetMessageV2ByUUID(req.MessageId)
	if err == nil {
		s.hub.Broadcast(&gen.Message{
			User:   "SYSTEM",
			Text:   "REACTION_V2:" + req.MessageId,
			RoomId: msg.RoomID,
		})
		s.hub.BroadcastV2Reaction(msg.RoomID, req.MessageId, reactionsJSON)
	}

	return &gen.SetReactionV2Response{
		Success:   true,
		Reactions: []byte(reactionsJSON),
	}, nil
}

// SearchMessages searches messages in a chat or across all user's chats.
func (s *server) SearchMessages(ctx context.Context, req *gen.SearchMessagesRequest) (*gen.SearchMessagesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, status.Errorf(codes.InvalidArgument, "query is required")
	}

	roomID := req.GetRoomId()
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	results, err := s.db.SearchMessages(userID, roomID, query, limit)
	if err != nil {
		logger.Errorf("SearchMessages: %v", err)
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	var protoResults []*gen.SearchResult
	for _, r := range results {
		protoResults = append(protoResults, &gen.SearchResult{
			MessageId: r.MessageID,
			RoomId:    r.RoomID,
			Username:  r.Username,
			Preview:   r.Preview,
			CreatedAt: r.CreatedAt,
		})
	}

	return &gen.SearchMessagesResponse{Messages: protoResults}, nil
}

// rowToProtoV2 converts a DB row to proto MessageV2.
func rowToProtoV2(r *MessageRowV2) *gen.MessageV2 {
	if r == nil {
		return nil
	}

	m := &gen.MessageV2{
		Id:       r.ID,
		RoomId:   r.RoomID,
		SenderId: r.SenderID,
		Edited:   r.Edited,
		IsRead:   r.IsRead,
		CreatedAt: timestamppb.New(r.CreatedAt),
		IsE2Ee:   r.IsE2EE,
	}

	if r.IsE2EE {
		m.E2EePayload = base64Encode(string(r.E2EEPayload))
	}

	switch r.ContentType {
	case "text":
		m.Content = &gen.MessageV2_Text{Text: r.Text}
	case "image", "voice", "file":
		var urls []string
		if r.MediaURLs != "" && r.MediaURLs != "[]" {
			json.Unmarshal([]byte(r.MediaURLs), &urls)
		}
		m.Content = &gen.MessageV2_Media{
			Media: &gen.MessageMedia{
				Type:     r.ContentType,
				Url:      r.MediaURL,
				Urls:     urls,
				Duration: r.Duration,
			},
		}
	case "deleted":
		m.Content = &gen.MessageV2_Text{Text: "[deleted]"}
	}

	if r.ReplyToID.Valid {
		m.Reply = &gen.MessageReply{
			MessageId: r.ReplyToID.String,
			Preview:   r.ReplyPreview.String,
		}
	}

	if r.Reactions != "" && r.Reactions != "{}" {
		m.Reactions = []byte(r.Reactions)
	}

	if r.Mentions.Valid && r.Mentions.String != "" && r.Mentions.String != "[]" {
		var mentions []string
		if err := json.Unmarshal([]byte(r.Mentions.String), &mentions); err == nil {
			m.Mentions = mentions
		}
	}

	return m
}
