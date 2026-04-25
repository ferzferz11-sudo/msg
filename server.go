// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.

package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

// server implements the gRPC ChatService interface
type server struct {
	gen.UnimplementedChatServiceServer
	hub         *Hub          // Hub for managing client connections
	db          *DB           // Database for message persistence
	firebaseApp *firebase.App // Firebase Admin SDK instance
	recentMsgs  sync.Map      // Cache for deduplicating identical rapid messages
}

// Chat handles bidirectional streaming for real-time messaging
func (s *server) Chat(stream gen.ChatService_ChatServer) error {
	var connectedUser string = "Anonymous"

	// Register the new client stream with the hub
	s.hub.Register(stream)
	defer func() {
		// Unregister the client when the connection ends
		s.hub.Unregister(stream)
		log.Printf("Client disconnected: %s", connectedUser)
	}()

	for {
		// Receive message from client
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Check authentication on first message (when password is provided)
		if msg.Password != "" && !s.hub.IsAuthenticated(stream) {
			// Trim username to avoid whitespace issues
			trimmedUser := strings.TrimSpace(msg.User)
			msg.User = trimmedUser
			connectedUser = msg.User

			log.Printf("Auth attempt: %s", connectedUser)

			// Check if user exists first
			userExists, err := s.db.UserExists(msg.User)
			if err != nil {
				log.Printf("Failed to check user existence: %v", err)
				return err
			}

			if userExists {
				// User exists, verify password
				storedHash, err := s.db.GetUserPasswordHash(msg.User)
				if err != nil {
					log.Printf("Failed to get password hash: %v", err)
					return err
				}

				if !CheckPassword(msg.Password, storedHash) {
					log.Printf("Authentication failed for user: %s", msg.User)

					// Send authentication failure message to the client
					authFailMsg := &gen.Message{
						User:      "SYSTEM",
						Text:      "AUTH_FAILED",
						Id:        uuid.New().String(),
						CreatedAt: timestamppb.Now(),
					}
					if err := stream.Send(authFailMsg); err != nil {
						log.Printf("Failed to send auth failed message: %v", err)
					}

					return fmt.Errorf("authentication failed")
				}
			} else {
				// New user, hash password and create
				passwordHash, err := HashPassword(msg.Password)
				if err != nil {
					log.Printf("Failed to hash password: %v", err)
					return err
				}

				err = s.db.SaveUser(msg.User, passwordHash)
				if err != nil {
					log.Printf("Failed to save user: %v", err)
					return err
				}
				log.Printf("Created new user: %s", msg.User)

				// Send registration success message to the client
				regMsg := &gen.Message{
					User:      "SYSTEM",
					Text:      "REGISTRATION_SUCCESS",
					Id:        uuid.New().String(),
					CreatedAt: timestamppb.Now(),
				}
				if err := stream.Send(regMsg); err != nil {
					log.Printf("Failed to send registration success message: %v", err)
				}
			}

			// Mark stream as authenticated
			s.hub.SetAuthenticated(stream, true)
			log.Printf("User authenticated: %s", msg.User)

			// Clear password from message before broadcasting
			msg.Password = ""
		}

		// Deduplication logic for identical rapid messages
		// Prevents double-posting from some source apps (e.g. Google Photos)
		if msg.Text != "" || msg.ImageUrl != "" {
			msgHash := fmt.Sprintf("%s:%s:%s:%s", msg.User, msg.RoomId, msg.Text, msg.ImageUrl)
			now := time.Now()
			if lastTime, ok := s.recentMsgs.Load(msgHash); ok {
				if now.Sub(lastTime.(time.Time)) < 2*time.Second {
					log.Printf("Deduplicated identical message from %s in %s", msg.User, msg.RoomId)
					continue
				}
			}
			s.recentMsgs.Store(msgHash, now)
		}

		// Reject messages from unauthenticated streams (except first auth message)
		// Temporarily disabled to debug authentication issues
		/*if !s.hub.IsAuthenticated(stream) && msg.Password == "" {
			log.Printf("Rejected message from unauthenticated stream")
			return fmt.Errorf("not authenticated")
		}*/

		// Обновляем имя пользователя в хабе, чтобы GetClients работал корректно
		s.hub.UpdateName(stream, msg.User)

		// Генерируем уникальный ID для сообщения
		msg.Id = uuid.New().String()

		// Set server timestamp for message
		msg.CreatedAt = timestamppb.Now()

		// Trim username to avoid whitespace issues
		trimmedUser := strings.TrimSpace(msg.User)
		msg.User = trimmedUser

		// Determine room ID
		roomID := msg.RoomId

		// Skip empty messages (unless they have an image)
		if strings.TrimSpace(msg.Text) == "" && msg.ImageUrl == "" {
			if roomID != "" {
				log.Printf("Auth signal from %s (Session room: %s)", msg.User, roomID)
			} else {
				log.Printf("Global Auth signal from %s", msg.User)
			}
			continue
		}

		log.Printf("[%s] in %s: %s (ImageURL: %s)", msg.User, roomID, msg.Text, msg.ImageUrl)

		if roomID == "" {
			log.Printf("Skipping message with empty room ID from %s", msg.User)
			continue
		}

		// Skip join messages (don't save to database)
		if strings.HasSuffix(msg.Text, " joined") || strings.HasSuffix(msg.Text, " присоединился") {
			// Still broadcast but don't save to DB
			log.Printf("Skipping join message from DB save: %s", msg.Text)
		} else {
			// Encrypt message text before saving to database
			encryptedText, err := encrypt(msg.Text)
			if err != nil {
				log.Printf("Failed to encrypt message: %v", err)
				continue
			}

			// Save encrypted message to database with UUID
			imageURL := msg.ImageUrl
			err = s.db.SaveMessage(msg.Id, msg.User, encryptedText, msg.CreatedAt.AsTime(), msg.RepliedToMessageId, msg.RepliedToUser, msg.RepliedToText, roomID, imageURL)
			if err != nil {
				log.Printf("Failed to save message to DB: %v", err)
			} else {
				log.Printf("Message saved to DB successfully: %s in room %s", msg.Id, roomID)
			}
		}

		// Update user's current room in hub
		s.hub.SetRoom(stream, roomID)

		// Get user's avatar URL
		avatarURL, err := s.db.GetUserAvatar(msg.User)
		if err != nil {
			log.Printf("Failed to get avatar for %s: %v", msg.User, err)
		}
		msg.AvatarUrl = avatarURL

		// Clear password and reply fields from message before broadcasting
		msg.Password = ""
		// Keep replied_to fields for display to clients

		// Broadcast message to all connected clients
		s.hub.Broadcast(msg)

		// Send push notification to all users in the room (except sender)
		// This ensures users in background receive notifications
		allUsers, err := s.db.GetAllUsers()
		if err != nil {
			log.Printf("Failed to get all users for push notifications: %v", err)
		} else {
			for _, user := range allUsers {
				// Skip the sender
				if user == msg.User {
					continue
				}

				// Check if user is a participant in the room
				userInRoom := false
				chat, err := s.db.GetChat(roomID)
				if err == nil {
					var participants []string
					json.Unmarshal([]byte(chat.Participants), &participants)
					for _, p := range participants {
						if p == user {
							userInRoom = true
							break
						}
					}
				}

				if !userInRoom {
					continue
				}

				// Send push notification to all users in the room
				// Online users will receive via gRPC, background users via push
				log.Printf("Sending push notification to user: %s", user)
				s.sendPushNotification(user, msg.User, msg.Text, roomID)
			}
		}
	}
}

// GetClients возвращает список активных пользователей
func (s *server) GetClients(ctx context.Context, req *gen.ClientListRequest) (*gen.ClientListResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	_ = req // req is required by gRPC interface but not used here
	users := s.hub.GetOnlineUsers()
	return &gen.ClientListResponse{
		Clients: users,
	}, nil
}

// GetAllUsers возвращает список всех зарегистрированных пользователей
func (s *server) GetAllUsers(ctx context.Context, req *gen.GetAllUsersRequest) (*gen.GetAllUsersResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	_ = req // req is required by gRPC interface but not used here
	users, err := s.db.GetAllUsers()
	if err != nil {
		log.Printf("Error fetching all users: %v", err)
		return nil, err
	}
	return &gen.GetAllUsersResponse{
		Users: users,
	}, nil
}

// GetHistory возвращает историю сообщений
func (s *server) GetHistory(_ context.Context, req *gen.GetHistoryRequest) (*gen.GetHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	roomID := req.Room
	if roomID == "" {
		return &gen.GetHistoryResponse{Messages: nil}, nil
	}

	rawMessages, err := s.db.GetMessages(limit, roomID)
	if err != nil {
		log.Printf("Error fetching history: %v", err)
		return nil, err
	}

	var messages []*gen.Message
	// Проходим в обратном порядке, чтобы сообщения были от старых к новым
	for i := len(rawMessages) - 1; i >= 0; i-- {
		m := rawMessages[i]

		// Check if encrypted data is empty
		if len(m.Encrypted) == 0 {
			log.Printf("Warning: message %s has empty encrypted data", m.MessageID)
			continue // Skip messages with no encrypted data
		}

		// Расшифровываем текст из базы
		decryptedText, err := decrypt(m.Encrypted)
		if err != nil {
			log.Printf("Failed to decrypt history message %s: %v", m.MessageID, err)
			decryptedText = "[Decryption Error]"
		}

		// Check if decrypted text is empty (skip only if no image)
		if decryptedText == "" && m.ImageURL == "" {
			log.Printf("Warning: message %s decrypted to empty string", m.MessageID)
			continue // Skip empty messages without images
		}

		// Получаем реакции для сообщения
		rawReactions, _ := s.db.GetReactionsForMessage(m.MessageID)
		var reactions []*gen.Reaction
		for _, r := range rawReactions {
			reactions = append(reactions, &gen.Reaction{
				User:  r.Username,
				Emoji: r.Emoji,
			})
		}

		messages = append(messages, &gen.Message{
			Id:                 m.MessageID,
			User:               m.Username,
			Text:               decryptedText,
			CreatedAt:          timestamppb.New(m.CreatedAt),
			Reactions:          reactions,
			RepliedToMessageId: m.RepliedToMessageID,
			RepliedToUser:      m.RepliedToUser,
			RepliedToText:      m.RepliedToText,
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			AvatarUrl:          m.AvatarURL,
			ImageUrl:           m.ImageURL,
			Edited:             m.Edited,
		})
	}

	return &gen.GetHistoryResponse{
		Messages: messages,
	}, nil
}

// SetReaction saves or updates a reaction and broadcasts it
func (s *server) SetReaction(_ context.Context, req *gen.ReactionRequest) (*gen.ReactionResponse, error) {
	err := s.db.SetReaction(req.MessageId, req.Reaction.User, req.Reaction.Emoji)
	if err != nil {
		log.Printf("Failed to set reaction: %v", err)
		return &gen.ReactionResponse{Success: false}, err
	}

	// Broadcast the reaction update to all clients
	// Note: We might want a dedicated broadcast for reactions,
	// but for now, we'll let clients refresh or we could send a special Message.
	// For 0.9.6, we'll just ensure it's in the DB for the next history fetch.

	return &gen.ReactionResponse{Success: true}, nil
}

// RegisterToken сохраняет FCM токен для пользователя
func (s *server) RegisterToken(_ context.Context, req *gen.TokenRequest) (*gen.TokenResponse, error) {
	err := s.db.SaveUserToken(req.User, req.Token)
	if err != nil {
		log.Printf("Failed to save token for %s: %v", req.User, err)
		return &gen.TokenResponse{Success: false}, err
	}

	displayToken := req.Token
	if len(displayToken) > 20 {
		displayToken = displayToken[:20] + "..."
	}
	log.Printf("Registered FCM token for user %s: [%s]", req.User, displayToken)
	return &gen.TokenResponse{Success: true}, nil
}

// GetChats возвращает список чатов для пользователя
func (s *server) GetChats(_ context.Context, req *gen.GetChatsRequest) (*gen.GetChatsResponse, error) {
	chats, err := s.db.GetUserChats(req.Username)
	if err != nil {
		log.Printf("Error fetching chats: %v", err)
		return nil, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, &gen.ChatInfo{
			Id:              c.ID,
			Name:            c.Name,
			Type:            c.Type,
			Participants:    c.Participants,
			CreatedAt:       timestamppb.New(c.CreatedAt),
			UnreadCount:     int32(c.UnreadCount),
			LastMessageTime: timestamppb.New(c.LastMessageTime),
		})
	}

	return &gen.GetChatsResponse{Chats: chatInfos}, nil
}

// CreateDirectChat создает или находит личный чат между двумя пользователями
func (s *server) CreateDirectChat(_ context.Context, req *gen.CreateDirectChatRequest) (*gen.CreateDirectChatResponse, error) {
	chatID, err := s.db.GetDirectChatBetweenUsers(req.User1, req.User2)
	if err != nil {
		log.Printf("Error creating direct chat: %v", err)
		return &gen.CreateDirectChatResponse{Success: false}, err
	}

	return &gen.CreateDirectChatResponse{ChatId: chatID, Success: true}, nil
}

func (s *server) CreateGroupChat(_ context.Context, req *gen.CreateGroupChatRequest) (*gen.CreateGroupChatResponse, error) {
	chatID := uuid.New().String()

	// Convert participants slice to JSON string
	participants := "["
	for i, p := range req.Participants {
		participants += "\"" + p + "\""
		if i < len(req.Participants)-1 {
			participants += ", "
		}
	}
	participants += "]"

	err := s.db.CreateChat(chatID, req.Name, "group", participants)
	if err != nil {
		log.Printf("Failed to create group chat: %v", err)
		return &gen.CreateGroupChatResponse{Success: false}, err
	}
	return &gen.CreateGroupChatResponse{ChatId: chatID, Success: true}, nil
}

// UpdateUsername обновляет имя пользователя
func (s *server) UpdateUsername(_ context.Context, req *gen.UpdateUsernameRequest) (*gen.UpdateUsernameResponse, error) {
	err := s.db.UpdateUsername(req.OldUsername, req.NewUsername)
	if err != nil {
		log.Printf("Failed to update username from %s to %s: %v", req.OldUsername, req.NewUsername, err)
		return &gen.UpdateUsernameResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Username updated from %s to %s", req.OldUsername, req.NewUsername)
	return &gen.UpdateUsernameResponse{
		Success: true,
		Message: "Username updated successfully",
	}, nil
}

// UpdatePassword обновляет пароль пользователя
func (s *server) UpdatePassword(_ context.Context, req *gen.UpdatePasswordRequest) (*gen.UpdatePasswordResponse, error) {
	// Проверяем старый пароль
	storedHash, err := s.db.GetUserPasswordHash(req.Username)
	if err != nil {
		log.Printf("Failed to get password hash for %s: %v", req.Username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "User not found",
		}, err
	}

	if !CheckPassword(req.OldPassword, storedHash) {
		log.Printf("Old password verification failed for user: %s", req.Username)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "Old password is incorrect",
		}, fmt.Errorf("old password verification failed")
	}

	// Обновляем пароль
	err = s.db.UpdatePassword(req.Username, req.NewPassword)
	if err != nil {
		log.Printf("Failed to update password for %s: %v", req.Username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Password updated for user: %s", req.Username)
	return &gen.UpdatePasswordResponse{
		Success: true,
		Message: "Password updated successfully",
	}, nil
}

// MarkRead marks messages in a room as read for a user
func (s *server) MarkRead(_ context.Context, req *gen.MarkReadRequest) (*gen.MarkReadResponse, error) {
	err := s.db.MarkRead(req.RoomId, req.Username)
	if err != nil {
		log.Printf("Failed to mark read for %s in room %s: %v", req.Username, req.RoomId, err)
		return &gen.MarkReadResponse{Success: false}, err
	}

	log.Printf("Marked read for %s in room %s", req.Username, req.RoomId)
	return &gen.MarkReadResponse{Success: true}, nil
}

// UpdateAvatar updates the avatar URL for a user
func (s *server) UpdateAvatar(_ context.Context, req *gen.UpdateAvatarRequest) (*gen.UpdateAvatarResponse, error) {
	err := s.db.UpdateAvatar(req.Username, req.AvatarUrl)
	if err != nil {
		log.Printf("Failed to update avatar for %s: %v", req.Username, err)
		return &gen.UpdateAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated avatar for %s", req.Username)
	return &gen.UpdateAvatarResponse{Success: true, Message: "Avatar updated successfully"}, nil
}

// UpdateProfile updates user bio and status
func (s *server) UpdateProfile(_ context.Context, req *gen.UpdateProfileRequest) (*gen.UpdateProfileResponse, error) {
	err := s.db.UpdateProfile(req.Username, req.Bio, req.Status)
	if err != nil {
		log.Printf("Failed to update profile for %s: %v", req.Username, err)
		return &gen.UpdateProfileResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated profile for %s", req.Username)
	return &gen.UpdateProfileResponse{Success: true, Message: "Profile updated successfully"}, nil
}

// GetUserProfile retrieves user profile information
func (s *server) GetUserProfile(_ context.Context, req *gen.GetUserProfileRequest) (*gen.GetUserProfileResponse, error) {
	profile, err := s.db.GetUserProfile(req.Username)
	if err != nil {
		log.Printf("Failed to get profile for %s: %v", req.Username, err)
		return &gen.GetUserProfileResponse{}, nil
	}

	return &gen.GetUserProfileResponse{
		Username:  profile.Username,
		Bio:       profile.Bio,
		Status:    profile.Status,
		AvatarUrl: profile.AvatarURL,
	}, nil
}

// GetUserAvatar retrieves the avatar URL for a user
func (s *server) GetUserAvatar(_ context.Context, req *gen.GetUserAvatarRequest) (*gen.GetUserAvatarResponse, error) {
	avatarURL, err := s.db.GetUserAvatar(req.Username)
	if err != nil {
		log.Printf("Failed to get avatar for %s: %v", req.Username, err)
		return &gen.GetUserAvatarResponse{AvatarUrl: ""}, nil
	}

	return &gen.GetUserAvatarResponse{AvatarUrl: avatarURL}, nil
}

// AddParticipant adds a user to a group chat
func (s *server) AddParticipant(_ context.Context, req *gen.AddParticipantRequest) (*gen.AddParticipantResponse, error) {
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		return &gen.AddParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		return &gen.AddParticipantResponse{Success: false, Message: "Participants can only be added to group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		return &gen.AddParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	// Check if user already in chat
	for _, p := range participants {
		if p == req.Username {
			return &gen.AddParticipantResponse{Success: false, Message: "User already in chat"}, nil
		}
	}

	participants = append(participants, req.Username)
	updatedParticipants, _ := json.Marshal(participants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		return &gen.AddParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	return &gen.AddParticipantResponse{Success: true, Message: "User added successfully"}, nil
}

// RemoveParticipant removes a user from a group chat
func (s *server) RemoveParticipant(_ context.Context, req *gen.RemoveParticipantRequest) (*gen.RemoveParticipantResponse, error) {
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		return &gen.RemoveParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		return &gen.RemoveParticipantResponse{Success: false, Message: "Participants can only be removed from group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		return &gen.RemoveParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	newParticipants := []string{}
	found := false
	for _, p := range participants {
		if p != req.Username {
			newParticipants = append(newParticipants, p)
		} else {
			found = true
		}
	}

	if !found {
		return &gen.RemoveParticipantResponse{Success: false, Message: "User not in chat"}, nil
	}

	updatedParticipants, _ := json.Marshal(newParticipants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		return &gen.RemoveParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	return &gen.RemoveParticipantResponse{Success: true, Message: "User removed successfully"}, nil
}

// DeleteChat deletes a chat and all its messages and images
func (s *server) DeleteChat(_ context.Context, req *gen.DeleteChatRequest) (*gen.DeleteChatResponse, error) {
	if req.ChatId == "" {
		return &gen.DeleteChatResponse{Success: false, Message: "Chat ID is required"}, nil
	}

	// 1. Get all image URLs for messages in this chat
	imageURLs, err := s.db.GetChatMessagesImageURLs(req.ChatId)
	if err != nil {
		log.Printf("Failed to get image URLs for chat %s: %v", req.ChatId, err)
		// Continue anyway to delete the chat record
	}

	// 2. Delete all image files from disk
	for _, url := range imageURLs {
		if err := DeleteImageFile(url); err != nil {
			log.Printf("Failed to delete image file %s: %v", url, err)
		}
	}

	// 3. Delete the chat and all messages from database
	err = s.db.DeleteChat(req.ChatId)
	if err != nil {
		log.Printf("Failed to delete chat %s from DB: %v", req.ChatId, err)
		return &gen.DeleteChatResponse{Success: false, Message: err.Error()}, err
	}

	log.Printf("Successfully deleted chat %s and all its content", req.ChatId)
	return &gen.DeleteChatResponse{Success: true, Message: "Chat deleted successfully"}, nil
}

// DeleteProfile deletes a user's profile and all their data
func (s *server) DeleteProfile(_ context.Context, req *gen.DeleteProfileRequest) (*gen.DeleteProfileResponse, error) {
	if req.Username == "" {
		return &gen.DeleteProfileResponse{Success: false, Message: "Username is required"}, nil
	}

	err := s.db.DeleteProfile(req.Username)
	if err != nil {
		log.Printf("Failed to delete profile for %s: %v", req.Username, err)
		return &gen.DeleteProfileResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Successfully deleted profile for user: %s", req.Username)
	return &gen.DeleteProfileResponse{Success: true, Message: "Profile deleted successfully"}, nil
}

// sendPushNotification отправляет уведомление через FCM
func (s *server) sendPushNotification(user, title, body, roomID string) {
	if s.firebaseApp == nil {
		log.Printf("Firebase not initialized, skipping push notification to %s", user)
		return
	}

	token, err := s.db.GetUserToken(user)
	if err != nil || token == "" {
		log.Printf("No FCM token found for user %s", user)
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		log.Printf("Failed to get Firebase messaging client: %v", err)
		return
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: map[string]string{
			"title":   title,
			"body":    body,
			"room_id": roomID,
		},
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		log.Printf("Failed to send push notification to %s: %v", user, err)
		return
	}

	log.Printf("Push notification sent successfully to %s", user)
}

// DeleteMessages deletes a list of messages
func (s *server) DeleteMessages(_ context.Context, req *gen.DeleteMessagesRequest) (*gen.DeleteMessagesResponse, error) {
	var anyDeleted bool
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}

		// Try deleting by ID first if available
		if msg.Id != "" {
			// Get image URL before deletion
			imageURL, err := s.db.GetMessageImageURL(msg.Id)
			if err != nil {
				log.Printf("Failed to get image URL for message %s: %v", msg.Id, err)
			}

			// Delete image file if exists
			if imageURL != "" {
				if err := DeleteImageFile(imageURL); err != nil {
					log.Printf("Failed to delete image file for message %s: %v", msg.Id, err)
					// Continue with message deletion even if image deletion fails
				}
			}

			err = s.db.DeleteMessageByUUID(msg.Id)
			if err == nil {
				anyDeleted = true
				log.Printf("Deleted message by ID: %s", msg.Id)
				continue
			}
		}

		// Fallback to time/user match if ID fails or is missing
		targetTime := msg.CreatedAt.AsTime()
		candidates, err := s.db.GetMessagesByUserAndTime(msg.User, targetTime)
		if err != nil {
			log.Printf("Failed to find message for deletion: %v", err)
			continue
		}

		for _, candidate := range candidates {
			decryptedText, err := decrypt(candidate.Encrypted)
			if err != nil {
				continue
			}

			if decryptedText == msg.Text {
				// Delete image file if candidate has one
				if candidate.ImageURL != "" {
					if err := DeleteImageFile(candidate.ImageURL); err != nil {
						log.Printf("Failed to delete image file for candidate message: %v", err)
						// Continue with message deletion even if image deletion fails
					}
				}

				err = s.db.DeleteMessageByID(candidate.ID)
				if err == nil {
					anyDeleted = true
					log.Printf("Deleted message by content from %s", msg.User)
				}
				break
			}
		}
	}

	return &gen.DeleteMessagesResponse{Success: anyDeleted}, nil
}

// EditMessage edits a message by ID
func (s *server) EditMessage(_ context.Context, req *gen.EditMessageRequest) (*gen.EditMessageResponse, error) {
	if req.MessageId == "" {
		return &gen.EditMessageResponse{Success: false, Message: "Message ID is required"}, nil
	}

	err := s.db.UpdateMessageText(req.MessageId, req.Text)
	if err != nil {
		log.Printf("Failed to edit message %s: %v", req.MessageId, err)
		return &gen.EditMessageResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Edited message %s", req.MessageId)
	return &gen.EditMessageResponse{Success: true, Message: "Message edited successfully"}, nil
}

func (s *server) AddContact(_ context.Context, req *gen.AddContactRequest) (*gen.AddContactResponse, error) {
	err := s.db.AddContact(req.Username, req.ContactUsername)
	if err != nil {
		log.Printf("Failed to add contact %s for %s: %v", req.ContactUsername, req.Username, err)
		return &gen.AddContactResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("User %s added contact %s", req.Username, req.ContactUsername)
	return &gen.AddContactResponse{Success: true, Message: "Contact added successfully"}, nil
}

func (s *server) RemoveContact(_ context.Context, req *gen.RemoveContactRequest) (*gen.RemoveContactResponse, error) {
	err := s.db.RemoveContact(req.Username, req.ContactUsername)
	if err != nil {
		log.Printf("Failed to remove contact %s for %s: %v", req.ContactUsername, req.Username, err)
		return &gen.RemoveContactResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("User %s removed contact %s", req.Username, req.ContactUsername)
	return &gen.RemoveContactResponse{Success: true, Message: "Contact removed successfully"}, nil
}

func (s *server) GetContacts(_ context.Context, req *gen.GetContactsRequest) (*gen.GetContactsResponse, error) {
	contacts, err := s.db.GetContacts(req.Username)
	if err != nil {
		log.Printf("Failed to get contacts for %s: %v", req.Username, err)
		return &gen.GetContactsResponse{Contacts: nil}, nil
	}
	return &gen.GetContactsResponse{Contacts: contacts}, nil
}

func (s *server) GetChatListVersion(_ context.Context, req *gen.GetChatListVersionRequest) (*gen.GetChatListVersionResponse, error) {
	version, err := s.db.GetUserChatListVersion(req.Username)
	if err != nil {
		return &gen.GetChatListVersionResponse{Version: 0}, nil
	}
	return &gen.GetChatListVersionResponse{Version: version}, nil
}

func (s *server) GetThemes(_ context.Context, req *gen.GetThemesRequest) (*gen.GetThemesResponse, error) {
	currentID, themes, err := s.db.GetUserThemes(req.Username)
	if err != nil {
		log.Printf("Failed to get themes for %s: %v", req.Username, err)
		return &gen.GetThemesResponse{CurrentThemeId: "dark"}, nil
	}

	var customThemes []*gen.CustomTheme
	for _, t := range themes {
		customThemes = append(customThemes, &gen.CustomTheme{
			Id:                 t.ThemeID,
			Name:               t.Name,
			PrimaryColor:       t.PrimaryColor,
			OnPrimaryColor:     t.OnPrimaryColor,
			SurfaceColor:       t.SurfaceColor,
			OnSurfaceColor:     t.OnSurfaceColor,
			BackgroundColor:    t.BackgroundColor,
			TextPrimaryColor:   t.TextPrimaryColor,
			TextSecondaryColor: t.TextSecondaryColor,
			IsDark:             t.IsDark,
		})
	}

	return &gen.GetThemesResponse{
		CurrentThemeId: currentID,
		CustomThemes:   customThemes,
	}, nil
}

func (s *server) SaveTheme(_ context.Context, req *gen.SaveThemeRequest) (*gen.SaveThemeResponse, error) {
	err := s.db.SaveUserTheme(req.Username, req.Theme)
	if err != nil {
		log.Printf("Failed to save theme for %s: %v", req.Username, err)
		return &gen.SaveThemeResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.SaveThemeResponse{Success: true, Message: "Theme saved"}, nil
}

func (s *server) SetCurrentTheme(_ context.Context, req *gen.SetCurrentThemeRequest) (*gen.SetCurrentThemeResponse, error) {
	err := s.db.SetCurrentTheme(req.Username, req.ThemeId)
	if err != nil {
		log.Printf("Failed to set current theme for %s: %v", req.Username, err)
		return &gen.SetCurrentThemeResponse{Success: false}, nil
	}
	return &gen.SetCurrentThemeResponse{Success: true}, nil
}

func (s *server) DeleteTheme(_ context.Context, req *gen.DeleteThemeRequest) (*gen.DeleteThemeResponse, error) {
	err := s.db.DeleteUserTheme(req.Username, req.ThemeId)
	if err != nil {
		log.Printf("Failed to delete theme for %s: %v", req.Username, err)
		return &gen.DeleteThemeResponse{Success: false}, nil
	}
	return &gen.DeleteThemeResponse{Success: true}, nil
}
