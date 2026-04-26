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
	rooms         map[gen.ChatService_ChatServer]string // maps stream to current room ID

	// onStatusChange is a callback triggered when user list changes
	onStatusChange func()
}

// NewHub creates a new Hub instance
func NewHub(onStatusChange func()) *Hub {
	return &Hub{
		clients:        make(map[gen.ChatService_ChatServer]string),
		authenticated:  make(map[gen.ChatService_ChatServer]bool),
		rooms:          make(map[gen.ChatService_ChatServer]string),
		onStatusChange: onStatusChange,
	}
}

// Register adds a new stream (client) to the broadcast list
func (h *Hub) Register(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	h.clients[stream] = "Anonymous"
	h.authenticated[stream] = false
	h.rooms[stream] = ""
	h.mu.Unlock()
	if h.onStatusChange != nil {
		h.onStatusChange()
	}
}

// UpdateName updates the username associated with a stream
func (h *Hub) UpdateName(stream gen.ChatService_ChatServer, name string) {
	h.mu.Lock()
	h.clients[stream] = name
	h.mu.Unlock()
	if h.onStatusChange != nil {
		h.onStatusChange()
	}
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

// SetRoom updates the room ID for a stream
func (h *Hub) SetRoom(stream gen.ChatService_ChatServer, roomID string) {
	h.mu.Lock()
	h.rooms[stream] = roomID
	h.mu.Unlock()
}

// GetRoom returns the current room ID for a stream
func (h *Hub) GetRoom(stream gen.ChatService_ChatServer) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[stream]
}

// Unregister removes a stream from the broadcast list
func (h *Hub) Unregister(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	delete(h.clients, stream)
	delete(h.authenticated, stream)
	delete(h.rooms, stream)
	h.mu.Unlock()
	if h.onStatusChange != nil {
		h.onStatusChange()
	}
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

// BroadcastGlobal sends a message to all connected and authenticated clients
func (h *Hub) BroadcastGlobal(msg *gen.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for stream, auth := range h.authenticated {
		if auth {
			_ = stream.Send(msg)
		}
	}
}

// Broadcast sends a message to all connected clients in the same room
func (h *Hub) Broadcast(msg *gen.Message) {
	h.mu.RLock()
	// Release the read lock at the end of the function
	defer h.mu.RUnlock()

	roomID := msg.RoomId
	if roomID == "" {
		return
	}

	for stream := range h.clients {
		// Only send to clients in the same room
		if h.rooms[stream] == roomID {
			err := stream.Send(msg)
			if err != nil {
				// If sending failed (client disconnected),
				// the removal logic is better handled by defer in the server's Chat method
				continue
			}
		}
	}
}
