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
			// Hash the password and check/create user
			passwordHash, err := HashPassword(msg.Password)
			if err != nil {
				log.Printf("Failed to hash password: %v", err)
				return err
			}

			// Check if user exists
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
					return fmt.Errorf("authentication failed")
				}
			} else {
				// New user, create with hashed password
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
		if !s.hub.IsAuthenticated(stream) && msg.Password == "" {
			log.Printf("Rejected message from unauthenticated stream")
			return fmt.Errorf("not authenticated")
		}

		// Обновляем имя пользователя в хабе, чтобы GetClients работал корректно
		s.hub.UpdateName(stream, msg.User)

		// Генерируем уникальный ID для сообщения
		msg.Id = uuid.New().String()

		// Set server timestamp for message
		msg.CreatedAt = timestamppb.Now()

		log.Printf("[%s]: %s", msg.User, msg.Text)

		// Encrypt message text before saving to database
		encryptedText, err := encrypt(msg.Text)
		if err != nil {
			log.Printf("Failed to encrypt message: %v", err)
			continue
		}

		// Save encrypted message to database with UUID
		err = s.db.SaveMessage(msg.Id, msg.User, encryptedText, msg.CreatedAt.AsTime(), msg.RepliedToMessageId, msg.RepliedToUser, msg.RepliedToText)
		if err != nil {
			log.Printf("Failed to save message to DB: %v", err)
		}

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

// GetHistory возвращает историю сообщений
func (s *server) GetHistory(ctx context.Context, req *gen.GetHistoryRequest) (*gen.GetHistoryResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	rawMessages, err := s.db.GetMessages(limit)
	if err != nil {
		log.Printf("Error fetching history: %v", err)
		return nil, err
	}

	var messages []*gen.Message
	// Проходим в обратном порядке, чтобы сообщения были от старых к новым
	for i := len(rawMessages) - 1; i >= 0; i-- {
		m := rawMessages[i]

		// Расшифровываем текст из базы
		decryptedText, err := decrypt(m.Encrypted)
		if err != nil {
			log.Printf("Failed to decrypt history message: %v", err)
			decryptedText = "[Decryption Error]"
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
