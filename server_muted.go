package main

import (
	"LavenderMessenger/gen"
	"context"

	"github.com/google/uuid"
)

func (s *server) GetMutedChats(_ context.Context, req *gen.GetMutedChatsRequest) (*gen.GetMutedChatsResponse, error) {
	if req.UserId == "" {
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}

	var mutedChats []string
	var err error

	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		mutedChats, err = s.db.GetMutedChatsByUserID(req.UserId)
	} else {
		mutedChats, err = s.db.GetMutedChats(req.UserId)
	}

	if err != nil {
		s.logErrorOnce("GetMutedChats:"+req.UserId, "Failed to get muted chats for user %s: %v", req.UserId, err)
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}
	return &gen.GetMutedChatsResponse{RoomIds: mutedChats}, nil
}

func (s *server) SetMutedChat(_ context.Context, req *gen.SetMutedChatRequest) (*gen.SetMutedChatResponse, error) {
	if req.UserId == "" {
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	var err error
	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		err = s.db.SetMutedChatByUserID(req.UserId, req.RoomId, req.Muted)
	} else {
		err = s.db.SetMutedChat(req.UserId, req.RoomId, req.Muted)
	}

	if err != nil {
		s.logErrorOnce("SetMutedChat:"+req.UserId, "Failed to set muted status for user %s in room %s (muted=%v): %v", req.UserId, req.RoomId, req.Muted, err)
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	action := "muted"
	if !req.Muted {
		action = "unmuted"
	}
	logger.Infof("Chat %s for user %s in room %s", action, req.UserId, req.RoomId)
	return &gen.SetMutedChatResponse{Success: true}, nil
}
