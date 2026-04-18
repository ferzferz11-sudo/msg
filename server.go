// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.

package main

import (
	"LavenderMessenger/gen"
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
			// Client closed the connection normally
			return nil
		}
		if err != nil {
			// Connection error occurred
			return err
		}

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
