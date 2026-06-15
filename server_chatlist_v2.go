package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ======= ChatList v2: PinChat / UnPinChat =======

func (s *server) PinChat(ctx context.Context, req *gen.PinChatRequest) (*gen.PinChatResponse, error) {
	userID := req.GetUserId()
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
	userID := req.GetUserId()
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
	userID := req.GetUserId()
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
	userID := req.GetUserId()
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
	userID := req.GetUserId()
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
	userID := req.GetUserId()
	username := req.GetUsername()

	// Resolve user ID from username if needed
	if userID == "" && username != "" {
		id, err := s.db.GetUserIdByUsername(username)
		if err == nil && id != "" {
			userID = id
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

	chats, err := s.db.GetUserChatsV2(userID, username, limit, int(req.GetOffset()), filter)
	if err != nil {
		logger.Errorf("Error fetching chats v2 for user %s: %v", userID, err)
		return &gen.GetChatsResponse{}, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, chatV2RowToProto(c))
	}

	return &gen.GetChatsResponse{Chats: chatInfos}, nil
}

// ======= Helpers =======

func (s *server) isChatParticipant(userID, chatID string) bool {
	chat, err := s.db.GetChat(chatID)
	if err != nil {
		return false
	}
	// Check participants JSON array for userID
	return strings.Contains(chat.Participants, userID)
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
