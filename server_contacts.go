package main

import (
	"LavenderMessenger/gen"
	"context"
)

func (s *server) AddContact(ctx context.Context, req *gen.AddContactRequest) (*gen.AddContactResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	contactID := req.UserId
	if contactID == "" {
		contactID = req.ContactUsername
	}

	var err error
	if isUUID(userID) && isUUID(contactID) {
		err = s.db.AddContactByUserID(userID, contactID)
	} else {
		username := resolveDisplayName(s.db, userID)
		err = s.db.AddContact(username, req.ContactUsername)
	}

	if err != nil {
		logger.Infof("Failed to add contact %s for %s: %v", contactID, userID, err)
		return &gen.AddContactResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("User %s added contact %s", userID, contactID)
	return &gen.AddContactResponse{Success: true, Message: "Contact added successfully"}, nil
}

func (s *server) RemoveContact(ctx context.Context, req *gen.RemoveContactRequest) (*gen.RemoveContactResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	contactID := req.UserId
	if contactID == "" {
		contactID = req.ContactUsername
	}

	var err error
	if isUUID(userID) && isUUID(contactID) {
		err = s.db.RemoveContactByUserID(userID, contactID)
	} else {
		username := resolveDisplayName(s.db, userID)
		err = s.db.RemoveContact(username, req.ContactUsername)
	}

	if err != nil {
		logger.Infof("Failed to remove contact %s for %s: %v", contactID, userID, err)
		return &gen.RemoveContactResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("User %s removed contact %s", userID, contactID)
	return &gen.RemoveContactResponse{Success: true, Message: "Contact removed successfully"}, nil
}

func (s *server) GetContacts(ctx context.Context, req *gen.GetContactsRequest) (*gen.GetContactsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	var contacts []string
	var err error
	if isUUID(userID) {
		contacts, err = s.db.GetContactsByUserID(userID)
	} else {
		contacts, err = s.db.GetContacts(userID)
	}

	if err != nil {
		logger.Infof("Failed to get contacts for %s: %v", userID, err)
		return &gen.GetContactsResponse{Contacts: nil}, nil
	}
	return &gen.GetContactsResponse{Contacts: contacts}, nil
}

func (s *server) GetChatListVersion(ctx context.Context, req *gen.GetChatListVersionRequest) (*gen.GetChatListVersionResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	var version int64
	var err error
	if isUUID(userID) {
		username, _ := s.db.GetUserByID(userID)
		version, err = s.db.GetUserChatListVersion(username)
	} else {
		version, err = s.db.GetUserChatListVersion(userID)
	}

	if err != nil {
		return &gen.GetChatListVersionResponse{Version: 0}, nil
	}
	return &gen.GetChatListVersionResponse{Version: version}, nil
}
