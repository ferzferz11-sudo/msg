// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the Hub for managing active client connections.
// It handles client registration, unregistration, and message broadcasting.

package main

import (
	"sync"

	"LavenderMessenger/gen" // Generated gRPC code package
)

// Hub manages active gRPC streams for client connections
type Hub struct {
	// mu protects the clients map from concurrent access from different goroutines
	mu      sync.RWMutex
	clients map[gen.ChatService_ChatServer]struct{}
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients: make(map[gen.ChatService_ChatServer]struct{}),
	}
}

// Register adds a new stream (client) to the broadcast list
func (h *Hub) Register(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	h.clients[stream] = struct{}{}
	h.mu.Unlock()
}

// Unregister removes a stream from the broadcast list
func (h *Hub) Unregister(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	delete(h.clients, stream)
	h.mu.Unlock()
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msg *gen.Message) {
	h.mu.RLock()
	// Release the read lock at the end of the function
	defer h.mu.RUnlock()

	for stream := range h.clients {
		// Send message to gRPC stream
		err := stream.Send(msg)
		if err != nil {
			// If sending failed (client disconnected),
			// the removal logic is better handled by defer in the server's Chat method
			continue
		}
	}
}
