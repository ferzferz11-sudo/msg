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
	mu            sync.RWMutex
	clients       map[gen.ChatService_ChatServer]string // maps stream to username
	authenticated map[gen.ChatService_ChatServer]bool   // tracks if stream is authenticated
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[gen.ChatService_ChatServer]string),
		authenticated: make(map[gen.ChatService_ChatServer]bool),
	}
}

// Register adds a new stream (client) to the broadcast list
func (h *Hub) Register(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	h.clients[stream] = "Anonymous"
	h.authenticated[stream] = false
	h.mu.Unlock()
}

// UpdateName updates the username associated with a stream
func (h *Hub) UpdateName(stream gen.ChatService_ChatServer, name string) {
	h.mu.Lock()
	h.clients[stream] = name
	h.mu.Unlock()
}

// SetAuthenticated marks a stream as authenticated
func (h *Hub) SetAuthenticated(stream gen.ChatService_ChatServer, auth bool) {
	h.mu.Lock()
	h.authenticated[stream] = auth
	h.mu.Unlock()
}

// IsAuthenticated checks if a stream is authenticated
func (h *Hub) IsAuthenticated(stream gen.ChatService_ChatServer) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.authenticated[stream]
}

// Unregister removes a stream from the broadcast list
func (h *Hub) Unregister(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	delete(h.clients, stream)
	delete(h.authenticated, stream)
	h.mu.Unlock()
}

// GetOnlineUsers returns a list of unique usernames currently connected
func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userMap := make(map[string]struct{})
	for _, name := range h.clients {
		userMap[name] = struct{}{}
	}

	var users []string
	for name := range userMap {
		users = append(users, name)
	}
	return users
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
