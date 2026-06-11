package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

			// Clear grace period on successful reconnect
			s.hub.ClearGracePeriod(msg.User)

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
			if err := stream.Send(serverInfoMsg); err != nil { log.Printf("Failed to send server info: %v", err) }

			// Inform the user about their admin status (check by user_id first, then username)
			if s.db.IsSuperAdmin(connectedUserID) || s.db.IsSuperAdmin(msg.User) {
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

		// Bot command detection — messages starting with "/"
		if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
			log.Printf("[BotCommand] %s in %s: %s", msg.User, roomID, msg.Text)

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
				log.Printf("[BotCommand] Error: %v", err)
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
				log.Printf("[%s] in %s: [E2EE image] (ImageURLs: %v)", msg.User, roomID, msg.ImageUrls)
			} else if msg.ImageUrl != "" {
				log.Printf("[%s] in %s: [E2EE image] (ImageURL: %s)", msg.User, roomID, msg.ImageUrl)
			} else if msg.VoiceUrl != "" {
				log.Printf("[%s] in %s: [E2EE voice] (%d seconds)", msg.User, roomID, msg.Duration)
			} else {
				log.Printf("[%s] in %s: [E2EE encrypted message]", msg.User, roomID)
			}
		} else {
			if len(msg.ImageUrls) > 0 {
				log.Printf("[%s] in %s: %s (ImageURLs: %v)", msg.User, roomID, msg.Text, msg.ImageUrls)
			} else if msg.ImageUrl != "" {
				log.Printf("[%s] in %s: %s (ImageURL: %s)", msg.User, roomID, msg.Text, msg.ImageUrl)
			} else if msg.VoiceUrl != "" {
				log.Printf("[%s] in %s: Voice message (%d seconds) - %s", msg.User, roomID, msg.Duration, msg.VoiceUrl)
			} else {
				log.Printf("[%s] in %s: %s", msg.User, roomID, msg.Text)
			}
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
			// For E2EE messages, client already encrypted the payload
			// Don't double-encrypt with server key
			var encryptedText []byte
			if msg.IsE2Ee {
				encryptedText = []byte(msg.E2EePayload)
			} else {
				var err error
				encryptedText, err = encrypt(msg.Text)
				if err != nil {
					log.Printf("Failed to encrypt message: %v", err)
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
				// For secret chats, don't leak message content in push
				pushText := msg.Text
				if chat.IsSecret {
					pushText = "New encrypted message"
				}
				s.sendPushNotification(user.Username, msg.User, pushText, roomID)
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

		username := req.Username
		if req.UserId != "" {
			resolved := s.resolveUsername(req.UserId)
			if resolved != "" {
				username = resolved
			}
		}

		currentTypingUser = username
		currentRoomID = req.RoomId

		// Broadcast typing signal to others
		signal := &gen.TypingSignal{
			RoomId:   req.RoomId,
			Username: username,
			IsTyping: req.IsTyping,
		}
		s.hub.BroadcastTyping(signal)
	}
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
			log.Printf("[CALL] Accepted: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "active")
		case gen.CallMessage_REJECT:
			log.Printf("[CALL] Rejected: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "rejected")
			// For missed calls, the message should be attributed to the CALLER (ReceiverId in REJECT signal)
			// so that the person who missed it sees it as an incoming message (on the left)
			s.saveCallSystemMessage(senderName, receiverName, "📞↘️", "Пропущенный вызов", receiverName, msg.ReceiverId)
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
			// For completed calls, attribute to the original CALLER for history consistency
			s.saveCallSystemMessage(senderName, receiverName, "📞↗️", "Звонок завершен"+durationText, receiverName, msg.ReceiverId)

		case gen.CallMessage_INITIATE_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
				// Initial setup doesn't broadcast to everyone yet, just creates the lobby
				log.Printf("[CONF] Lobby created for room %s by %s", msg.RoomId, msg.SenderId)
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
				startTimeMs := int64(data["start_time"].(float64))
				startTime := time.UnixMilli(startTimeMs)

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
			log.Printf("[CALL] Warning: Signal %s not delivered to %s (offline)",
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
