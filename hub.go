// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the Hub for managing active client connections.
// It handles client registration, unregistration, and message broadcasting.
// v2-only: all v1 Chat/Typing streams removed.

package main

import (
	"sync"
	"time"

	"LavenderMessenger/gen"
)

// Hub manages active gRPC streams for client connections
type Hub struct {
	mu          sync.RWMutex
	callStreams map[gen.ChatService_CallSessionServer]string

	// ChatV2 streams
	v2Clients map[gen.ChatService_ChatV2Server]string // stream → username
	v2UserIds map[gen.ChatService_ChatV2Server]string // stream → userId
	v2Rooms   map[gen.ChatService_ChatV2Server]string // stream → room ID

	// Reverse-lookup sets for O(1) IsUserOnline
	userIdSet      map[string]bool
	usernameSet    map[string]bool
	clientVersions map[string]string

	onStatusChange func()

	// Conferences: roomID -> participants list
	conferences map[string]*Conference

	// Grace period for reconnect: username -> disconnect timestamp
	gracePeriods map[string]time.Time
	graceMu      sync.Mutex
}

type Conference struct {
	CreatorID    string
	Participants map[string]string
	Invited      map[string]string
	Topic        string
	StartTime    time.Time
}

func NewHub(onStatusChange func()) *Hub {
	return &Hub{
		callStreams:    make(map[gen.ChatService_CallSessionServer]string),
		v2Clients:     make(map[gen.ChatService_ChatV2Server]string),
		v2UserIds:     make(map[gen.ChatService_ChatV2Server]string),
		v2Rooms:       make(map[gen.ChatService_ChatV2Server]string),
		userIdSet:     make(map[string]bool),
		usernameSet:   make(map[string]bool),
		clientVersions: make(map[string]string),
		conferences:   make(map[string]*Conference),
		onStatusChange: onStatusChange,
		gracePeriods:  make(map[string]time.Time),
	}
}

// --- Call streams ---

func (h *Hub) RegisterCall(stream gen.ChatService_CallSessionServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callStreams[stream] = "Anonymous"
}

func (h *Hub) UpdateCallName(stream gen.ChatService_CallSessionServer, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callStreams[stream] = name
}

func (h *Hub) UnregisterCall(stream gen.ChatService_CallSessionServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.callStreams, stream)
}

// --- Client version tracking ---

func (h *Hub) SetClientVersion(userId, version string) {
	if userId == "" || version == "" {
		return
	}
	h.mu.Lock()
	h.clientVersions[userId] = version
	h.mu.Unlock()
}

func (h *Hub) GetClientVersion(userId string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clientVersions[userId]
}

// --- Grace period ---

const gracePeriodDuration = 30 * time.Second

func (h *Hub) StartGracePeriod(username string) {
	h.graceMu.Lock()
	h.gracePeriods[username] = time.Now()
	h.graceMu.Unlock()
}

func (h *Hub) ClearGracePeriod(username string) {
	h.graceMu.Lock()
	delete(h.gracePeriods, username)
	h.graceMu.Unlock()
}

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

// --- ChatV2 streams ---

func (h *Hub) RegisterV2(stream gen.ChatService_ChatV2Server) {
	h.mu.Lock()
	h.v2Clients[stream] = "Anonymous"
	h.mu.Unlock()
}

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

func (h *Hub) SetV2Room(stream gen.ChatService_ChatV2Server, room string) {
	h.mu.Lock()
	h.v2Rooms[stream] = room
	h.mu.Unlock()
}

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

// --- Online status ---

func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	userMap := make(map[string]struct{})
	for _, name := range h.v2Clients {
		if name != "" && name != "Anonymous" {
			userMap[name] = struct{}{}
		}
	}
	h.mu.RUnlock()

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

func (h *Hub) IsUserOnline(userId, username string) bool {
	h.mu.RLock()
	if userId != "" && h.userIdSet[userId] {
		h.mu.RUnlock()
		return true
	}
	if username != "" && h.usernameSet[username] {
		h.mu.RUnlock()
		return true
	}
	h.mu.RUnlock()

	h.graceMu.Lock()
	t, exists := h.gracePeriods[username]
	h.graceMu.Unlock()
	if exists && time.Since(t) < gracePeriodDuration {
		return true
	}

	return false
}

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

// --- Broadcast (v2 only) ---

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

// BroadcastGlobalV2 sends a system message to all connected ChatV2 clients.
func (h *Hub) BroadcastGlobalV2(systemType, systemMessage string) {
	h.mu.RLock()
	var targets []gen.ChatService_ChatV2Server
	for stream := range h.v2Clients {
		targets = append(targets, stream)
	}
	h.mu.RUnlock()

	wrappedMsg := &gen.ChatV2Message{
		Payload: &gen.ChatV2Message_System{
			System: &gen.ChatV2System{
				Type:    systemType,
				Message: systemMessage,
			},
		},
	}

	for _, stream := range targets {
		_ = stream.Send(wrappedMsg)
	}
}

// BroadcastShutdown sends SERVER_SHUTTINGDOWN to all connected clients
func (h *Hub) BroadcastShutdown() {
	h.BroadcastGlobalV2("SERVER_SHUTTINGDOWN", "")
}

// BroadcastToRoom sends a system message to all ChatV2 clients in a room
func (h *Hub) BroadcastToRoom(roomID, systemType, systemMessage string) {
	if roomID == "" {
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

	wrappedMsg := &gen.ChatV2Message{
		Payload: &gen.ChatV2Message_System{
			System: &gen.ChatV2System{
				Type:    systemType,
				Message: systemMessage,
			},
		},
	}

	for _, stream := range targets {
		_ = stream.Send(wrappedMsg)
	}
}

// --- Call broadcasting ---

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

// --- Conferences ---

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
