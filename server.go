// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.

package main

import (
	"LavenderMessenger/gen"
	"context"
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
		err = s.db.SaveMessage(msg.Id, msg.User, encryptedText, msg.CreatedAt.AsTime())
		if err != nil {
			log.Printf("Failed to save message to DB: %v", err)
		}

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
			Id:        m.MessageID,
			User:      m.Username,
			Text:      decryptedText,
			CreatedAt: timestamppb.New(m.CreatedAt),
			Reactions: reactions,
		})
	}

	return &gen.GetHistoryResponse{
		Messages: messages,
	}, nil
}

// SetReaction saves or updates a reaction and broadcasts it
func (s *server) SetReaction(ctx context.Context, req *gen.ReactionRequest) (*gen.ReactionResponse, error) {
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

// DeleteMessages deletes a list of messages
func (s *server) DeleteMessages(ctx context.Context, req *gen.DeleteMessagesRequest) (*gen.DeleteMessagesResponse, error) {
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
