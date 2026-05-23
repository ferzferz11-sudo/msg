// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements the gRPC server for the Lavender Messenger.
// It handles client connections, message broadcasting, and encryption.

package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

const ServerVersion = "1.0.6.11"

// server implements the gRPC ChatService interface
type server struct {
	gen.UnimplementedChatServiceServer
	hub          *Hub          // Hub for managing client connections
	db           *DB           // Database for message persistence
	firebaseApp  *firebase.App // Firebase Admin SDK instance
	recentMsgs   sync.Map      // Cache for deduplicating identical rapid messages
	recentErrors sync.Map      // map[string]time.Time to prevent duplicate error logs
	fcmLogs      []*gen.FCMLogEntry
	fcmLogsMu    sync.Mutex
}

func (s *server) logErrorOnce(key string, format string, v ...interface{}) {
	now := time.Now()
	if last, ok := s.recentErrors.Load(key); ok {
		if now.Sub(last.(time.Time)) < 30*time.Second {
			return
		}
	}
	s.recentErrors.Store(key, now)
	log.Printf(format, v...)
}

func (s *server) logFCM(level, format string, v ...interface{}) {
	s.fcmLogsMu.Lock()
	defer s.fcmLogsMu.Unlock()

	msg := fmt.Sprintf(format, v...)
	entry := &gen.FCMLogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   msg,
	}
	s.fcmLogs = append(s.fcmLogs, entry)
	if len(s.fcmLogs) > 100 {
		s.fcmLogs = s.fcmLogs[1:]
	}
	log.Printf("[FCM %s] %s", level, msg)
}

// Chat handles bidirectional streaming for real-time messaging
func (s *server) Chat(stream gen.ChatService_ChatServer) error {
	var connectedUser string = "Anonymous"
	var connectedUserID string = ""
	var currentRoom string = ""

	// Register the new client stream with the hub
	s.hub.Register(stream)
	defer func() {
		// Unregister the client when the connection ends
		s.hub.Unregister(stream)
		log.Printf("Stream for %s closed", connectedUser)
	}()

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
			// Trim username to avoid whitespace issues
			trimmedUser := strings.TrimSpace(msg.User)
			msg.User = trimmedUser
			connectedUser = msg.User

			// Check if user exists first
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
					log.Printf("Auth failed: %s", msg.User)

					// Send authentication failure message to the client
					authFailMsg := &gen.Message{
						User:      "SYSTEM",
						Text:      "AUTH_FAILED",
						Id:        uuid.New().String(),
						CreatedAt: timestamppb.Now(),
					}
					if err := stream.Send(authFailMsg); err != nil {
						log.Printf("Failed to send auth failed message: %v", err)
					}

					return fmt.Errorf("authentication failed")
				}
			} else {
				// User does not exist. Check if registration is requested.
				if !msg.Register {
					log.Printf("Login attempt for non-existent user: %s", msg.User)

					// Send user not found message to the client
					notFoundMsg := &gen.Message{
						User:      "SYSTEM",
						Text:      "USER_NOT_FOUND",
						Id:        uuid.New().String(),
						CreatedAt: timestamppb.Now(),
					}
					if err := stream.Send(notFoundMsg); err != nil {
						log.Printf("Failed to send user not found message: %v", err)
					}
					return fmt.Errorf("user not found")
				}

				// New user, hash password and create
				passwordHash, err := HashPassword(msg.Password)
				if err != nil {
					log.Printf("Failed to hash password: %v", err)
					return err
				}

				err = s.db.SaveUser(msg.User, passwordHash)
				if err != nil {
					log.Printf("Failed to save user: %v", err)
					return err
				}
				log.Printf("Registered new user: %s", msg.User)

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

			// IMPORTANT: Update name in hub BEFORE broadcasting online status
			// so the list contains the actual username instead of "Anonymous"
			s.hub.UpdateName(stream, msg.User)
			s.hub.SetAuthenticated(stream, true)
			connectedUser = msg.User

			// Fetch and store user ID for session tracking
			uid, _ := s.db.GetUserIdByUsername(msg.User)
			connectedUserID = uid

			// Single unified log line for successful auth and initial connection
			log.Printf("Auth success: %s (v%s), initial signal: %s", msg.User, msg.ClientVersion, msg.RoomId)

			// Update last client version and last seen timestamp in DB
			if msg.ClientVersion != "" {
				_ = s.db.UpdateClientVersion(msg.User, msg.ClientVersion)
			} else {
				// Old client - still update last seen
				_ = s.db.UpdateLastSeen(msg.User)
			}

			// Send server info
			serverInfoMsg := &gen.Message{
				User:      "SYSTEM",
				Text:      "SERVER_INFO:" + ServerVersion,
				Id:        uuid.New().String(),
				CreatedAt: timestamppb.Now(),
			}
			_ = stream.Send(serverInfoMsg)

			// Inform the user about their admin status
			if s.db.IsSuperAdmin(msg.User) {
				statusMsg := &gen.Message{
					User:         "SYSTEM",
					Text:         "SET_SUPER_ADMIN",
					IsSuperAdmin: true,
					Id:           uuid.New().String(),
					CreatedAt:    timestamppb.Now(),
				}
				if err := stream.Send(statusMsg); err != nil {
					log.Printf("Failed to send super admin status: %v", err)
				}
			}

			// Register device session
			if msg.DeviceId != "" && connectedUserID != "" {
				ip := "unknown"
				if p, ok := peer.FromContext(stream.Context()); ok {
					ip = p.Addr.String()
				}
				err := s.db.AddUserDevice(connectedUserID, msg.DeviceId, msg.DeviceName, msg.ClientVersion, ip)
				if err != nil {
					log.Printf("Failed to register device %s for %s (ID: %s): %v", msg.DeviceId, msg.User, connectedUserID, err)
				} else {
					log.Printf("Device registered: %s (%s) for %s", msg.DeviceName, msg.DeviceId, msg.User)
				}
			}

			// Clear password from message before broadcasting
			msg.Password = ""
		}

		// Deduplication logic for identical rapid messages
		// Prevents double-posting from some source apps (e.g. Google Photos)
		if msg.Text != "" || msg.ImageUrl != "" {
			msgHash := fmt.Sprintf("%s:%s:%s:%s", msg.User, msg.RoomId, msg.Text, msg.ImageUrl)
			now := time.Now()
			if lastTime, ok := s.recentMsgs.Load(msgHash); ok {
				if now.Sub(lastTime.(time.Time)) < 2*time.Second {
					log.Printf("Msg deduplicated: %s in %s", msg.User, msg.RoomId)
					continue
				}
			}
			s.recentMsgs.Store(msgHash, now)
		}

		// Reject messages from unauthenticated streams (except first auth message)
		// Temporarily disabled to debug authentication issues
		/*if !s.hub.IsAuthenticated(stream) && msg.Password == "" {
			log.Printf("Rejected message from unauthenticated stream")
			return fmt.Errorf("not authenticated")
		}*/

		// Генерируем уникальный ID для сообщения, если его нет
		if msg.Id == "" {
			msg.Id = uuid.New().String()
		}

		// Set server timestamp for message
		msg.CreatedAt = timestamppb.Now()

		// Trim username to avoid whitespace issues
		trimmedUser := strings.TrimSpace(msg.User)
		msg.User = trimmedUser

		// Determine room ID
		roomID := msg.RoomId
		if roomID != "" && roomID != currentRoom {
			currentRoom = roomID
			s.hub.SetRoom(stream, roomID)
		}

		// Update device status if info is provided (periodic heartbeat)
		if msg.DeviceId != "" && s.hub.IsAuthenticated(stream) && connectedUserID != "" {
			ip := "unknown"
			if p, ok := peer.FromContext(stream.Context()); ok {
				ip = p.Addr.String()
			}
			_ = s.db.AddUserDevice(connectedUserID, msg.DeviceId, msg.DeviceName, msg.ClientVersion, ip)
		}

		// Skip empty messages (unless they have an image or voice)
		if strings.TrimSpace(msg.Text) == "" && msg.ImageUrl == "" && len(msg.ImageUrls) == 0 && msg.VoiceUrl == "" {
			// Don't log empty messages if they are just room switches (which we now log on auth)
			continue
		}

		if len(msg.ImageUrls) > 0 {
			log.Printf("[%s] in %s: %s (ImageURLs: %v)", msg.User, roomID, msg.Text, msg.ImageUrls)
		} else if msg.ImageUrl != "" {
			log.Printf("[%s] in %s: %s (ImageURL: %s)", msg.User, roomID, msg.Text, msg.ImageUrl)
		} else if msg.VoiceUrl != "" {
			log.Printf("[%s] in %s: Voice message (%d seconds) - %s", msg.User, roomID, msg.Duration, msg.VoiceUrl)
		} else {
			log.Printf("[%s] in %s: %s", msg.User, roomID, msg.Text)
		}

		if roomID == "" {
			log.Printf("Skipping message with empty room ID from %s", msg.User)
			continue
		}

		// Skip join messages (don't save to database)
		if strings.HasSuffix(msg.Text, " joined") || strings.HasSuffix(msg.Text, " присоединился") {
			// Still broadcast but don't save to DB
			log.Printf("Skipping join message from DB save: %s", msg.Text)
		} else {
			// Encrypt message text before saving to database
			encryptedText, err := encrypt(msg.Text)
			if err != nil {
				log.Printf("Failed to encrypt message: %v", err)
				continue
			}

			// Save encrypted message to database with UUID
			imageURL := msg.ImageUrl
			imageURLsJSON := "[]"
			if len(msg.ImageUrls) > 0 {
				imageURLsBytes, _ := json.Marshal(msg.ImageUrls)
				imageURLsJSON = string(imageURLsBytes)
			}
			voiceURL := msg.VoiceUrl
			duration := msg.Duration
			err = s.db.SaveMessage(msg.Id, msg.User, msg.UserId, encryptedText, msg.CreatedAt.AsTime(), msg.RepliedToMessageId, msg.RepliedToUser, msg.RepliedToText, roomID, imageURL, imageURLsJSON, voiceURL, duration)
			if err != nil {
				log.Printf("Failed to save msg: %v", err)
			} else {
				log.Printf("Msg saved: %s (%s)", msg.Id, roomID)
			}
		}

		// Update user's current room in hub
		s.hub.SetRoom(stream, roomID)

		// Get user's avatar URL
		avatarURL, err := s.db.GetUserAvatar(msg.User)
		if err != nil {
			log.Printf("Failed to get avatar for %s: %v", msg.User, err)
		}
		msg.AvatarUrl = avatarURL

		// Clear password and reply fields from message before broadcasting
		msg.Password = ""
		// Keep replied_to fields for display to clients

		// Broadcast message to all connected clients
		s.hub.Broadcast(msg)

		// Send push notification to all users in the room (except sender)
		// This ensures users in background receive notifications

		// CRITICAL: Check if sender wants to notify others
		senderNotifiesOthers := s.db.GetUserPushStatus(msg.User)

		allUsers, err := s.db.GetAllUsers()
		if err != nil {
			log.Printf("Failed to get all users for push notifications: %v", err)
		} else {
			for _, user := range allUsers {
				// Skip the sender
				if user.Username == msg.User {
					continue
				}

				if !senderNotifiesOthers {
					log.Printf("Push skip: %s has disabled outgoing notifications", msg.User)
					break // No need to check other participants if sender disabled it
				}

				// Check if user is a participant in the room
				userInRoom := false
				chat, err := s.db.GetChat(roomID)
				if err == nil {
					var participants []string
					json.Unmarshal([]byte(chat.Participants), &participants)
					for _, p := range participants {
						if p == user.Username {
							userInRoom = true
							break
						}
					}
				}

				if !userInRoom {
					continue
				}

				// Send push notification to all users in the room
				log.Printf("Push sent: %s", user.Username)
				s.sendPushNotification(user.Username, msg.User, msg.Text, roomID)
			}
		}
	}
}

func (s *server) Typing(stream gen.ChatService_TypingServer) error {
	var currentTypingUser string
	var currentRoomID string

	s.hub.RegisterTyping(stream)
	defer func() {
		if currentTypingUser != "" && currentRoomID != "" {
			// Send "stopped typing" signal on disconnect
			s.hub.BroadcastTyping(&gen.TypingSignal{
				RoomId:   currentRoomID,
				Username: currentTypingUser,
				IsTyping: false,
			})
		}
		s.hub.UnregisterTyping(stream)
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		currentTypingUser = req.Username
		currentRoomID = req.RoomId

		// Broadcast typing signal to others
		signal := &gen.TypingSignal{
			RoomId:   req.RoomId,
			Username: req.Username,
			IsTyping: req.IsTyping,
		}
		s.hub.BroadcastTyping(signal)
	}
}

// resolveUserId converts a potential username to a user ID if needed
func (s *server) resolveUserId(identifier string) string {
	if identifier == "" {
		return ""
	}
	// Check if it's a UUID
	if _, err := uuid.Parse(identifier); err == nil {
		return identifier
	}
	// It's a username, try to get the ID
	id, err := s.db.GetUserIdByUsername(identifier)
	if err == nil && id != "" {
		return id
	}
	return identifier
}

func (s *server) CallSession(stream gen.ChatService_CallSessionServer) error {
	var currentUserId string
	s.hub.RegisterCall(stream)
	defer func() {
		s.hub.UnregisterCall(stream)
		if currentUserId != "" {
			username := s.resolveUsername(currentUserId)
			log.Printf("[CALL] Stream closed: %s (%s)", currentUserId, username)
			s.handleAbruptDisconnect(currentUserId)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Don't log normal connection closures as errors
			if err != context.Canceled && !strings.Contains(err.Error(), "transport is closing") {
				log.Printf("[CALL] Error receiving signal: %v", err)
			}
			return err
		}

		// Normalize IDs to UUIDs for stable routing and attach names for display
		msg.SenderId = s.resolveUserId(msg.SenderId)
		msg.ReceiverId = s.resolveUserId(msg.ReceiverId)
		msg.SenderName = s.resolveUsername(msg.SenderId)
		msg.ReceiverName = s.resolveUsername(msg.ReceiverId)

		if currentUserId == "" && msg.SenderId != "" {
			currentUserId = msg.SenderId
			s.hub.UpdateCallName(stream, currentUserId)
			username := s.resolveUsername(currentUserId)
			log.Printf("[CALL] Stream identified: %s (%s)", currentUserId, username)
		}

		// Silence identity signals to reduce log volume
		if msg.ReceiverId == "" && msg.Payload == "IDENTITY" {
			continue
		}

		senderName := s.resolveUsername(msg.SenderId)
		receiverName := s.resolveUsername(msg.ReceiverId)
		log.Printf("[CALL] Signal: %s | From: %s (%s) | To: %s (%s) | CallID: %s",
			msg.Type.String(), msg.SenderId, senderName, msg.ReceiverId, receiverName, msg.CallId)

		// Handle database updates based on message type
		switch msg.Type {
		case gen.CallMessage_INITIATE:
			callId, err := s.db.CreateCall(msg.SenderId, msg.ReceiverId, "video", "")
			if err != nil {
				log.Printf("[CALL] Failed to create call in DB: %v", err)
			} else {
				msg.CallId = callId
				log.Printf("[CALL] New call created: %s", callId)
			}

			// 1. Route to receiver
			delivered := s.hub.BroadcastCall(msg)
			log.Printf("[CALL] INITIATE from %s to %s delivered: %v", msg.SenderId, msg.ReceiverId, delivered)

			// 2. Route back to sender so they get the generated call_id
			senderSignal := *msg
			senderSignal.ReceiverId = msg.SenderId
			s.hub.BroadcastCall(&senderSignal)

			// 3. Send push to wake up receiver
			s.sendCallPushNotification(msg.ReceiverId, msg.SenderName, msg.CallId)

			// 4. Add system message to chat
			s.saveCallSystemMessage(senderName, receiverName, "📹", "Видеозвонок")
			continue

		case gen.CallMessage_ACCEPT:
			log.Printf("[CALL] Accepted: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "active")
		case gen.CallMessage_REJECT:
			log.Printf("[CALL] Rejected: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "rejected")
			s.saveCallSystemMessage(senderName, receiverName, "📞↘️", "Пропущенный вызов")
		case gen.CallMessage_HANGUP:
			log.Printf("[CALL] Hung up: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "completed")

			durationText := ""
			duration, err := s.db.GetCallDuration(msg.CallId)
			if err == nil && duration > 0 {
				minutes := duration / 60
				seconds := duration % 60
				durationText = fmt.Sprintf(" (%d:%02d)", minutes, seconds)
			}
			s.saveCallSystemMessage(senderName, receiverName, "📞↗️", "Звонок завершен"+durationText)

		case gen.CallMessage_INITIATE_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
				s.saveConferenceSystemMessage(msg.RoomId, "Создана конференция")
			}
			members, _ := s.db.GetChatParticipants(msg.RoomId)
			s.hub.BroadcastConference(msg, members)
			continue

		case gen.CallMessage_JOIN_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
				s.saveConferenceSystemMessage(msg.RoomId, "Создана конференция")
			}
			s.hub.JoinConference(msg.RoomId, msg.SenderId, msg.SenderName)
			participants := s.hub.GetConferenceParticipants(msg.RoomId)
			creatorID := s.hub.GetConferenceCreator(msg.RoomId)
			response := map[string]interface{}{
				"participants": participants,
				"creator_id":   creatorID,
			}
			responseJSON, _ := json.Marshal(response)
			msg.Payload = string(responseJSON)
			members, _ := s.db.GetChatParticipants(msg.RoomId)
			s.hub.BroadcastConference(msg, members)
			continue

		case gen.CallMessage_LEAVE_CONFERENCE:
			s.hub.LeaveConference(msg.RoomId, msg.SenderId)
			members, _ := s.db.GetChatParticipants(msg.RoomId)
			s.hub.BroadcastConference(msg, members)
			continue

		case gen.CallMessage_END_CONFERENCE:
			if s.hub.IsConferenceCreator(msg.RoomId, msg.SenderId) {
				s.hub.EndConference(msg.RoomId)
				s.saveConferenceSystemMessage(msg.RoomId, "Конференция завершена")
				members, _ := s.db.GetChatParticipants(msg.RoomId)
				s.hub.BroadcastConference(msg, members)
			}
			continue
		}

		// Broadcast WebRTC signals (OFFER, ANSWER, ICE) to partner
		delivered := s.hub.BroadcastCall(msg)
		if !delivered {
			log.Printf("[CALL] Warning: Signal %s not delivered to %s (offline)",
				msg.Type.String(), msg.ReceiverId)
		}
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

// GetAllUsers возвращает список всех зарегистрированных пользователей
func (s *server) GetAllUsers(ctx context.Context, req *gen.GetAllUsersRequest) (*gen.GetAllUsersResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	_ = req // req is required by gRPC interface but not used here
	users, err := s.db.GetAllUsers()
	if err != nil {
		log.Printf("Error fetching all users: %v", err)
		return nil, err
	}

	var userInfos []*gen.UserInfo
	for _, u := range users {
		var lastSeen *timestamppb.Timestamp
		if u.LastSeenAt.Valid {
			lastSeen = timestamppb.New(u.LastSeenAt.Time)
		}

		userInfos = append(userInfos, &gen.UserInfo{
			Username:          u.Username,
			AvatarUrl:         u.AvatarURL,
			LastClientVersion: u.LastClientVersion,
			LastSeenAt:        lastSeen,
			Email:             u.Email,
		})
	}

	// Add server time to response for admin panel
	serverTime := timestamppb.Now()

	return &gen.GetAllUsersResponse{
		Users:      userInfos,
		ServerTime: serverTime,
	}, nil
}

// GetAllChats возвращает список всех чатов на сервере
func (s *server) GetAllChats(ctx context.Context, req *gen.GetAllChatsRequest) (*gen.GetAllChatsResponse, error) {
	chats, err := s.db.GetAllChats()
	if err != nil {
		log.Printf("Error fetching all chats: %v", err)
		return nil, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, &gen.ChatInfo{
			Id:                  c.ID,
			Name:                c.Name,
			Type:                c.Type,
			Participants:        c.Participants,
			CreatedAt:           timestamppb.New(c.CreatedAt),
			UnreadCount:         int32(c.UnreadCount),
			LastMessageTime:     timestamppb.New(c.LastMessageTime),
			Creator:             c.Creator,
			LastMessageText:     c.LastMessageText,
			AvatarUrl:           c.AvatarURL,
			FullAvatarUrl:       c.FullAvatarURL,
			LastMessageUsername: c.LastMessageUsername,
			LastMessageHasImage: c.LastMessageHasImage,
		})
	}

	return &gen.GetAllChatsResponse{
		Chats: chatInfos,
	}, nil
}

// GetHistory возвращает историю сообщений
func (s *server) GetHistory(_ context.Context, req *gen.GetHistoryRequest) (*gen.GetHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	roomID := req.Room
	if roomID == "" {
		return &gen.GetHistoryResponse{Messages: nil}, nil
	}

	rawMessages, err := s.db.GetMessages(limit, roomID)
	if err != nil {
		log.Printf("Error fetching history: %v", err)
		return nil, err
	}

	var messages []*gen.Message
	// Проходим в обратном порядке, чтобы сообщения были от старых к новым
	for i := len(rawMessages) - 1; i >= 0; i-- {
		m := rawMessages[i]

		// Check if encrypted data is empty
		if len(m.Encrypted) == 0 {
			log.Printf("Warning: message %s has empty encrypted data", m.MessageID)
			continue // Skip messages with no encrypted data
		}

		// Расшифровываем текст из базы
		decryptedText, err := decrypt(m.Encrypted)
		if err != nil {
			msgType := "text"
			if m.VoiceURL != "" {
				msgType = "voice"
			} else if m.ImageURL != "" {
				msgType = "image"
			}
			log.Printf("Failed to decrypt %s message %s (User: %s, Room: %s): %v", msgType, m.MessageID, m.Username, m.RoomID, err)

			// Show user-friendly error in the chat
			decryptedText = "не удалось расшифровать"
		}

		// Check if decrypted text is empty (skip ONLY if NO media at all)
		if decryptedText == "" && m.ImageURL == "" && m.VoiceURL == "" {
			log.Printf("Warning: message %s decrypted to empty string, skipping", m.MessageID)
			continue
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

		// Parse image URLs from JSON
		var imageURLs []string
		if m.ImageURLs != "" && m.ImageURLs != "[]" {
			json.Unmarshal([]byte(m.ImageURLs), &imageURLs)
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
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			AvatarUrl:          m.AvatarURL,
			ImageUrl:           m.ImageURL,
			ImageUrls:          imageURLs,
			Edited:             m.Edited,
			VoiceUrl:           m.VoiceURL,
			Duration:           m.Duration,
		})
	}

	return &gen.GetHistoryResponse{
		Messages: messages,
	}, nil
}

// SetReaction saves or updates a reaction and broadcasts it
func (s *server) SetReaction(_ context.Context, req *gen.ReactionRequest) (*gen.ReactionResponse, error) {
	// Получаем оригинальное сообщение для логирования текста
	var msgText string = "..."
	m, err := s.db.GetMessageByUUID(req.MessageId)
	if err == nil {
		decryptedText, err := decrypt(m.Encrypted)
		if err == nil {
			if len(decryptedText) > 15 {
				msgText = decryptedText[:15] + "..."
			} else {
				msgText = decryptedText
			}
		}
	}

	log.Printf("[Reaction] %s on %s (%s) by %s", req.Reaction.Emoji, req.MessageId, msgText, req.Reaction.User)

	err = s.db.SetReaction(req.MessageId, req.Reaction.User, req.Reaction.Emoji)
	if err != nil {
		log.Printf("Failed to set reaction: %v", err)
		return &gen.ReactionResponse{Success: false}, err
	}

	// Broadcast the updated message to all clients in the room
	// 1. Get the full message from DB
	if m.MessageID != "" { // m is already fetched above
		// 2. Decrypt text
		decryptedText, _ := decrypt(m.Encrypted)

		// 3. Get all reactions
		rawReactions, _ := s.db.GetReactionsForMessage(m.MessageID)
		var reactions []*gen.Reaction
		for _, r := range rawReactions {
			reactions = append(reactions, &gen.Reaction{
				User:  r.Username,
				Emoji: r.Emoji,
			})
		}

		// Parse image URLs from JSON
		var imageURLs []string
		if m.ImageURLs != "" && m.ImageURLs != "[]" {
			json.Unmarshal([]byte(m.ImageURLs), &imageURLs)
		}

		// 4. Create message object for broadcast
		msg := &gen.Message{
			Id:                 m.MessageID,
			User:               m.Username,
			Text:               decryptedText,
			CreatedAt:          timestamppb.New(m.CreatedAt),
			Reactions:          reactions,
			RepliedToMessageId: m.RepliedToMessageID,
			RepliedToUser:      m.RepliedToUser,
			RepliedToText:      m.RepliedToText,
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			AvatarUrl:          m.AvatarURL,
			ImageUrl:           m.ImageURL,
			ImageUrls:          imageURLs,
			Edited:             m.Edited,
			VoiceUrl:           m.VoiceURL,
			Duration:           m.Duration,
		}

		// 5. Broadcast to everyone in the room
		log.Printf("Broadcasting updated message %s with reactions to room %s", msg.Id, msg.RoomId)
		s.hub.Broadcast(msg)
	}

	return &gen.ReactionResponse{Success: true}, nil
}

// RegisterToken сохраняет FCM токен для пользователя
func (s *server) RegisterToken(_ context.Context, req *gen.TokenRequest) (*gen.TokenResponse, error) {
	err := s.db.SaveUserToken(req.User, req.Token, req.PushEnabled)
	if err != nil {
		log.Printf("Failed to save token for %s: %v", req.User, err)
		return &gen.TokenResponse{Success: false}, err
	}

	displayToken := req.Token
	if len(displayToken) > 10 {
		displayToken = displayToken[:10] + "..."
	}

	receiveStatus := "ENABLED"
	if req.Token == "DISABLED" {
		receiveStatus = "DISABLED"
	}

	s.logFCM("INFO", "Register: %s [%s] (Push for me: %s, Push from me: %v)",
		req.User, displayToken, receiveStatus, req.PushEnabled)
	return &gen.TokenResponse{Success: true}, nil
}

// GetChats возвращает список чатов для пользователя
func (s *server) GetChats(_ context.Context, req *gen.GetChatsRequest) (*gen.GetChatsResponse, error) {
	// Используем username для логов, а ID для запросов в БД
	// Убираем спам в логах, так как клиент опрашивает этот эндпоинт каждые 3 секунды
	// log.Printf("GetChats requested by user %s (ID: %s)", req.Username, req.UserId)

	// Если ID передан, используем его, иначе ищем по username (для старых клиентов)
	queryIdentifier := req.UserId
	if queryIdentifier == "" {
		id, err := s.db.GetUserIdByUsername(req.Username)
		if err == nil && id != "" {
			queryIdentifier = id
		} else {
			// Если не нашли ID, ставим нулевой UUID, чтобы запрос к БД не падал с ошибкой синтаксиса $1::uuid
			queryIdentifier = "00000000-0000-0000-0000-000000000000"
		}
	}

	chats, err := s.db.GetUserChats(queryIdentifier, req.Username)
	if err != nil {
		log.Printf("Error fetching chats for user %s: %v", req.Username, err)
		return nil, err
	}

	var chatInfos []*gen.ChatInfo
	for _, c := range chats {
		chatInfos = append(chatInfos, &gen.ChatInfo{
			Id:                  c.ID,
			Name:                c.Name,
			Type:                c.Type,
			Participants:        c.Participants,
			CreatedAt:           timestamppb.New(c.CreatedAt),
			UnreadCount:         int32(c.UnreadCount),
			LastMessageTime:     timestamppb.New(c.LastMessageTime),
			Creator:             c.Creator,
			LastMessageText:     c.LastMessageText,
			AvatarUrl:           c.AvatarURL,
			FullAvatarUrl:       c.FullAvatarURL,
			LastMessageUsername: c.LastMessageUsername,
			LastMessageHasImage: c.LastMessageHasImage,
		})
	}

	return &gen.GetChatsResponse{Chats: chatInfos}, nil
}

// CreateDirectChat создает или находит личный чат между двумя пользователями
func (s *server) CreateDirectChat(_ context.Context, req *gen.CreateDirectChatRequest) (*gen.CreateDirectChatResponse, error) {
	log.Printf("CreateDirectChat: %s <-> %s", req.User1, req.User2)
	chatID, err := s.db.GetDirectChatBetweenUsers(req.User1, req.User2)
	if err != nil {
		log.Printf("Error creating direct chat: %v", err)
		return &gen.CreateDirectChatResponse{Success: false}, err
	}

	log.Printf("Direct chat created/found: %s", chatID)
	return &gen.CreateDirectChatResponse{ChatId: chatID, Success: true}, nil
}

func (s *server) CreateGroupChat(_ context.Context, req *gen.CreateGroupChatRequest) (*gen.CreateGroupChatResponse, error) {
	log.Printf("CreateGroupChat: %s (Creator: %s)", req.Name, req.Creator)
	chatID := uuid.New().String()

	// Convert participants slice to JSON string
	participants := "["
	for i, p := range req.Participants {
		participants += "\"" + p + "\""
		if i < len(req.Participants)-1 {
			participants += ", "
		}
	}
	participants += "]"

	err := s.db.CreateChat(chatID, req.Name, "group", participants, req.Creator)
	if err != nil {
		log.Printf("Failed to create group chat in DB: %v", err)
		return &gen.CreateGroupChatResponse{Success: false}, err
	}
	log.Printf("Group chat created: %s (%s)", chatID, req.Name)
	return &gen.CreateGroupChatResponse{ChatId: chatID, Success: true}, nil
}

// UpdateUsername обновляет имя пользователя
func (s *server) UpdateUsername(_ context.Context, req *gen.UpdateUsernameRequest) (*gen.UpdateUsernameResponse, error) {
	err := s.db.UpdateUsername(req.OldUsername, req.NewUsername)
	if err != nil {
		log.Printf("Failed to update username from %s to %s: %v", req.OldUsername, req.NewUsername, err)
		return &gen.UpdateUsernameResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Username updated from %s to %s", req.OldUsername, req.NewUsername)
	return &gen.UpdateUsernameResponse{
		Success: true,
		Message: "Username updated successfully",
	}, nil
}

// UpdatePassword обновляет пароль пользователя
func (s *server) UpdatePassword(_ context.Context, req *gen.UpdatePasswordRequest) (*gen.UpdatePasswordResponse, error) {
	// Проверяем старый пароль
	storedHash, err := s.db.GetUserPasswordHash(req.Username)
	if err != nil {
		log.Printf("Failed to get password hash for %s: %v", req.Username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "User not found",
		}, err
	}

	if !CheckPassword(req.OldPassword, storedHash) {
		log.Printf("Old password verification failed for user: %s", req.Username)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "Old password is incorrect",
		}, fmt.Errorf("old password verification failed")
	}

	// Обновляем пароль
	err = s.db.UpdatePassword(req.Username, req.NewPassword)
	if err != nil {
		log.Printf("Failed to update password for %s: %v", req.Username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Password updated for user: %s", req.Username)
	return &gen.UpdatePasswordResponse{
		Success: true,
		Message: "Password updated successfully",
	}, nil
}

// AdminUpdatePassword allows a super admin to reset any user's password
func (s *server) AdminUpdatePassword(_ context.Context, req *gen.AdminUpdatePasswordRequest) (*gen.AdminUpdatePasswordResponse, error) {
	// Verify admin status
	if !s.db.IsSuperAdmin(req.AdminUsername) {
		log.Printf("Unauthorized AdminUpdatePassword attempt by %s", req.AdminUsername)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: "Unauthorized: only super admins can reset passwords",
		}, nil
	}

	// Update password
	err := s.db.UpdatePassword(req.TargetUsername, req.NewPassword)
	if err != nil {
		log.Printf("Failed to admin-reset password for %s: %v", req.TargetUsername, err)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Admin %s reset password for user: %s", req.AdminUsername, req.TargetUsername)
	return &gen.AdminUpdatePasswordResponse{
		Success: true,
		Message: "Password reset successfully",
	}, nil
}

// MarkRead marks messages in a room as read for a user
func (s *server) MarkRead(_ context.Context, req *gen.MarkReadRequest) (*gen.MarkReadResponse, error) {
	if strings.HasPrefix(req.RoomId, "favorites_") {
		return &gen.MarkReadResponse{Success: true}, nil
	}

	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	changed, err := s.db.MarkReadAndCheck(req.RoomId, username)
	if err != nil {
		log.Printf("Failed to mark read for %s in room %s: %v", username, req.RoomId, err)
		return &gen.MarkReadResponse{Success: false}, err
	}

	if changed {
		log.Printf("Marked read for %s in room %s", username, req.RoomId)
		// Broadcast read signal to the room
		s.hub.Broadcast(&gen.Message{
			User:   "SYSTEM",
			Text:   "READ_ALL:" + username,
			RoomId: req.RoomId,
		})
	}

	return &gen.MarkReadResponse{Success: true}, nil
}

// UpdateAvatar updates the avatar URL for a user
func (s *server) UpdateAvatar(_ context.Context, req *gen.UpdateAvatarRequest) (*gen.UpdateAvatarResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	// Save both thumbnail and full avatar URLs
	err := s.db.UpdateAvatarWithFull(username, req.AvatarUrl, req.FullAvatarUrl)
	if err != nil {
		log.Printf("Failed to update avatar for %s: %v", username, err)
		return &gen.UpdateAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated avatar for %s (thumb: %s, full: %s)", username, req.AvatarUrl, req.FullAvatarUrl)
	return &gen.UpdateAvatarResponse{Success: true, Message: "Avatar updated successfully"}, nil
}

// UpdateProfile updates user bio and status
func (s *server) UpdateProfile(_ context.Context, req *gen.UpdateProfileRequest) (*gen.UpdateProfileResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	err := s.db.UpdateProfile(username, req.Bio, req.Status)
	if err != nil {
		log.Printf("Failed to update profile for %s: %v", username, err)
		return &gen.UpdateProfileResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated profile for %s", username)
	return &gen.UpdateProfileResponse{Success: true, Message: "Profile updated successfully"}, nil
}

// GetUserProfile retrieves user profile information
func (s *server) GetUserProfile(_ context.Context, req *gen.GetUserProfileRequest) (*gen.GetUserProfileResponse, error) {
	var profile struct {
		Username, Bio, Status, AvatarURL string
		LastSeenAt                       sql.NullTime
	}
	var err error

	// Если передан user_id, используем его, иначе username (для обратной совместимости)
	if req.UserId != "" {
		profile, err = s.db.GetUserProfileById(req.UserId)
		if err != nil {
			log.Printf("Failed to get profile for user_id %s: %v", req.UserId, err)
			return &gen.GetUserProfileResponse{}, nil
		}
	} else if req.Username != "" {
		profile, err = s.db.GetUserProfile(req.Username)
		if err != nil {
			log.Printf("Failed to get profile for username %s: %v", req.Username, err)
			return &gen.GetUserProfileResponse{}, nil
		}
	} else {
		log.Printf("Failed to get profile: neither user_id nor username provided")
		return &gen.GetUserProfileResponse{}, nil
	}

	var lastSeen *timestamppb.Timestamp
	if profile.LastSeenAt.Valid {
		lastSeen = timestamppb.New(profile.LastSeenAt.Time)
	}

	return &gen.GetUserProfileResponse{
		Username:   profile.Username,
		Bio:        profile.Bio,
		Status:     profile.Status,
		AvatarUrl:  profile.AvatarURL,
		LastSeenAt: lastSeen,
	}, nil
}

// GetUserAvatar retrieves the avatar URL for a user (both thumbnail and full)
func (s *server) GetUserAvatar(_ context.Context, req *gen.GetUserAvatarRequest) (*gen.GetUserAvatarResponse, error) {
	avatarURL, fullAvatarURL, err := s.db.GetUserAvatarWithFull(req.Username)
	if err != nil {
		log.Printf("Failed to get avatar for %s: %v", req.Username, err)
		return &gen.GetUserAvatarResponse{AvatarUrl: "", FullAvatarUrl: ""}, nil
	}

	return &gen.GetUserAvatarResponse{AvatarUrl: avatarURL, FullAvatarUrl: fullAvatarURL}, nil
}

// AddParticipant adds a user to a group chat
func (s *server) AddParticipant(_ context.Context, req *gen.AddParticipantRequest) (*gen.AddParticipantResponse, error) {
	log.Printf("AddParticipant: Adding user %s to chat %s", req.Username, req.ChatId)
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		log.Printf("AddParticipant error: Chat %s not found", req.ChatId)
		return &gen.AddParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		log.Printf("AddParticipant error: Chat %s is not a group chat (type: %s)", req.ChatId, chat.Type)
		return &gen.AddParticipantResponse{Success: false, Message: "Participants can only be added to group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		log.Printf("AddParticipant error: Failed to parse participants for chat %s: %v", req.ChatId, err)
		return &gen.AddParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	// Check if user already in chat
	for _, p := range participants {
		if p == req.Username {
			log.Printf("AddParticipant: User %s is already in chat %s", req.Username, req.ChatId)
			return &gen.AddParticipantResponse{Success: false, Message: "User already in chat"}, nil
		}
	}

	participants = append(participants, req.Username)
	updatedParticipants, _ := json.Marshal(participants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		log.Printf("AddParticipant error: Failed to update DB for chat %s: %v", req.ChatId, err)
		return &gen.AddParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	// Notify all participants about the change
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers() // Refresh lists for everyone

	log.Printf("AddParticipant success: User %s added to chat %s", req.Username, req.ChatId)
	return &gen.AddParticipantResponse{Success: true, Message: "User added successfully"}, nil
}

// RemoveParticipant removes a user from a group chat
func (s *server) RemoveParticipant(_ context.Context, req *gen.RemoveParticipantRequest) (*gen.RemoveParticipantResponse, error) {
	log.Printf("RemoveParticipant: Removing user %s from chat %s", req.Username, req.ChatId)
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		log.Printf("RemoveParticipant error: Chat %s not found", req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Chat not found"}, nil
	}

	if chat.Type != "group" {
		log.Printf("RemoveParticipant error: Chat %s is not a group chat", req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Participants can only be removed from group chats"}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		log.Printf("RemoveParticipant error: Failed to parse participants: %v", err)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Internal error parsing participants"}, nil
	}

	newParticipants := []string{}
	found := false
	for _, p := range participants {
		if p != req.Username {
			newParticipants = append(newParticipants, p)
		} else {
			found = true
		}
	}

	if !found {
		log.Printf("RemoveParticipant error: User %s not in chat %s", req.Username, req.ChatId)
		return &gen.RemoveParticipantResponse{Success: false, Message: "User not in chat"}, nil
	}

	updatedParticipants, _ := json.Marshal(newParticipants)

	if err := s.db.UpdateChatParticipants(req.ChatId, string(updatedParticipants)); err != nil {
		log.Printf("RemoveParticipant error: Failed to update DB: %v", err)
		return &gen.RemoveParticipantResponse{Success: false, Message: "Failed to update participants"}, nil
	}

	// Notify all participants
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	_ = s.db.IncrementUserChatListVersion(req.Username) // Notify the removed user too
	s.broadcastOnlineUsers()

	log.Printf("RemoveParticipant success: User %s removed from chat %s", req.Username, req.ChatId)
	return &gen.RemoveParticipantResponse{Success: true, Message: "User removed successfully"}, nil
}

// DeleteChat deletes a chat and all its messages and images
func (s *server) DeleteChat(_ context.Context, req *gen.DeleteChatRequest) (*gen.DeleteChatResponse, error) {
	if req.ChatId == "" {
		return &gen.DeleteChatResponse{Success: false, Message: "Chat ID is required"}, nil
	}

	log.Printf("DeleteChat: Request to delete chat %s by %s", req.ChatId, req.RequesterUsername)

	// 1. Get all participants and creator before deletion
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		log.Printf("DeleteChat warning: Chat %s not found or DB error: %v", req.ChatId, err)
		// Return error to inform user that chat is already deleted
		return &gen.DeleteChatResponse{Success: false, Message: "Chat or group already deleted"}, nil
	}

	// Security check: only creator can delete group chats
	// We allow users to delete their own direct chats, but groups must be deleted by the creator
	if chat.Type == "group" && chat.CreatorUsername != req.RequesterUsername {
		log.Printf("DeleteChat error: User %s is not authorized to delete group chat %s (creator: %s)",
			req.RequesterUsername, req.ChatId, chat.CreatorUsername)
		return &gen.DeleteChatResponse{
			Success: false,
			Message: "You don't have permission to delete this group. Only the group administrator can delete it.",
		}, nil
	}

	var participants []string
	if err := json.Unmarshal([]byte(chat.Participants), &participants); err != nil {
		log.Printf("DeleteChat warning: Failed to parse participants for %s: %v", req.ChatId, err)
	}

	// 2. Get all image URLs to delete files
	imageURLs, err := s.db.GetChatMessagesImageURLs(req.ChatId)
	if err != nil {
		log.Printf("DeleteChat warning: Failed to get image URLs for chat %s: %v", req.ChatId, err)
	}

	// 3. Delete all image files from disk
	for _, url := range imageURLs {
		if err := DeleteImageFile(url); err != nil {
			log.Printf("DeleteChat error: Failed to delete image file %s: %v", url, err)
		}
	}

	// 4. Delete the chat and all messages from database
	err = s.db.DeleteChat(req.ChatId)
	if err != nil {
		log.Printf("DeleteChat error: Failed to delete chat %s from DB: %v", req.ChatId, err)
		return &gen.DeleteChatResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("DeleteChat success: Chat %s deleted. Notifying %d participants.", req.ChatId, len(participants))

	// 5. Increment version for all former participants so their lists refresh
	for _, p := range participants {
		_ = s.db.IncrementUserChatListVersion(p)
	}

	// 6. Send signal to clear cache for all participants
	s.hub.Broadcast(&gen.Message{
		User:   "SYSTEM",
		Text:   "CLEAR_CACHE:" + req.ChatId,
		RoomId: req.ChatId,
	})

	// 7. Send signal to exit the deleted chat for all participants
	s.hub.Broadcast(&gen.Message{
		User:   "SYSTEM",
		Text:   "CHAT_DELETED:" + req.ChatId,
		RoomId: req.ChatId,
	})

	// 8. Broadcast update signal
	s.broadcastOnlineUsers()

	return &gen.DeleteChatResponse{Success: true, Message: "Chat deleted successfully"}, nil
}

// DeleteProfile deletes a user's profile and all their data
func (s *server) DeleteProfile(ctx context.Context, req *gen.DeleteProfileRequest) (*gen.DeleteProfileResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	if username == "" {
		return &gen.DeleteProfileResponse{Success: false, Message: "Username or User ID is required"}, nil
	}

	log.Printf("DeleteProfile: Request to delete user %s", username)

	err := s.db.DeleteProfile(username)
	if err != nil {
		log.Printf("Failed to delete profile for %s: %v", username, err)
		return &gen.DeleteProfileResponse{Success: false, Message: err.Error()}, nil
	}

	// Force disconnect the user if they are currently online
	s.hub.BroadcastGlobal(&gen.Message{
		User: "SYSTEM",
		Text: "FORCE_DISCONNECT:" + username,
	})

	log.Printf("Successfully deleted profile for user: %s", username)
	s.broadcastOnlineUsers()

	return &gen.DeleteProfileResponse{Success: true, Message: "Profile deleted successfully"}, nil
}

// sendPushNotification отправляет уведомление через FCM
func (s *server) sendPushNotification(user, title, body, roomID string) {
	if s.firebaseApp == nil {
		s.logFCM("WARN", "Skip %s: Firebase not init", user)
		return
	}

	// Проверяем, не замьючен ли чат для этого пользователя
	mutedChats, err := s.db.GetMutedChats(user)
	if err == nil {
		for _, mutedRoomID := range mutedChats {
			if mutedRoomID == roomID {
				s.logFCM("INFO", "Skip %s: Chat %s is muted", user, roomID)
				return
			}
		}
	}

	token, err := s.db.GetUserToken(user)
	if err != nil || token == "" {
		s.logFCM("WARN", "Skip %s: No token", user)
		return
	}

	if token == "DISABLED" {
		s.logFCM("INFO", "Skip %s: User disabled push", user)
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		s.logFCM("ERROR", "Client err: %v", err)
		return
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: map[string]string{
			"title":   title,
			"body":    body,
			"room_id": roomID,
			"sender":  title,
		},
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		s.logFCM("ERROR", "Send to %s failed: %v", user, err)
		return
	}

	s.logFCM("SUCCESS", "Sent to %s", user)
}

func (s *server) saveConferenceSystemMessage(roomID, text string) {
	msgId := uuid.New().String()
	createdAt := time.Now()
	displayText := "📹 " + text

	// Encrypt for database
	encryptedText, err := encrypt(displayText)
	if err != nil {
		log.Printf("[CONF] Encryption failed for system message: %v", err)
		return
	}

	// Save to DB
	err = s.db.SaveMessage(msgId, "SYSTEM", "", encryptedText, createdAt, "", "", "", roomID, "", "[]", "", 0)
	if err != nil {
		log.Printf("[CONF] Failed to save call system message: %v", err)
		return
	}

	// Broadcast to the room
	broadcastMsg := &gen.Message{
		Id:        msgId,
		User:      "SYSTEM",
		Text:      displayText,
		CreatedAt: timestamppb.New(createdAt),
		RoomId:    roomID,
	}
	s.hub.Broadcast(broadcastMsg)
}

func (s *server) saveCallSystemMessage(u1, u2, icon, text string) {
	chatID, err := s.db.GetDirectChatBetweenUsers(u1, u2)
	if err != nil {
		log.Printf("[CALL] Failed to find chat for system message: %v", err)
		return
	}

	msgId := uuid.New().String()
	createdAt := time.Now()
	displayText := icon + " " + text

	// Encrypt for database
	encryptedText, err := encrypt(displayText)
	if err != nil {
		log.Printf("[CALL] Encryption failed for system message: %v", err)
		return
	}

	// Save to DB
	err = s.db.SaveMessage(msgId, "SYSTEM", "", encryptedText, createdAt, "", "", "", chatID, "", "[]", "", 0)
	if err != nil {
		log.Printf("[CALL] Failed to save call system message: %v", err)
		return
	}

	// Broadcast to the room
	broadcastMsg := &gen.Message{
		Id:        msgId,
		User:      "SYSTEM",
		Text:      displayText,
		CreatedAt: timestamppb.New(createdAt),
		RoomId:    chatID,
	}
	s.hub.Broadcast(broadcastMsg)
}

func (s *server) handleAbruptDisconnect(userId string) {
	// Send HANGUP signal to any potential partners
	// In a real production app, we would query the 'calls' table for 'active' calls involving this user
	// For now, we'll broadcast a system-level hangup to clear the UI on the other side
	// if they were waiting for this user.
	log.Printf("[CALL] Handling abrupt disconnect for %s", userId)
}

func (s *server) sendCallPushNotification(receiverId, senderName, callId string) {
	if s.firebaseApp == nil {
		return
	}

	// Resolve receiverId to username if it's a UUID
	username := s.resolveUsername(receiverId)
	senderUsername := s.resolveUsername(senderName)

	token, err := s.db.GetUserToken(username)
	if err != nil || token == "" || token == "DISABLED" {
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		return
	}

	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"type":      "VOIP_CALL",
			"call_id":   callId,
			"sender_id": senderUsername,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		s.logFCM("ERROR", "Call Push to %s failed: %v", receiverId, err)
	} else {
		s.logFCM("SUCCESS", "Call Push sent to %s", receiverId)
	}
}

func (s *server) broadcastOnlineUsers() {
	users := s.hub.GetOnlineUsers()
	usersJson, _ := json.Marshal(users)
	msg := &gen.Message{
		User:      "SYSTEM",
		Text:      "ONLINE_USERS_UPDATE:" + string(usersJson),
		Id:        uuid.New().String(),
		CreatedAt: timestamppb.Now(),
	}
	s.hub.BroadcastGlobal(msg)
}

// DeleteMessages deletes a list of messages
func (s *server) DeleteMessages(_ context.Context, req *gen.DeleteMessagesRequest) (*gen.DeleteMessagesResponse, error) {
	var anyDeleted bool
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}

		// Permission check: only sender or group admin or chat participant (for SYSTEM messages) can delete
		canDelete := false
		if msg.User == req.RequesterUsername {
			canDelete = true
		} else if msg.RoomId != "" {
			chat, err := s.db.GetChat(msg.RoomId)
			if err == nil {
				// Group admin can delete any message
				if chat.CreatorUsername == req.RequesterUsername {
					canDelete = true
				} else if msg.User == "SYSTEM" {
					// Any participant can delete SYSTEM messages in the chat
					var participants []string
					if json.Unmarshal([]byte(chat.Participants), &participants) == nil {
						for _, p := range participants {
							if p == req.RequesterUsername {
								canDelete = true
								break
							}
						}
					}
				}
			}
		}

		if !canDelete {
			log.Printf("Unauthorized delete attempt by %s for message in %s", req.RequesterUsername, msg.RoomId)
			continue
		}

		// Try deleting by ID first if available
		if msg.Id != "" {
			// Get full message with image URLs before deletion
			fullMsg, err := s.db.GetMessageByUUID(msg.Id)
			if err != nil {
				log.Printf("Failed to get message %s: %v", msg.Id, err)
			} else {
				// Delete single image file if exists
				if fullMsg.ImageURL != "" {
					if err := DeleteImageFile(fullMsg.ImageURL); err != nil {
						log.Printf("Failed to delete image file for message %s: %v", msg.Id, err)
						// Continue with message deletion even if image deletion fails
					}
				}

				// Delete all gallery images if exists
				if fullMsg.ImageURLs != "" && fullMsg.ImageURLs != "[]" {
					var imageURLs []string
					if err := json.Unmarshal([]byte(fullMsg.ImageURLs), &imageURLs); err == nil {
						for _, url := range imageURLs {
							if err := DeleteImageFile(url); err != nil {
								log.Printf("Failed to delete gallery image file for message %s: %v", msg.Id, err)
								// Continue with message deletion even if image deletion fails
							}
						}
					}
				}
			}

			err = s.db.DeleteMessageByUUID(msg.Id)
			if err == nil {
				anyDeleted = true
				log.Printf("Deleted message by ID: %s", msg.Id)

				// Increment chat list version for all participants to trigger cache refresh
				_ = s.db.IncrementParticipantsChatListVersion(msg.RoomId)

				// Broadcast deletion to the room
				s.hub.Broadcast(&gen.Message{
					User:   "SYSTEM",
					Text:   "DELETE_MESSAGE:" + msg.Id,
					RoomId: msg.RoomId,
				})
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
				// Delete single image file if candidate has one
				if candidate.ImageURL != "" {
					if err := DeleteImageFile(candidate.ImageURL); err != nil {
						log.Printf("Failed to delete image file for candidate message: %v", err)
						// Continue with message deletion even if image deletion fails
					}
				}

				// Delete all gallery images if candidate has them
				if candidate.ImageURLs != "" && candidate.ImageURLs != "[]" {
					var imageURLs []string
					if err := json.Unmarshal([]byte(candidate.ImageURLs), &imageURLs); err == nil {
						for _, url := range imageURLs {
							if err := DeleteImageFile(url); err != nil {
								log.Printf("Failed to delete gallery image file for candidate message: %v", err)
								// Continue with message deletion even if image deletion fails
							}
						}
					}
				}

				err = s.db.DeleteMessageByID(candidate.ID)
				if err == nil {
					anyDeleted = true
					log.Printf("Deleted message by content from %s", msg.User)

					// Increment chat list version for all participants to trigger cache refresh
					_ = s.db.IncrementParticipantsChatListVersion(candidate.RoomID)

					// Broadcast deletion to the room
					s.hub.Broadcast(&gen.Message{
						User:   "SYSTEM",
						Text:   "DELETE_MESSAGE:" + candidate.MessageID,
						RoomId: candidate.RoomID,
					})
				}
				break
			}
		}
	}

	return &gen.DeleteMessagesResponse{Success: anyDeleted}, nil
}

// EditMessage edits a message by ID
func (s *server) EditMessage(_ context.Context, req *gen.EditMessageRequest) (*gen.EditMessageResponse, error) {
	if req.MessageId == "" {
		return &gen.EditMessageResponse{Success: false, Message: "Message ID is required"}, nil
	}

	err := s.db.UpdateMessageText(req.MessageId, req.Text)
	if err != nil {
		log.Printf("Failed to edit message %s: %v", req.MessageId, err)
		return &gen.EditMessageResponse{Success: false, Message: err.Error()}, nil
	}

	// Broadcast the updated message
	m, err := s.db.GetMessageByUUID(req.MessageId)
	if err == nil {
		// Increment chat list version for all participants to trigger cache refresh
		_ = s.db.IncrementParticipantsChatListVersion(m.RoomID)

		decryptedText, _ := decrypt(m.Encrypted)
		rawReactions, _ := s.db.GetReactionsForMessage(m.MessageID)
		var reactions []*gen.Reaction
		for _, r := range rawReactions {
			reactions = append(reactions, &gen.Reaction{User: r.Username, Emoji: r.Emoji})
		}

		// Parse image URLs from JSON
		var imageURLs []string
		if m.ImageURLs != "" && m.ImageURLs != "[]" {
			json.Unmarshal([]byte(m.ImageURLs), &imageURLs)
		}

		s.hub.Broadcast(&gen.Message{
			Id:                 m.MessageID,
			User:               m.Username,
			Text:               decryptedText,
			CreatedAt:          timestamppb.New(m.CreatedAt),
			Reactions:          reactions,
			RepliedToMessageId: m.RepliedToMessageID,
			RepliedToUser:      m.RepliedToUser,
			RepliedToText:      m.RepliedToText,
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			AvatarUrl:          m.AvatarURL,
			ImageUrl:           m.ImageURL,
			ImageUrls:          imageURLs,
			Edited:             m.Edited,
			VoiceUrl:           m.VoiceURL,
			Duration:           m.Duration,
		})
	}

	log.Printf("Edited message %s", req.MessageId)
	return &gen.EditMessageResponse{Success: true, Message: "Message edited successfully"}, nil
}

func (s *server) UpdateChatName(_ context.Context, req *gen.UpdateChatNameRequest) (*gen.UpdateChatNameResponse, error) {
	if req.ChatId == "" || req.NewName == "" {
		return &gen.UpdateChatNameResponse{Success: false, Message: "Chat ID and New Name are required"}, nil
	}

	log.Printf("UpdateChatName: Updating chat %s to '%s'", req.ChatId, req.NewName)

	err := s.db.UpdateChatName(req.ChatId, req.NewName)
	if err != nil {
		log.Printf("UpdateChatName error: %v", err)
		return &gen.UpdateChatNameResponse{Success: false, Message: err.Error()}, nil
	}

	// Increment version for all participants so their lists refresh
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers()

	return &gen.UpdateChatNameResponse{Success: true, Message: "Chat name updated successfully"}, nil
}

func (s *server) UpdateChatAvatar(_ context.Context, req *gen.UpdateChatAvatarRequest) (*gen.UpdateChatAvatarResponse, error) {
	if req.ChatId == "" || req.AvatarUrl == "" {
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Chat ID and Avatar URL are required"}, nil
	}

	log.Printf("UpdateChatAvatar: Checking admin status for chat %s, user %s", req.ChatId, req.Username)

	// Get chat to verify admin status
	chat, err := s.db.GetChat(req.ChatId)
	if err != nil {
		log.Printf("UpdateChatAvatar error: Chat not found: %v", err)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Chat not found"}, nil
	}

	// Verify user is the creator/admin
	if chat.CreatorUsername != req.Username {
		log.Printf("UpdateChatAvatar error: User %s is not admin (creator: %s)", req.Username, chat.CreatorUsername)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: "Only chat admin can change group photo"}, nil
	}

	// Update avatar (both thumbnail and full version)
	err = s.db.UpdateChatAvatarWithFull(req.ChatId, req.AvatarUrl, req.FullAvatarUrl)
	if err != nil {
		log.Printf("UpdateChatAvatar error: %v", err)
		return &gen.UpdateChatAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("UpdateChatAvatar: Updated avatar for chat %s by admin %s (thumb: %s, full: %s)", req.ChatId, req.Username, req.AvatarUrl, req.FullAvatarUrl)

	// Increment version for all participants so their lists refresh
	_ = s.db.IncrementParticipantsChatListVersion(req.ChatId)
	s.broadcastOnlineUsers()

	return &gen.UpdateChatAvatarResponse{Success: true, Message: "Chat avatar updated successfully"}, nil
}

func (s *server) AddContact(_ context.Context, req *gen.AddContactRequest) (*gen.AddContactResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	err := s.db.AddContact(username, req.ContactUsername)
	if err != nil {
		log.Printf("Failed to add contact %s for %s: %v", req.ContactUsername, username, err)
		return &gen.AddContactResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("User %s added contact %s", username, req.ContactUsername)
	return &gen.AddContactResponse{Success: true, Message: "Contact added successfully"}, nil
}

func (s *server) RemoveContact(_ context.Context, req *gen.RemoveContactRequest) (*gen.RemoveContactResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	err := s.db.RemoveContact(username, req.ContactUsername)
	if err != nil {
		log.Printf("Failed to remove contact %s for %s: %v", req.ContactUsername, username, err)
		return &gen.RemoveContactResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("User %s removed contact %s", username, req.ContactUsername)
	return &gen.RemoveContactResponse{Success: true, Message: "Contact removed successfully"}, nil
}

func (s *server) GetContacts(_ context.Context, req *gen.GetContactsRequest) (*gen.GetContactsResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	contacts, err := s.db.GetContacts(username)
	if err != nil {
		log.Printf("Failed to get contacts for %s: %v", username, err)
		return &gen.GetContactsResponse{Contacts: nil}, nil
	}
	return &gen.GetContactsResponse{Contacts: contacts}, nil
}

func (s *server) GetChatListVersion(_ context.Context, req *gen.GetChatListVersionRequest) (*gen.GetChatListVersionResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	version, err := s.db.GetUserChatListVersion(username)
	if err != nil {
		return &gen.GetChatListVersionResponse{Version: 0}, nil
	}
	return &gen.GetChatListVersionResponse{Version: version}, nil
}

// resolveUsername converts a potential user ID to a username if needed
func (s *server) resolveUsername(identifier string) string {
	if identifier == "" {
		return ""
	}
	// Check if it's a UUID
	if _, err := uuid.Parse(identifier); err == nil {
		// It's a UUID, try to get the username
		username, err := s.db.GetUsernameByID(identifier)
		if err == nil {
			return username
		}
	}
	return identifier
}

func (s *server) GetThemes(_ context.Context, req *gen.GetThemesRequest) (*gen.GetThemesResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	} else {
		username = s.resolveUsername(username)
	}

	currentID, themes, err := s.db.GetUserThemes(username)
	if err != nil {
		s.logErrorOnce("GetThemes:"+username, "Failed to get themes for %s: %v", username, err)
		return &gen.GetThemesResponse{CurrentThemeId: "dark"}, nil
	}

	var customThemes []*gen.CustomTheme
	for _, t := range themes {
		customThemes = append(customThemes, &gen.CustomTheme{
			Id:                         t.ThemeID,
			Name:                       t.Name,
			PrimaryColor:               t.PrimaryColor,
			OnPrimaryColor:             t.OnPrimaryColor,
			SurfaceColor:               t.SurfaceColor,
			OnSurfaceColor:             t.OnSurfaceColor,
			BackgroundColor:            t.BackgroundColor,
			TextPrimaryColor:           t.TextPrimaryColor,
			TextSecondaryColor:         t.TextSecondaryColor,
			IsDark:                     t.IsDark,
			ChatBackgroundImageUrl:     t.ChatBackgroundImageUrl,
			ChatListBackgroundImageUrl: t.ChatListBackgroundImageUrl,
			BottomPanelColor:           t.BottomPanelColor,
			OnBottomPanelColor:         t.OnBottomPanelColor,
			SurfaceContainer:           t.SurfaceContainer,
			OutgoingBubbleColor:        t.OutgoingBubbleColor,
			IncomingBubbleColor:        t.IncomingBubbleColor,
		})
	}

	log.Printf("Retrieved %d custom themes for user %s (Current: %s)", len(customThemes), username, currentID)

	return &gen.GetThemesResponse{
		CurrentThemeId: currentID,
		CustomThemes:   customThemes,
	}, nil
}

func (s *server) SaveTheme(_ context.Context, req *gen.SaveThemeRequest) (*gen.SaveThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	} else {
		username = s.resolveUsername(username)
	}

	log.Printf("Saving theme '%s' (ID: %s) for user %s. Chat Background URL: %s", req.Theme.Name, req.Theme.Id, username, req.Theme.ChatBackgroundImageUrl)
	err := s.db.SaveUserTheme(username, req.Theme)
	if err != nil {
		s.logErrorOnce("SaveTheme:"+username, "Failed to save theme for %s: %v", username, err)
		return &gen.SaveThemeResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("Theme '%s' saved successfully for %s", req.Theme.Name, username)
	return &gen.SaveThemeResponse{Success: true, Message: "Theme saved"}, nil
}

func (s *server) SetCurrentTheme(_ context.Context, req *gen.SetCurrentThemeRequest) (*gen.SetCurrentThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	} else {
		username = s.resolveUsername(username)
	}

	log.Printf("Setting current theme to %s for user %s", req.ThemeId, username)
	err := s.db.SetCurrentTheme(username, req.ThemeId)
	if err != nil {
		s.logErrorOnce("SetCurrentTheme:"+username, "Failed to set current theme for %s: %v", username, err)
		return &gen.SetCurrentThemeResponse{Success: false}, nil
	}
	return &gen.SetCurrentThemeResponse{Success: true}, nil
}

func (s *server) DeleteTheme(_ context.Context, req *gen.DeleteThemeRequest) (*gen.DeleteThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	} else {
		username = s.resolveUsername(username)
	}

	log.Printf("Deleting theme %s for user %s", req.ThemeId, username)
	err := s.db.DeleteUserTheme(username, req.ThemeId)
	if err != nil {
		s.logErrorOnce("DeleteTheme:"+username, "Failed to delete theme for %s: %v", username, err)
		return &gen.DeleteThemeResponse{Success: false}, nil
	}
	log.Printf("Theme %s deleted successfully for %s", req.ThemeId, username)
	return &gen.DeleteThemeResponse{Success: true}, nil
}

func (s *server) GetFCMLogs(_ context.Context, _ *gen.GetFCMLogsRequest) (*gen.GetFCMLogsResponse, error) {
	s.fcmLogsMu.Lock()
	defer s.fcmLogsMu.Unlock()

	// Return a copy to avoid concurrent issues
	logs := make([]*gen.FCMLogEntry, len(s.fcmLogs))
	for i, l := range s.fcmLogs {
		logs[i] = &gen.FCMLogEntry{
			Timestamp: l.Timestamp,
			Level:     l.Level,
			Message:   l.Message,
		}
	}
	return &gen.GetFCMLogsResponse{Logs: logs}, nil
}

// SaveDraft saves a draft message for a user in a specific room
func (s *server) SaveDraft(_ context.Context, req *gen.SaveDraftRequest) (*gen.SaveDraftResponse, error) {
	if req.UserId == "" {
		return &gen.SaveDraftResponse{Success: false, Message: "empty user id"}, nil
	}

	var err error
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		err = s.db.SaveDraftByUserID(req.UserId, req.RoomId, req.DraftText, req.RepliedToMessageId, req.RepliedToUser, req.RepliedToText)
	} else {
		err = s.db.SaveDraft(req.UserId, req.RoomId, req.DraftText, req.RepliedToMessageId, req.RepliedToUser, req.RepliedToText)
	}

	if err != nil {
		s.logErrorOnce("SaveDraft:"+req.UserId, "Failed to save draft for user %s in room %s: %v", req.UserId, req.RoomId, err)
		return &gen.SaveDraftResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("Draft saved for user %s in room %s (length: %d)", req.UserId, req.RoomId, len(req.DraftText))
	return &gen.SaveDraftResponse{Success: true, Message: "Draft saved successfully"}, nil
}

// GetDraft retrieves a draft message for a user in a specific room
func (s *server) GetDraft(_ context.Context, req *gen.GetDraftRequest) (*gen.GetDraftResponse, error) {
	if req.UserId == "" {
		return &gen.GetDraftResponse{HasDraft: false}, nil
	}

	var draft struct {
		DraftText          string
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		UpdatedAt          time.Time
	}
	var err error

	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		draft, err = s.db.GetDraftByUserID(req.UserId, req.RoomId)
	} else {
		draft, err = s.db.GetDraft(req.UserId, req.RoomId)
	}

	if err != nil {
		s.logErrorOnce("GetDraft:"+req.UserId, "Failed to get draft for user %s in room %s: %v", req.UserId, req.RoomId, err)
		return &gen.GetDraftResponse{HasDraft: false}, nil
	}

	hasDraft := draft.DraftText != "" || draft.RepliedToMessageID != ""

	return &gen.GetDraftResponse{
		DraftText:          draft.DraftText,
		RepliedToMessageId: draft.RepliedToMessageID,
		RepliedToUser:      draft.RepliedToUser,
		RepliedToText:      draft.RepliedToText,
		HasDraft:           hasDraft,
	}, nil
}

// DeleteDraft removes a draft message for a user in a specific room
// Only logs if a draft was actually deleted (not for empty deletions)
func (s *server) DeleteDraft(_ context.Context, req *gen.DeleteDraftRequest) (*gen.DeleteDraftResponse, error) {
	if req.UserId == "" {
		return &gen.DeleteDraftResponse{Success: false}, nil
	}

	var deleted bool
	var err error
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		deleted, err = s.db.DeleteDraftByUserID(req.UserId, req.RoomId)
	} else {
		err = s.db.DeleteDraft(req.UserId, req.RoomId)
		deleted = err == nil
	}

	if err != nil {
		s.logErrorOnce("DeleteDraft:"+req.UserId, "Failed to delete draft for user %s in room %s: %v", req.UserId, req.RoomId, err)
		return &gen.DeleteDraftResponse{Success: false}, nil
	}
	// Only log if we actually deleted something (not for empty/duplicate deletions)
	if deleted {
		log.Printf("Draft deleted for user %s in room %s", req.UserId, req.RoomId)
	}
	return &gen.DeleteDraftResponse{Success: true}, nil
}

// GetMutedChats returns the list of chat rooms where the user has disabled push notifications
func (s *server) GetMutedChats(_ context.Context, req *gen.GetMutedChatsRequest) (*gen.GetMutedChatsResponse, error) {
	if req.UserId == "" {
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}

	var mutedChats []string
	var err error

	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		mutedChats, err = s.db.GetMutedChatsByUserID(req.UserId)
	} else {
		mutedChats, err = s.db.GetMutedChats(req.UserId)
	}

	if err != nil {
		s.logErrorOnce("GetMutedChats:"+req.UserId, "Failed to get muted chats for user %s: %v", req.UserId, err)
		return &gen.GetMutedChatsResponse{RoomIds: []string{}}, nil
	}
	return &gen.GetMutedChatsResponse{RoomIds: mutedChats}, nil
}

// SetMutedChat sets or unsets the mute status for a chat room
// When muted=true, the user will not receive push notifications from this chat
func (s *server) SetMutedChat(_ context.Context, req *gen.SetMutedChatRequest) (*gen.SetMutedChatResponse, error) {
	if req.UserId == "" {
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	var err error
	// Check if it's a UUID or username
	if _, uuidErr := uuid.Parse(req.UserId); uuidErr == nil {
		err = s.db.SetMutedChatByUserID(req.UserId, req.RoomId, req.Muted)
	} else {
		err = s.db.SetMutedChat(req.UserId, req.RoomId, req.Muted)
	}

	if err != nil {
		s.logErrorOnce("SetMutedChat:"+req.UserId, "Failed to set muted status for user %s in room %s (muted=%v): %v", req.UserId, req.RoomId, req.Muted, err)
		return &gen.SetMutedChatResponse{Success: false}, nil
	}

	action := "muted"
	if !req.Muted {
		action = "unmuted"
	}
	log.Printf("Chat %s for user %s in room %s", action, req.UserId, req.RoomId)
	return &gen.SetMutedChatResponse{Success: true}, nil
}

// GetUserId retrieves the user ID (UUID) for a given username
func (s *server) GetUserId(_ context.Context, req *gen.GetUserIdRequest) (*gen.GetUserIdResponse, error) {
	userID, err := s.db.GetUserIdByUsername(req.Username)
	if err != nil {
		log.Printf("Failed to get user ID for %s: %v", req.Username, err)
		return &gen.GetUserIdResponse{UserId: "", Found: false}, nil
	}
	return &gen.GetUserIdResponse{UserId: userID, Found: true}, nil
}

func (s *server) AddFavorite(ctx context.Context, req *gen.AddFavoriteRequest) (*gen.AddFavoriteResponse, error) {
	if req.UserId == "" || req.MessageId == "" {
		return &gen.AddFavoriteResponse{Success: false, Message: "empty user id or message id"}, nil
	}
	err := s.db.AddFavorite(req.UserId, req.MessageId)
	if err != nil {
		log.Printf("Failed to add favorite: %v", err)
		return &gen.AddFavoriteResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.AddFavoriteResponse{Success: true}, nil
}

func (s *server) RemoveFavorite(ctx context.Context, req *gen.RemoveFavoriteRequest) (*gen.RemoveFavoriteResponse, error) {
	if req.UserId == "" || req.MessageId == "" {
		return &gen.RemoveFavoriteResponse{Success: false}, nil
	}
	err := s.db.RemoveFavorite(req.UserId, req.MessageId)
	if err != nil {
		log.Printf("Failed to remove favorite: %v", err)
		return &gen.RemoveFavoriteResponse{Success: false}, nil
	}
	return &gen.RemoveFavoriteResponse{Success: true}, nil
}

func (s *server) GetFavorites(ctx context.Context, req *gen.GetFavoritesRequest) (*gen.GetFavoritesResponse, error) {
	if req.UserId == "" {
		return &gen.GetFavoritesResponse{Messages: nil}, nil
	}
	favs, err := s.db.GetFavorites(req.UserId)
	if err != nil {
		log.Printf("Failed to get favorites: %v", err)
		return &gen.GetFavoritesResponse{Messages: nil}, nil
	}

	var messages []*gen.Message
	for _, m := range favs {
		decryptedText, err := decrypt(m.Encrypted)
		if err != nil {
			decryptedText = "не удалось расшифровать"
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

		// Parse image URLs from JSON
		var imageURLs []string
		if m.ImageURLs != "" && m.ImageURLs != "[]" {
			json.Unmarshal([]byte(m.ImageURLs), &imageURLs)
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
			RoomId:             m.RoomID,
			IsRead:             m.IsRead,
			AvatarUrl:          m.AvatarURL,
			ImageUrl:           m.ImageURL,
			ImageUrls:          imageURLs,
			Edited:             m.Edited,
			VoiceUrl:           m.VoiceURL,
			Duration:           m.Duration,
		})
	}

	return &gen.GetFavoritesResponse{Messages: messages}, nil
}

func (s *server) SaveFavoriteMessage(ctx context.Context, req *gen.Message) (*gen.AddFavoriteResponse, error) {
	if req.User == "" {
		return &gen.AddFavoriteResponse{Success: false, Message: "username required"}, nil
	}

	// 1. Generate ID and Timestamp
	req.Id = uuid.New().String()
	req.CreatedAt = timestamppb.Now()

	// 2. Encrypt text
	encryptedText, err := encrypt(req.Text)
	if err != nil {
		return &gen.AddFavoriteResponse{Success: false, Message: "encryption failed"}, nil
	}

	// 3. Get User UUID
	userID, err := s.db.GetUserIdByUsername(req.User)
	if err != nil {
		return &gen.AddFavoriteResponse{Success: false, Message: "user not found"}, nil
	}

	// 4. Save message to DB in a special "favorites" room for consistency
	// Room ID is "favorites_" + username
	favRoomID := "favorites_" + req.User
	imageURLsJSON := "[]"
	if len(req.ImageUrls) > 0 {
		imageURLsBytes, _ := json.Marshal(req.ImageUrls)
		imageURLsJSON = string(imageURLsBytes)
	}
	err = s.db.SaveMessage(req.Id, req.User, req.UserId, encryptedText, req.CreatedAt.AsTime(), req.RepliedToMessageId, req.RepliedToUser, req.RepliedToText, favRoomID, req.ImageUrl, imageURLsJSON, req.VoiceUrl, req.Duration)
	if err != nil {
		return &gen.AddFavoriteResponse{Success: false, Message: "failed to save message"}, nil
	}

	// 5. Add to favorites table
	err = s.db.AddFavorite(userID, req.Id)
	if err != nil {
		return &gen.AddFavoriteResponse{Success: false, Message: "failed to link favorite"}, nil
	}

	// 6. Broadcast to favorites room for live update
	// Get reactions for the message we just saved (should be empty but good for consistency)
	req.RoomId = favRoomID
	s.hub.Broadcast(req)

	return &gen.AddFavoriteResponse{Success: true}, nil
}

func (s *server) GetDevices(ctx context.Context, req *gen.GetDevicesRequest) (*gen.GetDevicesResponse, error) {
	_ = ctx
	dbDevices, err := s.db.GetUserDevices(req.UserId)
	if err != nil {
		return nil, err
	}

	var pbDevices []*gen.DeviceInfo
	for _, d := range dbDevices {
		pbDevices = append(pbDevices, &gen.DeviceInfo{
			DeviceId:      d.DeviceID,
			DeviceName:    d.DeviceName,
			ClientVersion: d.ClientVersion,
			IpAddress:     d.IPAddress,
			LastSeenAt:    timestamppb.New(d.LastSeenAt),
		})
	}

	return &gen.GetDevicesResponse{Devices: pbDevices}, nil
}

func (s *server) DeleteDevice(ctx context.Context, req *gen.DeleteDeviceRequest) (*gen.DeleteDeviceResponse, error) {
	_ = ctx
	err := s.db.DeleteUserDevice(req.DeviceId, req.UserId)
	if err != nil {
		return &gen.DeleteDeviceResponse{Success: false, Message: err.Error()}, nil
	}

	// Tell connected client to logout
	s.hub.BroadcastGlobal(&gen.Message{
		User: "SYSTEM",
		Text: "FORCE_DISCONNECT_DEVICE:" + req.DeviceId,
	})

	return &gen.DeleteDeviceResponse{Success: true, Message: "Device removed"}, nil
}

func (s *server) DeleteOtherDevices(ctx context.Context, req *gen.DeleteDeviceRequest) (*gen.DeleteDeviceResponse, error) {
	_ = ctx
	err := s.db.DeleteOtherDevices(req.UserId, req.DeviceId)
	if err != nil {
		return &gen.DeleteDeviceResponse{Success: false, Message: err.Error()}, nil
	}

	// Tell all other devices of this user to logout
	s.hub.BroadcastGlobal(&gen.Message{
		User: "SYSTEM",
		Text: "FORCE_LOGOUT_EXCEPT:" + req.DeviceId,
	})

	return &gen.DeleteDeviceResponse{Success: true, Message: "All other sessions terminated"}, nil
}

// RequestPasswordReset initiates password reset by sending email with reset token
func (s *server) RequestPasswordReset(_ context.Context, req *gen.RequestPasswordResetRequest) (*gen.RequestPasswordResetResponse, error) {
	if req.Email == "" {
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Email is required"}, nil
	}

	// Check if SMTP is configured
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		return &gen.RequestPasswordResetResponse{Success: false, Message: "SMTP_NOT_CONFIGURED"}, nil
	}

	// Find user by email
	userId, err := s.db.GetUserIdByEmail(req.Email)
	if err != nil {
		// Don't reveal if email exists or not for security
		log.Printf("Password reset requested for non-existent email: %s", req.Email)
		return &gen.RequestPasswordResetResponse{Success: true, Message: "If the email exists, a reset link has been sent"}, nil
	}

	// Generate reset token
	token, err := GenerateResetToken()
	if err != nil {
		log.Printf("Failed to generate reset token: %v", err)
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to generate reset token"}, nil
	}

	// Token expires in 1 hour
	expiresAt := time.Now().Add(1 * time.Hour)

	// Save token to database
	err = s.db.CreatePasswordResetToken(token, userId, expiresAt)
	if err != nil {
		log.Printf("Failed to save reset token: %v", err)
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to save reset token"}, nil
	}

	// Send email with token
	err = SendPasswordResetEmail(req.Email, token)
	if err != nil {
		log.Printf("Failed to send reset email: %v", err)
		if err.Error() == "SMTP_NOT_CONFIGURED" {
			return &gen.RequestPasswordResetResponse{Success: false, Message: "SMTP_NOT_CONFIGURED"}, nil
		}
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to send reset email"}, nil
	}

	log.Printf("Password reset initiated for email: %s", req.Email)
	return &gen.RequestPasswordResetResponse{Success: true, Message: "If the email exists, a reset link has been sent"}, nil
}

// ResetPassword resets user password using valid token
func (s *server) ResetPassword(_ context.Context, req *gen.ResetPasswordRequest) (*gen.ResetPasswordResponse, error) {
	if req.Token == "" || req.NewPassword == "" {
		return &gen.ResetPasswordResponse{Success: false, Message: "Token and new password are required"}, nil
	}

	// Validate token and get user ID
	userId, err := s.db.ValidatePasswordResetToken(req.Token)
	if err != nil {
		log.Printf("Invalid or expired reset token: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "Invalid or expired reset token"}, nil
	}

	// Get username from user ID
	var username string
	err = s.db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, userId).Scan(&username)
	if err != nil {
		log.Printf("Failed to get username from user ID: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "User not found"}, nil
	}

	// Update password
	err = s.db.UpdatePassword(username, req.NewPassword)
	if err != nil {
		log.Printf("Failed to update password: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "Failed to update password"}, nil
	}

	// Delete used token
	_ = s.db.DeletePasswordResetToken(req.Token)

	log.Printf("Password reset successful for user: %s", username)
	return &gen.ResetPasswordResponse{Success: true, Message: "Password reset successfully"}, nil
}
