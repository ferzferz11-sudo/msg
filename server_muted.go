package main

import (
	"LavenderMessenger/gen"
	"context"

	"github.com/google/uuid"
)

func (s *server) GetMutedChats(ctx context.Context, req *gen.GetMutedChatsRequest) (*gen.GetMutedChatsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}

	var mutedChats []string
	var err error

	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(userID); uuidErr == nil {
		mutedChats, err = s.db.GetMutedChatsByUserID(userID)
	} else {
		mutedChats, err = s.db.GetMutedChats(userID)
	}

	if err != nil {
		s.logErrorOnce("GetMutedChats:"+userID, "Failed to get muted chats for user %s: %v", userID, err)
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}
	return &gen.GetMutedChatsResponse{RoomIds: mutedChats}, nil
}

func (s *server) SetMutedChat(ctx context.Context, req *gen.SetMutedChatRequest) (*gen.SetMutedChatResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	var err error
	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(userID); uuidErr == nil {
		err = s.db.SetMutedChatByUserID(userID, req.RoomId, req.Muted)
	} else {
		err = s.db.SetMutedChat(userID, req.RoomId, req.Muted)
	}

	if err != nil {
		s.logErrorOnce("SetMutedChat:"+userID, "Failed to set muted status for user %s in room %s (muted=%v): %v", userID, req.RoomId, req.Muted, err)
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	action := "muted"
	if !req.Muted {
		action = "unmuted"
	}
	logger.Infof("Chat %s for user %s in room %s", action, userID, req.RoomId)
	return &gen.SetMutedChatResponse{Success: true}, nil
}
