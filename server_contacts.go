package main

import (
	"LavenderMessenger/gen"
	"context"
)

func (s *server) AddContact(_ context.Context, req *gen.AddContactRequest) (*gen.AddContactResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	err := s.db.AddContact(username, req.ContactUsername)
	if err != nil {
		logger.Infof("Failed to add contact %s for %s: %v", req.ContactUsername, username, err)
		return &gen.AddContactResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("User %s added contact %s", username, req.ContactUsername)
	return &gen.AddContactResponse{Success: true, Message: "Contact added successfully"}, nil
}

func (s *server) RemoveContact(_ context.Context, req *gen.RemoveContactRequest) (*gen.RemoveContactResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	err := s.db.RemoveContact(username, req.ContactUsername)
	if err != nil {
		logger.Infof("Failed to remove contact %s for %s: %v", req.ContactUsername, username, err)
		return &gen.RemoveContactResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("User %s removed contact %s", username, req.ContactUsername)
	return &gen.RemoveContactResponse{Success: true, Message: "Contact removed successfully"}, nil
}

func (s *server) GetContacts(_ context.Context, req *gen.GetContactsRequest) (*gen.GetContactsResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	contacts, err := s.db.GetContacts(username)
	if err != nil {
		logger.Infof("Failed to get contacts for %s: %v", username, err)
		return &gen.GetContactsResponse{Contacts: nil}, nil
	}
	return &gen.GetContactsResponse{Contacts: contacts}, nil
}

func (s *server) GetChatListVersion(_ context.Context, req *gen.GetChatListVersionRequest) (*gen.GetChatListVersionResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	version, err := s.db.GetUserChatListVersion(username)
	if err != nil {
		return &gen.GetChatListVersionResponse{Version: 0}, nil
	}
	return &gen.GetChatListVersionResponse{Version: version}, nil
}
