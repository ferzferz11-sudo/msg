package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ======= ChatList v2: PinChat / UnPinChat =======

func (s *server) PinChat(ctx context.Context, req *gen.PinChatRequest) (*gen.PinChatResponse, error) {
	userID := GetUserID(ctx)
	chatID := req.GetChatId()

	if userID == "" || chatID == "" {
		return &gen.PinChatResponse{Success: false}, fmt.Errorf("user_id and chat_id are required")
	}

	// Verify user is participant of the chat
	if !s.isChatParticipant(userID, chatID) {
		return &gen.PinChatResponse{Success: false}, fmt.Errorf("user is not a participant of this chat")
	}

	err := s.db.PinChat(userID, chatID)
	if err != nil {
		logger.Errorf("Failed to pin chat %s for user %s: %v", chatID, userID, err)
		return &gen.PinChatResponse{Success: false}, err
	}

	logger.Infof("Chat %s pinned by user %s", chatID, userID)
	return &gen.PinChatResponse{Success: true}, nil
}

func (s *server) UnPinChat(ctx context.Context, req *gen.UnPinChatRequest) (*gen.UnPinChatResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()

	if userID == "" || chatID == "" {
		return &gen.UnPinChatResponse{Success: false}, fmt.Errorf("user_id and chat_id are required")
	}

	err := s.db.UnPinChat(userID, chatID)
	if err != nil {
		logger.Errorf("Failed to unpin chat %s for user %s: %v", chatID, userID, err)
		return &gen.UnPinChatResponse{Success: false}, err
	}

	logger.Infof("Chat %s unpinned by user %s", chatID, userID)
	return &gen.UnPinChatResponse{Success: true}, nil
}

// ======= ChatList v2: SearchChats =======

func (s *server) SearchChats(ctx context.Context, req *gen.SearchChatsRequest) (*gen.SearchChatsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	query := strings.TrimSpace(req.GetQuery())

	if userID == "" {
		return &gen.SearchChatsResponse{}, fmt.Errorf("user_id is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	chats, err := s.db.SearchChats(userID, query, limit, int(req.GetOffset()))
	if err != nil {
		logger.Errorf("Failed to search chats for user %s: %v", userID, err)
		return &gen.SearchChatsResponse{}, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, chatV2RowToProto(c))
	}

	return &gen.SearchChatsResponse{Chats: chatInfos}, nil
}

// ======= ChatList v2: ArchiveChat / UnarchiveChat =======

func (s *server) ArchiveChat(ctx context.Context, req *gen.ArchiveChatRequest) (*gen.ArchiveChatResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()

	if userID == "" || chatID == "" {
		return &gen.ArchiveChatResponse{Success: false}, fmt.Errorf("user_id and chat_id are required")
	}

	if !s.isChatParticipant(userID, chatID) {
		return &gen.ArchiveChatResponse{Success: false}, fmt.Errorf("user is not a participant of this chat")
	}

	err := s.db.ArchiveChat(userID, chatID)
	if err != nil {
		logger.Errorf("Failed to archive chat %s for user %s: %v", chatID, userID, err)
		return &gen.ArchiveChatResponse{Success: false}, err
	}

	logger.Infof("Chat %s archived by user %s", chatID, userID)
	return &gen.ArchiveChatResponse{Success: true}, nil
}

func (s *server) UnarchiveChat(ctx context.Context, req *gen.UnarchiveChatRequest) (*gen.UnarchiveChatResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()

	if userID == "" || chatID == "" {
		return &gen.UnarchiveChatResponse{Success: false}, fmt.Errorf("user_id and chat_id are required")
	}

	err := s.db.UnarchiveChat(userID, chatID)
	if err != nil {
		logger.Errorf("Failed to unarchive chat %s for user %s: %v", chatID, userID, err)
		return &gen.UnarchiveChatResponse{Success: false}, err
	}

	logger.Infof("Chat %s unarchived by user %s", chatID, userID)
	return &gen.UnarchiveChatResponse{Success: true}, nil
}

// ======= ChatList v2: GetChats with pagination and filters =======

func (s *server) GetChatsV2(ctx context.Context, req *gen.GetChatsRequest) (*gen.GetChatsResponse, error) {
	userID := GetUserID(ctx)
	username := req.GetUsername()
	deviceID := GetDeviceID(ctx)
	clientVer := s.hub.GetClientVersion(userID)
	logger.Debugf("GetChatsV2: user=%s v=%s device=%s", userID, clientVer, deviceID)

	// Resolve user ID from username if needed (v1 fallback)
	if userID == "" && username != "" {
		id, err := s.db.GetUserIdByUsername(username)
		if err == nil && id != "" {
			userID = id
		}
	}

	// Also try username from context (v1 interceptor fallback)
	if userID == "" {
		if ctxUsername := GetUsername(ctx); ctxUsername != "" {
			id, err := s.db.GetUserIdByUsername(ctxUsername)
			if err == nil && id != "" {
				userID = id
				username = ctxUsername
			}
		}
	}

	if userID == "" {
		return &gen.GetChatsResponse{}, nil
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100 // default
	}
	if limit > 500 {
		limit = 500
	}

	filter := req.GetFilter()
	cursor := req.GetCursor()

	// Use cursor-based pagination if cursor is provided, else legacy offset
	var result *ChatV2Result
	var err error
	if cursor != "" {
		result, err = s.db.GetUserChatsV2Cursor(userID, username, limit, cursor, filter)
	} else {
		// Legacy offset path for backward compatibility
		chats, dbErr := s.db.GetUserChatsV2(userID, username, limit, int(req.GetOffset()), filter)
		if dbErr != nil {
			logger.Errorf("Error fetching chats v2 for user %s: %v", userID, dbErr)
			return &gen.GetChatsResponse{}, dbErr
		}
		result = &ChatV2Result{Chats: chats}
	}

	if err != nil {
		logger.Errorf("Error fetching chats v2 for user %s: %v", userID, err)
		return &gen.GetChatsResponse{}, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range result.Chats {
		chatInfos = append(chatInfos, chatV2RowToProto(c))
	}

	return &gen.GetChatsResponse{
		Chats:      chatInfos,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// ======= Helpers =======

func (s *server) isChatParticipant(userID, chatID string) bool {
	chat, err := s.db.GetChat(chatID)
	if err != nil {
		return false
	}
	// Parse JSON array properly to avoid false positives
	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		return false
	}
	for _, p := range participants {
		if p == userID {
			return true
		}
	}
	return false
}

// chatV2RowToProto converts a ChatV2Row to proto ChatInfo with v2 fields
func chatV2RowToProto(c ChatV2Row) *gen.ChatInfo {
	return &gen.ChatInfo{
		Id:                  c.ID,
		Name:                c.Name,
		Type:                c.Type,
		Participants:        c.Participants,
		CreatedAt:           timestamppb.New(c.CreatedAt),
		UnreadCount:         int32(c.UnreadCount),
		LastMessageTime:     timestamppb.New(c.LastMessageTime),
		Creator:             c.Creator,
		LastMessageText:     c.LastMessageText,
		AvatarUrl:           c.AvatarURL,
		FullAvatarUrl:       c.FullAvatarURL,
		LastMessageUsername: c.LastMessageUsername,
		LastMessageHasImage: c.LastMessageHasImage,
		AllowMembersToAdd:   c.AllowMembersToAdd,
		IsPinned:            c.IsPinned,
		IsMuted:             c.IsMuted,
		IsArchived:          c.IsArchived,
		PinnedAt:            c.PinnedAt,
	}
}

// ======= Pin Message: PinMessage / UnPinMessage / GetPinnedMessages =======

func (s *server) PinMessage(ctx context.Context, req *gen.PinMessageRequest) (*gen.PinMessageResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()
	messageID := req.GetMessageId()

	if userID == "" || chatID == "" || messageID == "" {
		return &gen.PinMessageResponse{Success: false}, fmt.Errorf("user_id, chat_id, and message_id are required")
	}

	// Verify user is participant of the chat
	if !s.isChatParticipant(userID, chatID) {
		return &gen.PinMessageResponse{Success: false}, fmt.Errorf("user is not a participant of this chat")
	}

	err := s.db.PinMessage(userID, chatID, messageID)
	if err != nil {
		logger.Errorf("Failed to pin message %s in chat %s for user %s: %v", messageID, chatID, userID, err)
		return &gen.PinMessageResponse{Success: false}, err
	}

	logger.Infof("Message %s pinned in chat %s by user %s", messageID, chatID, userID)
	return &gen.PinMessageResponse{Success: true}, nil
}

func (s *server) UnPinMessage(ctx context.Context, req *gen.UnPinMessageRequest) (*gen.UnPinMessageResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()
	messageID := req.GetMessageId()

	if userID == "" || chatID == "" || messageID == "" {
		return &gen.UnPinMessageResponse{Success: false}, fmt.Errorf("user_id, chat_id, and message_id are required")
	}

	err := s.db.UnPinMessage(userID, chatID, messageID)
	if err != nil {
		logger.Errorf("Failed to unpin message %s in chat %s for user %s: %v", messageID, chatID, userID, err)
		return &gen.UnPinMessageResponse{Success: false}, err
	}

	logger.Infof("Message %s unpinned in chat %s by user %s", messageID, chatID, userID)
	return &gen.UnPinMessageResponse{Success: true}, nil
}

func (s *server) GetPinnedMessages(ctx context.Context, req *gen.GetPinnedMessagesRequest) (*gen.GetPinnedMessagesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	chatID := req.GetChatId()

	if userID == "" || chatID == "" {
		return &gen.GetPinnedMessagesResponse{}, fmt.Errorf("user_id and chat_id are required")
	}

	pinnedRows, err := s.db.GetPinnedMessages(userID, chatID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		logger.Errorf("Failed to get pinned messages for user %s in chat %s: %v", userID, chatID, err)
		return &gen.GetPinnedMessagesResponse{}, err
	}

	var messages []*gen.Message
	for _, r := range pinnedRows {
		messages = append(messages, &gen.Message{
			Id:        r.MessageID,
			User:      r.User,
			Text:      r.Text,
			CreatedAt: timestamppb.New(r.CreatedAt),
		})
	}

	return &gen.GetPinnedMessagesResponse{Messages: messages}, nil
}
