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
	clientUserIds map[gen.ChatService_ChatServer]string // maps stream to userId (UUID)
	authenticated map[gen.ChatService_ChatServer]bool   // tracks if stream is authenticated
	rooms         map[gen.ChatService_ChatServer]string // maps stream to current room ID
	typingStreams map[gen.ChatService_TypingServer]string
	callStreams   map[gen.ChatService_CallSessionServer]string

	// ChatV2 streams
	v2Clients   map[gen.ChatService_ChatV2Server]string // stream → username
	v2UserIds   map[gen.ChatService_ChatV2Server]string // stream → userId
	v2Rooms     map[gen.ChatService_ChatV2Server]string // stream → room ID

	// Reverse-lookup sets for O(1) IsUserOnline
	userIdSet      map[string]bool   // userId → online (v2 clients)
	usernameSet    map[string]bool   // username → online (v1 clients)
	clientVersions map[string]string // userId → client version

	onStatusChange func()

	// Conferences: roomID -> participants list
	conferences map[string]*Conference

	// Grace period for reconnect: username -> disconnect timestamp
	gracePeriods map[string]time.Time
	graceMu      sync.Mutex
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
		clientUserIds:  make(map[gen.ChatService_ChatServer]string),
		authenticated:  make(map[gen.ChatService_ChatServer]bool),
		rooms:          make(map[gen.ChatService_ChatServer]string),
		typingStreams:  make(map[gen.ChatService_TypingServer]string),
		callStreams:    make(map[gen.ChatService_CallSessionServer]string),
		v2Clients:      make(map[gen.ChatService_ChatV2Server]string),
		v2UserIds:      make(map[gen.ChatService_ChatV2Server]string),
		v2Rooms:        make(map[gen.ChatService_ChatV2Server]string),
		userIdSet:      make(map[string]bool),
		usernameSet:    make(map[string]bool),
		clientVersions: make(map[string]string),
		conferences:    make(map[string]*Conference),
		onStatusChange: onStatusChange,
		gracePeriods:   make(map[string]time.Time),
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
	if oldName != "" && oldName != "Anonymous" {
		h.usernameSet[oldName] = false
		delete(h.usernameSet, oldName)
	}
	if name != "" && name != "Anonymous" {
		h.usernameSet[name] = true
	}
	h.mu.Unlock()

	// Only trigger status change if the name actually changed from Anonymous
	if oldName != name && h.onStatusChange != nil {
		h.onStatusChange()
	}
}

// SetUserId sets the userId (UUID) associated with a stream.
// Used for v2 JWT auth where userId is the primary identifier.
func (h *Hub) SetUserId(stream gen.ChatService_ChatServer, userId string) {
	h.mu.Lock()
	oldId := h.clientUserIds[stream]
	h.clientUserIds[stream] = userId
	if oldId != "" {
		h.userIdSet[oldId] = false
		delete(h.userIdSet, oldId)
	}
	if userId != "" {
		h.userIdSet[userId] = true
	}
	h.mu.Unlock()
}

// SetClientVersion stores the client version for a user
func (h *Hub) SetClientVersion(userId, version string) {
	if userId == "" || version == "" {
		return
	}
	h.mu.Lock()
	h.clientVersions[userId] = version
	h.mu.Unlock()
}

// GetClientVersion returns the client version for a user
func (h *Hub) GetClientVersion(userId string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clientVersions[userId]
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

const gracePeriodDuration = 30 * time.Second

// StartGracePeriod starts a grace period for a user after connection loss.
// During grace period the user is considered "reconnecting" not "offline".
func (h *Hub) StartGracePeriod(username string) {
	h.graceMu.Lock()
	h.gracePeriods[username] = time.Now()
	h.graceMu.Unlock()
}

// IsInGracePeriod checks if user is within reconnect grace period.
func (h *Hub) IsInGracePeriod(username string) bool {
	h.graceMu.Lock()
	defer h.graceMu.Unlock()
	if t, ok := h.gracePeriods[username]; ok {
		if time.Since(t) < gracePeriodDuration {
			return true
		}
		delete(h.gracePeriods, username)
	}
	return false
}

// ClearGracePeriod clears grace period (on successful reconnect or explicit disconnect).
func (h *Hub) ClearGracePeriod(username string) {
	h.graceMu.Lock()
	delete(h.gracePeriods, username)
	h.graceMu.Unlock()
}

// GetGracePeriodRemaining returns remaining grace period duration, or 0 if expired.
func (h *Hub) GetGracePeriodRemaining(username string) time.Duration {
	h.graceMu.Lock()
	defer h.graceMu.Unlock()
	if t, ok := h.gracePeriods[username]; ok {
		remaining := gracePeriodDuration - time.Since(t)
		if remaining > 0 {
			return remaining
		}
		delete(h.gracePeriods, username)
	}
	return 0
}

// Unregister removes a stream from the broadcast list.
// For graceful reconnect: does NOT immediately remove if user is in grace period.
// Instead starts a grace period so brief disconnects don't kill the session.
func (h *Hub) Unregister(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	username := h.clients[stream]
	userId := h.clientUserIds[stream]
	delete(h.clients, stream)
	delete(h.clientUserIds, stream)
	delete(h.authenticated, stream)
	delete(h.rooms, stream)
	if userId != "" {
		h.userIdSet[userId] = false
		delete(h.userIdSet, userId)
		delete(h.clientVersions, userId)
	}
	if username != "" && username != "Anonymous" {
		h.usernameSet[username] = false
		delete(h.usernameSet, username)
	}
	h.mu.Unlock()

	// If user was authenticated, start grace period for reconnect
	// This allows the client to reconnect within 30s without losing session state
	if username != "" && username != "Anonymous" {
		h.StartGracePeriod(username)
	}

	if h.onStatusChange != nil {
		h.onStatusChange()
	}
}

// RegisterV2 registers a ChatV2 stream
func (h *Hub) RegisterV2(stream gen.ChatService_ChatV2Server) {
	h.mu.Lock()
	h.v2Clients[stream] = "Anonymous"
	h.mu.Unlock()
}

// UnregisterV2 unregisters a ChatV2 stream
func (h *Hub) UnregisterV2(stream gen.ChatService_ChatV2Server) {
	h.mu.Lock()
	username := h.v2Clients[stream]
	userId := h.v2UserIds[stream]
	delete(h.v2Clients, stream)
	delete(h.v2UserIds, stream)
	delete(h.v2Rooms, stream)
	if userId != "" {
		h.userIdSet[userId] = false
		delete(h.userIdSet, userId)
	}
	if username != "" && username != "Anonymous" {
		h.usernameSet[username] = false
		delete(h.usernameSet, username)
	}
	h.mu.Unlock()

	if username != "" && username != "Anonymous" {
		h.StartGracePeriod(username)
	}
	if h.onStatusChange != nil {
		h.onStatusChange()
	}
}

// SetV2Room sets the room for a ChatV2 stream
func (h *Hub) SetV2Room(stream gen.ChatService_ChatV2Server, room string) {
	h.mu.Lock()
	h.v2Rooms[stream] = room
	h.mu.Unlock()
}

// SetV2UserId sets the userId for a ChatV2 stream
func (h *Hub) SetV2UserId(stream gen.ChatService_ChatV2Server, userId string) {
	h.mu.Lock()
	oldId := h.v2UserIds[stream]
	h.v2UserIds[stream] = userId
	if oldId != "" {
		h.userIdSet[oldId] = false
		delete(h.userIdSet, oldId)
	}
	if userId != "" {
		h.userIdSet[userId] = true
	}
	h.mu.Unlock()
}

// SetV2Username sets the username for a ChatV2 stream
func (h *Hub) SetV2Username(stream gen.ChatService_ChatV2Server, name string) {
	h.mu.Lock()
	oldName := h.v2Clients[stream]
	h.v2Clients[stream] = name
	if oldName != "" && oldName != "Anonymous" {
		h.usernameSet[oldName] = false
		delete(h.usernameSet, oldName)
	}
	if name != "" && name != "Anonymous" {
		h.usernameSet[name] = true
	}
	h.mu.Unlock()
}

// SnapshotRoomStreams returns a copy of all ChatV2 streams in a room for safe sending.
func (h *Hub) SnapshotRoomStreams(roomID string) []gen.ChatService_ChatV2Server {
	h.mu.RLock()
	var targets []gen.ChatService_ChatV2Server
	for stream, room := range h.v2Rooms {
		if room == roomID {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()
	return targets
}

// GetOnlineUsers returns a list of unique usernames currently connected.
// Includes users in grace period (reconnecting) so they appear online during brief disconnects.
func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	userMap := make(map[string]struct{})
	for _, name := range h.clients {
		if name != "" && name != "Anonymous" {
			userMap[name] = struct{}{}
		}
	}
	// Also include v2 clients
	for _, name := range h.v2Clients {
		if name != "" && name != "Anonymous" {
			userMap[name] = struct{}{}
		}
	}
	h.mu.RUnlock()

	// Also include users in grace period (reconnecting) — but only if not expired
	h.graceMu.Lock()
	for username, t := range h.gracePeriods {
		if time.Since(t) < gracePeriodDuration {
			userMap[username] = struct{}{}
		} else {
			delete(h.gracePeriods, username)
		}
	}
	h.graceMu.Unlock()

	var users []string
	for name := range userMap {
		users = append(users, name)
	}
	return users
}

// IsUserOnline checks if a user currently has an active gRPC stream.
// userId is the primary identifier (v2 JWT auth).
// Deprecated: username is used as fallback for legacy v1 clients. Will be removed when all clients use v2.
// Used by push notification logic to avoid sending push to online users.
func (h *Hub) IsUserOnline(userId, username string) bool {
	h.mu.RLock()
	// O(1) check by userId (v2 clients)
	if userId != "" && h.userIdSet[userId] {
		h.mu.RUnlock()
		return true
	}
	// O(1) fallback: check by username (v1 clients)
	if username != "" && h.usernameSet[username] {
		h.mu.RUnlock()
		return true
	}
	h.mu.RUnlock()

	// Also check grace period (user may be reconnecting)
	h.graceMu.Lock()
	t, exists := h.gracePeriods[username]
	h.graceMu.Unlock()
	if exists && time.Since(t) < gracePeriodDuration {
		return true
	}

	return false
}

// GetOnlineUserSet returns a map of userId -> true for all currently connected users.
// Used by admin panel for real-time online status.
func (h *Hub) GetOnlineUserSet() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	online := make(map[string]bool)
	for uid := range h.userIdSet {
		if uid != "" {
			online[uid] = true
		}
	}
	for username := range h.usernameSet {
		if username != "" {
			online[username] = true
		}
	}
	return online
}

// BroadcastV2Reaction sends a REACTION_V2 system message to all ChatV2 clients in the same room.
// payload format: "messageID|reactionsJSON"
func (h *Hub) BroadcastV2Reaction(roomID, messageID, reactionsJSON string) {
	if roomID == "" || messageID == "" {
		return
	}

	h.mu.RLock()
	var targets []gen.ChatService_ChatV2Server
	for stream, room := range h.v2Rooms {
		if room == roomID {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()

	payload := messageID + "|" + reactionsJSON
	wrappedMsg := &gen.ChatV2Message{
		Payload: &gen.ChatV2Message_System{
			System: &gen.ChatV2System{
				Type:    "REACTION_V2",
				Message: payload,
			},
		},
	}

	for _, stream := range targets {
		_ = stream.Send(wrappedMsg)
	}
}

// BroadcastGlobal sends a message to all connected and authenticated clients
func (h *Hub) BroadcastGlobal(msg *gen.Message) {
	h.mu.RLock()
	var targets []gen.ChatService_ChatServer
	for stream, auth := range h.authenticated {
		if auth {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()

	for _, stream := range targets {
		_ = stream.Send(msg)
	}
}

// BroadcastShutdown sends SERVER_SHUTTINGDOWN to all connected clients
func (h *Hub) BroadcastShutdown() {
	msg := &gen.Message{
		User:   "SYSTEM",
		Text:   "SERVER_SHUTTINGDOWN",
		RoomId: "",
	}
	h.BroadcastGlobal(msg)
}

// Broadcast sends a message to all connected clients in the same room
func (h *Hub) Broadcast(msg *gen.Message) {
	roomID := msg.RoomId
	if roomID == "" {
		return
	}

	// Snapshot streams under lock, then send without lock
	h.mu.RLock()
	var targets []gen.ChatService_ChatServer
	for stream := range h.clients {
		if h.rooms[stream] == roomID {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()

	for _, stream := range targets {
		if err := stream.Send(msg); err != nil {
			continue
		}
	}
}

// BroadcastTyping sends a typing signal to all clients in the same room
func (h *Hub) BroadcastTyping(signal *gen.TypingSignal) {
	roomID := signal.RoomId
	if roomID == "" {
		return
	}

	h.mu.RLock()
	var targets []gen.ChatService_TypingServer
	for stream := range h.typingStreams {
		targets = append(targets, stream)
	}
	h.mu.RUnlock()

	for _, stream := range targets {
		_ = stream.Send(signal)
	}
}

// BroadcastCall sends a call signal to the specific receiver. Returns true if delivered to at least one stream.
func (h *Hub) BroadcastCall(signal *gen.CallMessage) bool {
	h.mu.RLock()
	var targets []gen.ChatService_CallSessionServer
	for stream, username := range h.callStreams {
		if username == signal.ReceiverId || username == signal.ReceiverName {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()

	delivered := false
	for _, stream := range targets {
		if err := stream.Send(signal); err == nil {
			delivered = true
		}
	}
	return delivered
}

// BroadcastConference sends a signal to all members of a group room
func (h *Hub) BroadcastConference(signal *gen.CallMessage, roomMembers []string) {
	memberMap := make(map[string]bool)
	for _, m := range roomMembers {
		memberMap[m] = true
	}

	h.mu.RLock()
	var targets []gen.ChatService_CallSessionServer
	for stream, username := range h.callStreams {
		if memberMap[username] {
			targets = append(targets, stream)
		}
	}
	h.mu.RUnlock()

	for _, stream := range targets {
		_ = stream.Send(signal)
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
