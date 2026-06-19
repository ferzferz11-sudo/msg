package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *server) Chat(stream gen.ChatService_ChatServer) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic recovered in Chat stream: %v", r)
		}
	}()

	var connectedUser string = "Anonymous"
	var connectedUserID string = ""
	var currentRoom string = ""

	// Register the new client stream with the hub
	s.hub.Register(stream)
	defer func() {
		// Unregister the client when the connection ends
		s.hub.Unregister(stream)
		logger.Infof("Stream for %s closed", connectedUser)
	}()

	// Track auth method for this stream
	authDone := false

	for {
		// Receive message from client
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// ===== Authentication =====
		// First message with JWT token (v2 auth only)
		if !authDone && msg.JwtToken != "" {
			var authSuccess bool
			var authErr string

			// ChatStream v2: JWT Bearer token auth
			claims, err := ValidateToken(msg.JwtToken)
			if err != nil {
				authErr = fmt.Sprintf("JWT validation failed: %v", err)
				authSuccess = false
			} else if claims.Type != "access" {
				authErr = "expected access token, got refresh token"
				authSuccess = false
			} else {
				// JWT valid — extract user info
				connectedUserID = claims.UserID
				connectedUser = claims.Username

				// Update hub
				trimmedUser := strings.TrimSpace(connectedUser)
				msg.User = trimmedUser
				connectedUser = trimmedUser
				s.hub.UpdateName(stream, trimmedUser)
				s.hub.SetUserId(stream, connectedUserID)
				s.hub.SetAuthenticated(stream, true)

				authSuccess = true
			}

			if !authSuccess {
				logger.Errorf("ChatStream auth failed: %s", authErr)
				authFailMsg := &gen.Message{
					User:      "SYSTEM",
					Text:      "AUTH_FAILED",
					Id:        uuid.New().String(),
					CreatedAt: timestamppb.Now(),
				}
				if err := stream.Send(authFailMsg); err != nil {
					logger.Errorf("Failed to send auth failed message: %v", err)
				}
				return fmt.Errorf("authentication failed: %s", authErr)
			}

			// Post-auth setup
			authDone = true

			// Clear grace period on successful reconnect
			s.hub.ClearGracePeriod(connectedUser)

			// Fetch and store user ID (for JWT path it's already set)
			if connectedUserID == "" {
				uid, _ := s.db.GetUserIdByUsername(connectedUser)
				connectedUserID = uid
			}

			// Log successful auth
			clientVer := msg.ClientVersion
			if clientVer == "" {
				clientVer = s.hub.GetClientVersion(connectedUserID)
			}
			logger.Infof("Auth success: %s (JWT) v=%s device=%s signal=%s", connectedUser, clientVer, msg.DeviceId, msg.RoomId)

			// Update last client version and last seen timestamp in DB
			if msg.ClientVersion != "" {
				_ = s.db.UpdateClientVersion(connectedUser, msg.ClientVersion)
				s.hub.SetClientVersion(connectedUserID, msg.ClientVersion)
			} else {
				_ = s.db.UpdateLastSeen(connectedUser)
			}

			// Send server info
			serverInfoMsg := &gen.Message{
				User:      "SYSTEM",
				Text:      "SERVER_INFO:" + ServerVersion,
				Id:        uuid.New().String(),
				CreatedAt: timestamppb.Now(),
			}
			if err := stream.Send(serverInfoMsg); err != nil {
				logger.Errorf("Failed to send server info: %v", err)
			}

			// Inform the user about their admin status
			if s.db.IsSuperAdmin(connectedUserID) || s.db.IsSuperAdmin(connectedUser) {
				statusMsg := &gen.Message{
					User:         "SYSTEM",
					Text:         "SET_SUPER_ADMIN",
					IsSuperAdmin: true,
					Id:           uuid.New().String(),
					CreatedAt:    timestamppb.Now(),
				}
				if err := stream.Send(statusMsg); err != nil {
					logger.Errorf("Failed to send super admin status: %v", err)
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
					logger.Errorf("Failed to register device %s for %s (ID: %s): %v", msg.DeviceId, connectedUser, connectedUserID, err)
				} else {
					logger.Infof("Device registered: %s (%s) for %s", msg.DeviceName, msg.DeviceId, connectedUser)
				}
			}

			// Clear sensitive fields from message before broadcasting
			msg.Password = ""
			msg.JwtToken = ""
		}

		// Deduplication logic for identical rapid messages
		// Prevents double-posting from some source apps (e.g. Google Photos)
		if msg.Text != "" || msg.ImageUrl != "" {
			msgHash := fmt.Sprintf("%s:%s:%s:%s", msg.User, msg.RoomId, msg.Text, msg.ImageUrl)
			now := time.Now()
			if lastTime, ok := s.recentMsgs.Load(msgHash); ok {
				if now.Sub(lastTime.(time.Time)) < 2*time.Second {
					logger.Infof("Msg deduplicated: %s in %s", msg.User, msg.RoomId)
					continue
				}
			}
			s.recentMsgs.Store(msgHash, now)
		}

		// Reject messages from unauthenticated streams (except first auth message)
		// Temporarily disabled to debug authentication issues
		/*if !s.hub.IsAuthenticated(stream) && msg.Password == "" {
			logger.Info("Rejected message from unauthenticated stream")
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

		// Bot command detection — messages starting with "/"
		if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
			logger.Infof("[BotCommand] %s in %s: %s", msg.User, roomID, msg.Text)

			// Parse command and args
			parts := strings.Fields(msg.Text)
			cmd := parts[0]
			var args []string
			if len(parts) > 1 {
				args = parts[1:]
			}

			// Process bot command
			botReq := &gen.BotCommandRequest{
				UserId:   connectedUserID,
				Username: connectedUser,
				ChatId:   roomID,
				Command:  cmd,
				Args:     args,
			}
			botResp, err := s.ProcessBotCommand(nil, botReq)
			if err != nil {
				logger.Errorf("[BotCommand] Error: %v", err)
			} else if botResp != nil {
				// Send bot response as a system message to the room
				botMsg := &gen.Message{
					User:      "🤖 OWL Bot",
					Text:      botResp.ResponseText,
					Id:        fmt.Sprintf("bot_%d", time.Now().UnixNano()),
					CreatedAt: timestamppb.Now(),
					RoomId:    roomID,
				}
				if botResp.IsError && botResp.ErrorMessage != "" {
					botMsg.Text = "⚠️ " + botResp.ErrorMessage
				}
				// Broadcast to room
				s.hub.Broadcast(botMsg)
			}
			// Don't save bot commands to DB, just respond
			continue
		}

		// Log message — for E2EE/secret chats, never log message text
		if msg.IsE2Ee {
			if len(msg.ImageUrls) > 0 {
				logger.Infof("[%s] in %s: [E2EE image] (ImageURLs: %v)", msg.User, roomID, msg.ImageUrls)
			} else if msg.ImageUrl != "" {
				logger.Infof("[%s] in %s: [E2EE image] (ImageURL: %s)", msg.User, roomID, msg.ImageUrl)
			} else if msg.VoiceUrl != "" {
				logger.Infof("[%s] in %s: [E2EE voice] (%d seconds)", msg.User, roomID, msg.Duration)
			} else {
				logger.Infof("[%s] in %s: [E2EE encrypted message]", msg.User, roomID)
			}
		} else {
			if len(msg.ImageUrls) > 0 {
				logger.Infof("[%s] in %s: %s (ImageURLs: %v)", msg.User, roomID, msg.Text, msg.ImageUrls)
			} else if msg.ImageUrl != "" {
				logger.Infof("[%s] in %s: %s (ImageURL: %s)", msg.User, roomID, msg.Text, msg.ImageUrl)
			} else if msg.VoiceUrl != "" {
				logger.Infof("[%s] in %s: Voice message (%d seconds) - %s", msg.User, roomID, msg.Duration, msg.VoiceUrl)
			} else {
				logger.Infof("[%s] in %s: %s", msg.User, roomID, msg.Text)
			}
		}

		if roomID == "" {
			logger.Infof("Skipping message with empty room ID from %s", msg.User)
			continue
		}

		// Skip join messages (don't save to database)
		if strings.HasSuffix(msg.Text, " joined") || strings.HasSuffix(msg.Text, " присоединился") {
			// Still broadcast but don't save to DB
			logger.Infof("Skipping join message from DB save: %s", msg.Text)
		} else {
			// For E2EE messages, client already encrypted the payload
			// Don't double-encrypt with server key
			var encryptedText []byte
			if msg.IsE2Ee {
				encryptedText = []byte(msg.E2EePayload)
			} else {
				var err error
				encryptedText, err = encrypt(msg.Text)
				if err != nil {
					logger.Errorf("Failed to encrypt message: %v", err)
					continue
				}
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
			err = s.db.SaveMessage(msg.Id, msg.User, msg.UserId, encryptedText, msg.CreatedAt.AsTime(), msg.RepliedToMessageId, msg.RepliedToUser, msg.RepliedToText, roomID, imageURL, imageURLsJSON, voiceURL, duration, msg.IsE2Ee)
			if err != nil {
				logger.Errorf("Failed to save msg: %v", err)
			} else {
				logger.Infof("Msg saved: %s (%s)", msg.Id, roomID)
			}
		}

		// Update user's current room in hub
		s.hub.SetRoom(stream, roomID)

		// Get user's avatar URL
		avatarURL, err := s.db.GetUserAvatar(msg.User)
		if err != nil {
			logger.Errorf("Failed to get avatar for %s: %v", msg.User, err)
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
		if !senderNotifiesOthers {
			logger.Infof("Push skip: %s has disabled outgoing notifications", msg.User)
		} else {
			// Get chat participants ONCE (not per user)
			chat, err := s.db.GetChat(roomID)
			if err != nil {
				logger.Errorf("Failed to get chat for push: %v", err)
			} else {
				var participants []string
				json.Unmarshal([]byte(chat.Participants), &participants)

				// Build participant set for O(1) lookup
				participantSet := make(map[string]bool, len(participants))
				for _, p := range participants {
					participantSet[p] = true
				}

				// Get all users, but only send to participants
				allUsers, err := s.db.GetAllUsers()
				if err != nil {
					logger.Errorf("Failed to get all users for push notifications: %v", err)
			} else {
				var targets []pushTarget
				for _, user := range allUsers {
					if user.Username == msg.User {
						continue
					}
					if !participantSet[user.Username] {
						continue
					}

					targets = append(targets, pushTarget{
						UserId:   user.UserId,
						Username: user.Username,
					})
				}

				if len(targets) > 0 {
					pushText := msg.Text
					if chat.IsSecret {
						pushText = "New encrypted message"
					}
					s.sendBatchPushNotifications(targets, msg.User, pushText, roomID)
				}
			}
			}
		}
	}
}

func (s *server) Typing(stream gen.ChatService_TypingServer) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic recovered in Typing stream: %v", r)
		}
	}()

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

		username := req.Username
		if req.UserId != "" {
			resolved := resolveDisplayName(s.db, req.UserId)
			if resolved != "" {
				username = resolved
			}
		}

		currentTypingUser = username
		currentRoomID = req.RoomId

		signal := &gen.TypingSignal{
			RoomId:   req.RoomId,
			Username: username,
			IsTyping: req.IsTyping,
		}
		s.hub.BroadcastTyping(signal)
	}
}

func (s *server) CallSession(stream gen.ChatService_CallSessionServer) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic recovered in CallSession stream: %v", r)
		}
	}()

	var currentUserId string
	s.hub.RegisterCall(stream)
	defer func() {
		s.hub.UnregisterCall(stream)
		if currentUserId != "" {
			username := resolveDisplayName(s.db, currentUserId)
			logger.Infof("[CALL] Stream closed: %s (%s)", currentUserId, username)
			s.handleAbruptDisconnect(currentUserId)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if err != context.Canceled && !strings.Contains(err.Error(), "transport is closing") {
				logger.Errorf("[CALL] Error receiving signal: %v", err)
			}
			return err
		}

		msg.SenderId = s.resolveUserId(msg.SenderId)
		msg.ReceiverId = s.resolveUserId(msg.ReceiverId)
		msg.SenderName = resolveDisplayName(s.db, msg.SenderId)
		msg.ReceiverName = resolveDisplayName(s.db, msg.ReceiverId)

		if currentUserId == "" && msg.SenderId != "" {
			currentUserId = msg.SenderId
			s.hub.UpdateCallName(stream, currentUserId)
			username := resolveDisplayName(s.db, currentUserId)
			logger.Infof("[CALL] Stream identified: %s (%s)", currentUserId, username)
		}

		if msg.ReceiverId == "" && msg.Payload == "IDENTITY" {
			continue
		}

		senderName := resolveDisplayName(s.db, msg.SenderId)
		receiverName := resolveDisplayName(s.db, msg.ReceiverId)
		logger.Infof("[CALL] Signal: %s | From: %s (%s) | To: %s (%s) | CallID: %s",
			msg.Type.String(), msg.SenderId, senderName, msg.ReceiverId, receiverName, msg.CallId)

		// Handle database updates based on message type
		switch msg.Type {
		case gen.CallMessage_INITIATE:
			callId, err := s.db.CreateCall(msg.SenderId, msg.ReceiverId, "video", "")
			if err != nil {
				logger.Errorf("[CALL] Failed to create call in DB: %v", err)
			} else {
				msg.CallId = callId
				logger.Infof("[CALL] New call created: %s", callId)
			}

			// 1. Route to receiver
			delivered := s.hub.BroadcastCall(msg)
			logger.Infof("[CALL] INITIATE from %s to %s delivered: %v", msg.SenderId, msg.ReceiverId, delivered)

			// 2. Route back to sender so they get the generated call_id
			originalReceiver := msg.ReceiverId
			msg.ReceiverId = msg.SenderId
			s.hub.BroadcastCall(msg)
			msg.ReceiverId = originalReceiver

			// 3. Send push to wake up receiver
			s.sendCallPushNotification(msg.ReceiverId, msg.SenderName, msg.CallId)

			// 4. Add system message to chat
			s.saveCallSystemMessage(senderName, receiverName, "📹", "Видеозвонок", senderName, msg.SenderId)
			continue

		case gen.CallMessage_ACCEPT:
			logger.Infof("[CALL] Accepted: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "active")
		case gen.CallMessage_REJECT:
			logger.Infof("[CALL] Rejected: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "rejected")
			// For missed calls, the message should be attributed to the CALLER (ReceiverId in REJECT signal)
			// so that the person who missed it sees it as an incoming message (on the left)
			s.saveCallSystemMessage(senderName, receiverName, "📞↘️", "Пропущенный вызов", receiverName, msg.ReceiverId)
		case gen.CallMessage_HANGUP:
			logger.Infof("[CALL] Hung up: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "completed")

			durationText := ""
			duration, err := s.db.GetCallDuration(msg.CallId)
			if err == nil && duration > 0 {
				minutes := duration / 60
				seconds := duration % 60
				durationText = fmt.Sprintf(" (%d:%02d)", minutes, seconds)
			}
			// For completed calls, attribute to the original CALLER for history consistency
			s.saveCallSystemMessage(senderName, receiverName, "📞↗️", "Звонок завершен"+durationText, receiverName, msg.ReceiverId)

		case gen.CallMessage_INITIATE_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
				// Initial setup doesn't broadcast to everyone yet, just creates the lobby
				logger.Infof("[CONF] Lobby created for room %s by %s", msg.RoomId, msg.SenderId)
			}
			// Respond to creator with the status
			s.broadcastConferenceStatus(msg.RoomId)
			continue

		case gen.CallMessage_JOIN_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
			}
			s.hub.JoinConference(msg.RoomId, msg.SenderId, msg.SenderName)
			s.saveConferenceSystemMessage(msg.RoomId, "Конференция", senderName, msg.SenderId)
			s.broadcastConferenceStatus(msg.RoomId)
			continue

		case gen.CallMessage_LEAVE_CONFERENCE:
			s.hub.LeaveConference(msg.RoomId, msg.SenderId)
			s.saveConferenceSystemMessage(msg.RoomId, "Конференция", senderName, msg.SenderId)
			s.broadcastConferenceStatus(msg.RoomId)
			continue

		case gen.CallMessage_INVITE_TO_CONFERENCE:
			// Just add to the invited list, don't send push yet
			targetUserID := msg.ReceiverId
			targetUserName := msg.ReceiverName
			s.hub.InviteToConference(msg.RoomId, targetUserID, targetUserName)
			s.broadcastConferenceStatus(msg.RoomId)
			continue

		case gen.CallMessage_REMOVE_FROM_CONFERENCE:
			s.hub.RemoveFromConference(msg.RoomId, msg.ReceiverId)
			s.broadcastConferenceStatus(msg.RoomId)
			continue

		case gen.CallMessage_UPDATE_CONFERENCE:
			if s.hub.IsConferenceCreator(msg.RoomId, msg.SenderId) {
				var data map[string]interface{}
				json.Unmarshal([]byte(msg.Payload), &data)
				topic := fmt.Sprintf("%v", data["topic"])
				startTimeMs, ok := data["start_time"].(float64)
				if !ok {
					startTimeMs = 0
				}
				startTime := time.UnixMilli(int64(startTimeMs))

				s.hub.UpdateConferenceMetadata(msg.RoomId, topic, startTime)

				// If trigger_notify is set, send pushes to all invited members
				if notify, ok := data["trigger_notify"].(bool); ok && notify {
					invited := s.hub.GetConferenceInvited(msg.RoomId)
					notificationText := fmt.Sprintf("Начало конференции: %s", topic)
					if topic == "" {
						notificationText = "Конференция начинается!"
					}
					for uid := range invited {
						s.sendConferencePush(uid, notificationText, msg.RoomId, startTime)
					}
				}

				s.broadcastConferenceStatus(msg.RoomId)
			}
			continue

		case gen.CallMessage_END_CONFERENCE:
			if s.hub.IsConferenceCreator(msg.RoomId, msg.SenderId) {
				s.hub.EndConference(msg.RoomId)
				s.saveConferenceSystemMessage(msg.RoomId, "Конференция завершена", senderName, msg.SenderId)
				s.broadcastConferenceStatus(msg.RoomId)
			}
			continue
		}

		// Broadcast WebRTC signals (OFFER, ANSWER, ICE) to partner
		delivered := s.hub.BroadcastCall(msg)
		if !delivered {
			logger.Warnf("[CALL] Warning: Signal %s not delivered to %s (offline)",
				msg.Type.String(), msg.ReceiverId)
		}
	}
}

func (s *server) GetClients(ctx context.Context, req *gen.ClientListRequest) (*gen.ClientListResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	_ = req // req is required by gRPC interface but not used here
	users := s.hub.GetOnlineUsers()
	return &gen.ClientListResponse{
		Clients: users,
	}, nil
}

// cleanupRecentMsgs removes dedup cache entries older than 10 seconds
func (s *server) cleanupRecentMsgs() {
	s.recentMsgs.Range(func(key, value interface{}) bool {
		if time.Since(value.(time.Time)) > 10*time.Second {
			s.recentMsgs.Delete(key)
		}
		return true
	})
}
