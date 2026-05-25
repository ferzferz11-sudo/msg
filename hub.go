// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the Hub for managing active client connections.
// It handles client registration, unregistration, and message broadcasting.

package main

import (
	"sync"
	"time"

	"LavenderMessenger/gen" // Generated gRPC code package
)

// Hub manages active gRPC streams for client connections
type Hub struct {
	// mu protects the clients map from concurrent access from different goroutines
	mu            sync.RWMutex
	clients       map[gen.ChatService_ChatServer]string // maps stream to username
	authenticated map[gen.ChatService_ChatServer]bool   // tracks if stream is authenticated
	rooms         map[gen.ChatService_ChatServer]string // maps stream to current room ID
	typingStreams map[gen.ChatService_TypingServer]string
	callStreams   map[gen.ChatService_CallSessionServer]string

	onStatusChange func()

	// Conferences: roomID -> participants list
	conferences map[string]*Conference
}

type Conference struct {
	CreatorID    string
	Participants map[string]string // userID -> username (currently in call)
	Invited      map[string]string // userID -> username (invited but not necessarily joined)
	Topic        string
	StartTime    time.Time
}

// NewHub creates a new Hub instance
func NewHub(onStatusChange func()) *Hub {
	return &Hub{
		clients:        make(map[gen.ChatService_ChatServer]string),
		authenticated:  make(map[gen.ChatService_ChatServer]bool),
		rooms:          make(map[gen.ChatService_ChatServer]string),
		typingStreams:  make(map[gen.ChatService_TypingServer]string),
		callStreams:    make(map[gen.ChatService_CallSessionServer]string),
		conferences:    make(map[string]*Conference),
		onStatusChange: onStatusChange,
	}
}

// RegisterCall adds a new call stream
func (h *Hub) RegisterCall(stream gen.ChatService_CallSessionServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callStreams[stream] = "Anonymous"
}

// UpdateCallName updates the username associated with a call stream
func (h *Hub) UpdateCallName(stream gen.ChatService_CallSessionServer, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callStreams[stream] = name
}

// UnregisterCall removes a call stream
func (h *Hub) UnregisterCall(stream gen.ChatService_CallSessionServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.callStreams, stream)
}

// RegisterTyping adds a new typing stream to the hub
func (h *Hub) RegisterTyping(stream gen.ChatService_TypingServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.typingStreams[stream] = ""
}

// UnregisterTyping removes a typing stream
func (h *Hub) UnregisterTyping(stream gen.ChatService_TypingServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.typingStreams, stream)
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
	oldName := h.clients[stream]
	h.clients[stream] = name
	h.mu.Unlock()

	// Only trigger status change if the name actually changed from Anonymous
	if oldName != name && h.onStatusChange != nil {
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

// BroadcastTyping sends a typing signal to all clients in the same room
func (h *Hub) BroadcastTyping(signal *gen.TypingSignal) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	roomID := signal.RoomId
	if roomID == "" {
		return
	}

	for stream := range h.typingStreams {
		// In BIDI stream we don't have a direct way to know the room without a map
		// For now we broadcast to all, and client will filter by roomId
		_ = stream.Send(signal)
	}
}

// BroadcastCall sends a call signal to the specific receiver. Returns true if delivered to at least one stream.
func (h *Hub) BroadcastCall(signal *gen.CallMessage) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	delivered := false
	for stream, username := range h.callStreams {
		if username == signal.ReceiverId {
			err := stream.Send(signal)
			if err == nil {
				delivered = true
			}
		}
	}
	return delivered
}

// BroadcastConference sends a signal to all members of a group room
func (h *Hub) BroadcastConference(signal *gen.CallMessage, roomMembers []string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Create a map for fast lookup
	memberMap := make(map[string]bool)
	for _, m := range roomMembers {
		memberMap[m] = true
	}

	for stream, username := range h.callStreams {
		// If the user is a member of the room (and not the sender, optional)
		if memberMap[username] {
			_ = stream.Send(signal)
		}
	}
}

func (h *Hub) InitiateConference(roomID, creatorID, creatorName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conferences[roomID] = &Conference{
		CreatorID: creatorID,
		Participants: map[string]string{
			creatorID: creatorName,
		},
		Invited:   make(map[string]string),
		Topic:     "",
		StartTime: time.Now(),
	}
}

func (h *Hub) UpdateConferenceMetadata(roomID, topic string, startTime time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conf, ok := h.conferences[roomID]; ok {
		conf.Topic = topic
		conf.StartTime = startTime
	}
}

func (h *Hub) InviteToConference(roomID, userID, userName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conf, ok := h.conferences[roomID]; ok {
		conf.Invited[userID] = userName
	}
}

func (h *Hub) RemoveFromConference(roomID, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conf, ok := h.conferences[roomID]; ok {
		delete(conf.Invited, userID)
	}
}

func (h *Hub) GetConferenceInvited(roomID string) map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		res := make(map[string]string)
		for k, v := range conf.Invited {
			res[k] = v
		}
		return res
	}
	return nil
}

func (h *Hub) GetConferenceTopic(roomID string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		return conf.Topic
	}
	return ""
}

func (h *Hub) GetConferenceStartTime(roomID string) time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		return conf.StartTime
	}
	return time.Time{}
}

func (h *Hub) JoinConference(roomID, userID, userName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conf, ok := h.conferences[roomID]; ok {
		conf.Participants[userID] = userName
	}
}

func (h *Hub) LeaveConference(roomID, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conf, ok := h.conferences[roomID]; ok {
		delete(conf.Participants, userID)
		if len(conf.Participants) == 0 {
			delete(h.conferences, roomID)
		}
	}
}

func (h *Hub) EndConference(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conferences, roomID)
}

func (h *Hub) GetConferenceParticipants(roomID string) map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		// Return a copy to avoid concurrent access issues
		res := make(map[string]string)
		for k, v := range conf.Participants {
			res[k] = v
		}
		return res
	}
	return nil
}

func (h *Hub) IsConferenceCreator(roomID, userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		return conf.CreatorID == userID
	}
	return false
}

func (h *Hub) GetConferenceCreator(roomID string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conf, ok := h.conferences[roomID]; ok {
		return conf.CreatorID
	}
	return ""
}
