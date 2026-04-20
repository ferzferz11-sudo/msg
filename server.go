// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.

package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// server implements the gRPC ChatService interface
type server struct {
	gen.UnimplementedChatServiceServer
	hub *Hub // Hub for managing client connections
	db  *DB  // Database for message persistence
}

// Chat handles bidirectional streaming for real-time messaging
func (s *server) Chat(stream gen.ChatService_ChatServer) error {
	// Register the new client stream with the hub
	s.hub.Register(stream)
	defer func() {
		// Unregister the client when the connection ends
		s.hub.Unregister(stream)
		log.Println("Client disconnected")
	}()

	log.Println("New client connected")

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

		log.Printf("[%s]: %s", msg.User, msg.Text)

		// Skip empty messages
		if strings.TrimSpace(msg.Text) == "" {
			log.Printf("Skipping empty message from %s", msg.User)
			continue
		}

		// Determine room ID
		roomID := msg.RoomId
		if roomID == "" {
			roomID = "general"
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
			err = s.db.SaveMessage(msg.Id, msg.User, encryptedText, msg.CreatedAt.AsTime(), msg.RepliedToMessageId, msg.RepliedToUser, msg.RepliedToText, roomID)
			if err != nil {
				log.Printf("Failed to save message to DB: %v", err)
			} else {
				log.Printf("Message saved to DB successfully: %s in room %s", msg.Id, roomID)
			}
		}

		// Update user's current room in hub
		s.hub.SetRoom(stream, roomID)

		// Clear password and reply fields from message before broadcasting
		msg.Password = ""
		// Keep replied_to fields for display to clients

		// Broadcast message to all connected clients
		s.hub.Broadcast(msg)
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
		roomID = "general"
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

		// Check if decrypted text is empty
		if decryptedText == "" {
			log.Printf("Warning: message %s decrypted to empty string", m.MessageID)
			continue // Skip empty messages
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
	log.Printf("Registered FCM token for user: %s", req.User)
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
			Id:           c.ID,
			Name:         c.Name,
			Type:         c.Type,
			Participants: c.Participants,
			CreatedAt:    timestamppb.New(c.CreatedAt),
			UnreadCount:  int32(c.UnreadCount),
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

// sendPushNotification отправляет уведомление через FCM
func (s *server) sendPushNotification(user, title, body string) {
	token, err := s.db.GetUserToken(user)
	if err != nil || token == "" {
		return
	}

	// Здесь будет логика отправки HTTP запроса к Firebase
	// Для работы нужно будет добавить FIREBASE_SERVER_KEY в .env
	log.Printf("Sending push to %s (token: %s...) - Title: %s, Body: %s", user, token[:10], title, body)
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
			err := s.db.DeleteMessageByUUID(msg.Id)
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
