package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *server) GetAllChats(ctx context.Context, req *gen.GetAllChatsRequest) (*gen.GetAllChatsResponse, error) {
	chats, err := s.db.GetAllChats()
	if err != nil {
		logger.Errorf("Error fetching all chats: %v", err)
		return nil, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, &gen.ChatInfo{
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
			IsPinned:            false,
			IsMuted:             false,
			IsArchived:          false,
			PinnedAt:            0,
		})
	}

	return &gen.GetAllChatsResponse{
		Chats: chatInfos,
	}, nil
}

func (s *server) CreateDirectChat(_ context.Context, req *gen.CreateDirectChatRequest) (*gen.CreateDirectChatResponse, error) {
	u1 := req.User1
	if req.User1Id != "" {
		resolved := resolveDisplayName(s.db, req.User1Id)
		if resolved != "" {
			u1 = resolved
		}
	}
	u2 := req.User2
	if req.User2Id != "" {
		resolved := resolveDisplayName(s.db, req.User2Id)
		if resolved != "" {
			u2 = resolved
		}
	}

	// Validate both users exist
	exists1, err1 := s.db.UserExists(u1)
	exists2, err2 := s.db.UserExists(u2)
	if err1 != nil || err2 != nil || !exists1 || !exists2 {
		logger.Errorf("CreateDirectChat: user not found (u1=%s exists=%v err=%v, u2=%s exists=%v err=%v)", u1, exists1, err1, u2, exists2, err2)
		return &gen.CreateDirectChatResponse{Success: false}, fmt.Errorf("user not found")
	}

	logger.Infof("CreateDirectChat: %s <-> %s", u1, u2)
	chatID, err := s.db.GetDirectChatBetweenUsers(u1, u2)
	if err != nil {
		logger.Errorf("Error creating direct chat: %v", err)
		return &gen.CreateDirectChatResponse{Success: false}, err
	}

	logger.Infof("Direct chat created/found: %s", chatID)
	return &gen.CreateDirectChatResponse{ChatId: chatID, Success: true}, nil
}

func (s *server) CreateGroupChat(_ context.Context, req *gen.CreateGroupChatRequest) (*gen.CreateGroupChatResponse, error) {
	creator := req.Creator
	if req.CreatorId != "" {
		resolved := resolveDisplayName(s.db, req.CreatorId)
		if resolved != "" {
			creator = resolved
		}
	}

	logger.Infof("CreateGroupChat: %s (Creator: %s)", req.Name, creator)
	chatID := uuid.New().String()

	// Convert participants slice to JSON string safely
	participantsJSON, err := json.Marshal(req.Participants)
	if err != nil {
		return &gen.CreateGroupChatResponse{Success: false}, fmt.Errorf("failed to encode participants: %w", err)
	}

	err = s.db.CreateChat(chatID, req.Name, "group", string(participantsJSON), creator, req.CreatorId)
	if err != nil {
		logger.Infof("Failed to create group chat in DB: %v", err)
		return &gen.CreateGroupChatResponse{Success: false}, err
	}
	logger.Infof("Group chat created: %s (%s)", chatID, req.Name)
	return &gen.CreateGroupChatResponse{ChatId: chatID, Success: true}, nil
}

func (s *server) AddParticipant(_ context.Context, req *gen.AddParticipantRequest) (*gen.AddParticipantResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := resolveDisplayName(s.db, req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	logger.Infof("AddParticipant: Adding user %s to chat %s", username, req.ChatId)
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		logger.Errorf("AddParticipant error: Chat %s not found", req.ChatId)
		return &gen.AddParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		logger.Errorf("AddParticipant error: Chat %s is not a group chat (type: %s)", req.ChatId, chat.Type)
		return &gen.AddParticipantResponse{Success: false, Message: "Participants can only be added to group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		logger.Errorf("AddParticipant error: Failed to parse participants for chat %s: %v", req.ChatId, err)
		return &gen.AddParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	// Check if user already in chat
	for _, p := range participants {
		if p == username {
			logger.Infof("AddParticipant: User %s is already in chat %s", username, req.ChatId)
			return &gen.AddParticipantResponse{Success: false, Message: "User already in chat"}, nil
		}
	}

	participants = append(participants, username)
	updatedParticipants, _ := json.Marshal(participants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		logger.Errorf("AddParticipant error: Failed to update DB for chat %s: %v", req.ChatId, err)
		return &gen.AddParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	// Notify all participants about the change
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers() // Refresh lists for everyone

	logger.Infof("AddParticipant success: User %s added to chat %s", username, req.ChatId)
	return &gen.AddParticipantResponse{Success: true, Message: "User added successfully"}, nil
}

func (s *server) RemoveParticipant(_ context.Context, req *gen.RemoveParticipantRequest) (*gen.RemoveParticipantResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := resolveDisplayName(s.db, req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	logger.Infof("RemoveParticipant: Removing user %s from chat %s", username, req.ChatId)
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		logger.Errorf("RemoveParticipant error: Chat %s not found", req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		logger.Errorf("RemoveParticipant error: Chat %s is not a group chat", req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Participants can only be removed from group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		logger.Errorf("RemoveParticipant error: Failed to parse participants: %v", err)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	newParticipants := []string{}
	found := false
	for _, p := range participants {
		if p != username {
			newParticipants = append(newParticipants, p)
		} else {
			found = true
		}
	}

	if !found {
		logger.Errorf("RemoveParticipant error: User %s not in chat %s", username, req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "User not in chat"}, nil
	}

	updatedParticipants, _ := json.Marshal(newParticipants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		logger.Errorf("RemoveParticipant error: Failed to update DB: %v", err)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	// Notify all participants
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	_ = s.db.IncrementUserChatListVersion(username) // Notify the removed user too
	s.broadcastOnlineUsers()

	logger.Infof("RemoveParticipant success: User %s removed from chat %s", username, req.ChatId)
	return &gen.RemoveParticipantResponse{Success: true, Message: "User removed successfully"}, nil
}

func (s *server) DeleteChat(_ context.Context, req *gen.DeleteChatRequest) (*gen.DeleteChatResponse, error) {
	if req.ChatId == "" {
		return &gen.DeleteChatResponse{Success: false, Message: "Chat ID is required"}, nil
	}

	requesterUsername := req.RequesterUsername
	if req.RequesterUserId != "" {
		resolved := resolveDisplayName(s.db, req.RequesterUserId)
		if resolved != "" {
			requesterUsername = resolved
		}
	}

	logger.Infof("DeleteChat: Request to delete chat %s by %s", req.ChatId, requesterUsername)

	// 1. Get all participants and creator before deletion
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		// Fallback: check if this is a hermes session that wasn't synced to chats
		if s.hermesDB != nil {
			if sessionID := s.hermesDB.GetSessionID(req.ChatId); sessionID != "" {
				logger.Infof("DeleteChat: Chat %s not in chats but found in hermes_sessions, cleaning up", req.ChatId)
				s.hermesDB.DeleteSession(req.ChatId)
				// Also try to delete from chats (may not exist, that's OK)
				_ = s.db.DeleteChat(req.ChatId)
				return &gen.DeleteChatResponse{Success: true, Message: "Hermes session deleted"}, nil
			}
		}
		logger.Errorf("DeleteChat warning: Chat %s not found or DB error: %v", req.ChatId, err)
		// Return error to inform user that chat is already deleted
		return &gen.DeleteChatResponse{Success: false, Message: "Chat or group already deleted"}, nil
	}

	// Security check: only creator can delete group chats
	// We allow users to delete their own direct chats, but groups must be deleted by the creator
	if chat.Type == "group" && chat.CreatorUsername != requesterUsername {
		logger.Errorf("DeleteChat error: User %s is not authorized to delete group chat %s (creator: %s)",
			requesterUsername, req.ChatId, chat.CreatorUsername)
		return &gen.DeleteChatResponse{
			Success: false,
			Message: "You don't have permission to delete this group. Only the group administrator can delete it.",
		}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		logger.Warnf("DeleteChat warning: Failed to parse participants for %s: %v", req.ChatId, err)
	}

	// 2. Get all image URLs to delete files
	imageURLs, err := s.db.GetChatMessagesImageURLs(req.ChatId)
	if err != nil {
		logger.Warnf("DeleteChat warning: Failed to get image URLs for chat %s: %v", req.ChatId, err)
	}

	// 3. Delete all image files from disk
	for _, url := range imageURLs {
		if err := DeleteImageFile(url); err != nil {
			logger.Errorf("DeleteChat error: Failed to delete image file %s: %v", url, err)
		}
	}

	// 4. Delete the chat and all messages from database
	err = s.db.DeleteChat(req.ChatId)
	if err != nil {
		logger.Errorf("DeleteChat error: Failed to delete chat %s from DB: %v", req.ChatId, err)
		return &gen.DeleteChatResponse{Success: false, Message: err.Error()}, nil
	}

	// 4b. Cascade delete AI-specific data for hermes chats
	if chat.Type == "hermes" {
		if s.hermesDB != nil {
			s.hermesDB.DeleteSession(req.ChatId)
			logger.Infof("DeleteChat: Hermes session %s deleted from hermes_sessions", req.ChatId)
		}
		// Also clean up any orphaned hermes_messages for this session
		_, _ = s.db.Exec("DELETE FROM hermes_messages WHERE session_id = $1", req.ChatId)
	} else if chat.Type == "owl" {
		// Clean up orphaned owl_messages (FK CASCADE handles settings)
		_, _ = s.db.Exec("DELETE FROM owl_messages WHERE chat_id = $1", req.ChatId)
		_, _ = s.db.Exec("DELETE FROM owl_chat_settings WHERE chat_id = $1", req.ChatId)
	}

	logger.Infof("DeleteChat success: Chat %s deleted (type=%s).", req.ChatId, chat.Type)

	// 5. Increment version for all former participants so their lists refresh
	// Skip for AI chats (owl/hermes) — participants contains UUIDs, not usernames,
	// and the deleting user already knows the chat is gone. Broadcast below is enough.
	if chat.Type != "owl" && chat.Type != "hermes" {
		logger.Infof("DeleteChat: Notifying %d participants.", len(participants))
		_ = s.db.IncrementChatListVersionByUsernames(participants)
	}

	// 6. Send signal to clear cache for all participants
	s.hub.Broadcast(&gen.Message{
		User:   "SYSTEM",
		Text:   "CLEAR_CACHE:" + req.ChatId,
		RoomId: req.ChatId,
	})

	// 7. Send signal to exit the deleted chat for all participants
	s.hub.Broadcast(&gen.Message{
		User:   "SYSTEM",
		Text:   "CHAT_DELETED:" + req.ChatId,
		RoomId: req.ChatId,
	})

	// 8. Broadcast update signal
	s.broadcastOnlineUsers()

	return &gen.DeleteChatResponse{Success: true, Message: "Chat deleted successfully"}, nil
}

func (s *server) UpdateChatName(_ context.Context, req *gen.UpdateChatNameRequest) (*gen.UpdateChatNameResponse, error) {
	if req.ChatId == "" || req.NewName == "" {
		return &gen.UpdateChatNameResponse{Success: false, Message: "Chat ID and New Name are required"}, nil
	}

	logger.Infof("UpdateChatName: Updating chat %s to '%s'", req.ChatId, req.NewName)

	err := s.db.UpdateChatName(req.ChatId, req.NewName)
	if err != nil {
		logger.Errorf("UpdateChatName error: %v", err)
		return &gen.UpdateChatNameResponse{Success: false, Message: err.Error()}, nil
	}

	// Increment version for all participants so their lists refresh
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers()

	return &gen.UpdateChatNameResponse{Success: true, Message: "Chat name updated successfully"}, nil
}

func (s *server) UpdateChatAvatar(_ context.Context, req *gen.UpdateChatAvatarRequest) (*gen.UpdateChatAvatarResponse, error) {
	if req.ChatId == "" || req.AvatarUrl == "" {
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Chat ID and Avatar URL are required"}, nil
	}

	username := req.Username
	if req.UserId != "" {
		resolved := resolveDisplayName(s.db, req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	logger.Infof("UpdateChatAvatar: Checking admin status for chat %s, user %s", req.ChatId, username)

	// Get chat to verify admin status
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		logger.Errorf("UpdateChatAvatar error: Chat not found: %v", err)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Chat not found"}, nil
	}

	// Verify user is the creator/admin
	if chat.CreatorUsername != username {
		logger.Errorf("UpdateChatAvatar error: User %s is not admin (creator: %s)", username, chat.CreatorUsername)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Only chat admin can change group photo"}, nil
	}

	// Update avatar (both thumbnail and full version)
	err = s.db.UpdateChatAvatarWithFull(req.ChatId, req.AvatarUrl, req.FullAvatarUrl)
	if err != nil {
		logger.Errorf("UpdateChatAvatar error: %v", err)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	logger.Infof("UpdateChatAvatar: Updated avatar for chat %s by admin %s (thumb: %s, full: %s)", req.ChatId, username, req.AvatarUrl, req.FullAvatarUrl)

	// Increment version for all participants so their lists refresh
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers()

	return &gen.UpdateChatAvatarResponse{Success: true, Message: "Chat avatar updated successfully"}, nil
}

func (s *server) UpdateChatSettings(_ context.Context, req *gen.UpdateChatSettingsRequest) (*gen.UpdateChatSettingsResponse, error) {
	if req.ChatId == "" {
		return &gen.UpdateChatSettingsResponse{Success: false, Message: "Chat ID is required"}, nil
	}

	// Verify user is admin
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		return &gen.UpdateChatSettingsResponse{Success: false, Message: "Chat not found"}, nil
	}

	username := resolveDisplayName(s.db, req.UserId)
	if chat.CreatorUsername != username {
		return &gen.UpdateChatSettingsResponse{Success: false, Message: "Unauthorized: only admin can change settings"}, nil
	}

	err = s.db.UpdateChatSettings(req.ChatId, req.AllowMembersToAdd)
	if err != nil {
		return &gen.UpdateChatSettingsResponse{Success: false, Message: err.Error()}, nil
	}

	// Refresh list for participants
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers()

	logger.Infof("UpdateChatSettings: Chat %s allow_add updated to %v by %s", req.ChatId, req.AllowMembersToAdd, username)
	return &gen.UpdateChatSettingsResponse{Success: true, Message: "Settings updated successfully"}, nil
}
