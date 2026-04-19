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

		// Set server timestamp for message
		msg.CreatedAt = timestamppb.Now()

		log.Printf("[%s]: %s", msg.User, msg.Text)

		// Encrypt message text before saving to database
		encryptedText, err := encrypt(msg.Text)
		if err != nil {
			log.Printf("Failed to encrypt message: %v", err)
			continue
		}

		// Save encrypted message to database
		err = s.db.SaveMessage(msg.User, encryptedText, msg.CreatedAt.AsTime())
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

		messages = append(messages, &gen.Message{
			User:      m.Username,
			Text:      decryptedText,
			CreatedAt: timestamppb.New(m.CreatedAt),
		})
	}

	return &gen.GetHistoryResponse{
		Messages: messages,
	}, nil
}
