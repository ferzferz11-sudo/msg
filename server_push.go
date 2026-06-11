package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"firebase.google.com/go/v4/messaging"
)

func (s *server) RegisterToken(_ context.Context, req *gen.TokenRequest) (*gen.TokenResponse, error) {
	username := req.User
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	err := s.db.SaveUserToken(username, req.Token, req.PushEnabled)
	if err != nil {
		log.Printf("Failed to save token for %s: %v", username, err)
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
		username, displayToken, receiveStatus, req.PushEnabled)
	return &gen.TokenResponse{Success: true}, nil
}

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

func (s *server) saveConferenceSystemMessage(roomID, text, senderName, senderId string) {
	msgId := "conf_" + roomID // Stable ID for live updates
	createdAt := time.Now().UTC()
	displayText := "📹 " + text

	// Get participant count if active
	participants := s.hub.GetConferenceParticipants(roomID)
	if participants != nil && len(participants) > 0 {
		displayText = fmt.Sprintf("📹 Конференция: %d участников. (Войти)", len(participants))
	} else if strings.Contains(text, "завершена") {
		displayText = "📹 Конференция завершена"
	}

	// Save to DB using the performer's name instead of SYSTEM if available
	user := "SYSTEM"
	uid := ""
	if senderName != "" {
		user = senderName
		uid = senderId
	}

	// Encrypt for database
	encryptedText, _ := encrypt(displayText)

	// Save to DB (now supports ON CONFLICT update)
	err := s.db.SaveMessage(msgId, user, uid, encryptedText, createdAt, "", "", "", roomID, "", "[]", "", 0)
	if err != nil {
		log.Printf("[CONF] Failed to save call system message: %v", err)
		return
	}

	// Broadcast to the room
	broadcastMsg := &gen.Message{
		Id:        msgId,
		User:      user,
		UserId:    uid,
		Text:      displayText,
		CreatedAt: timestamppb.New(createdAt),
		RoomId:    roomID,
	}
	s.hub.Broadcast(broadcastMsg)
}

func (s *server) broadcastConferenceStatus(roomID string) {
	participants := s.hub.GetConferenceParticipants(roomID)
	invited := s.hub.GetConferenceInvited(roomID)
	creatorID := s.hub.GetConferenceCreator(roomID)
	topic := s.hub.GetConferenceTopic(roomID)
	startTime := s.hub.GetConferenceStartTime(roomID)

	response := map[string]interface{}{
		"participants": participants,
		"invited":      invited,
		"creator_id":   creatorID,
		"topic":        topic,
		"start_time":   startTime.UnixMilli(),
	}
	responseJSON, _ := json.Marshal(response)

	msg := &gen.CallMessage{
		Type:    gen.CallMessage_JOIN_CONFERENCE, // We reuse JOIN_CONFERENCE for status updates
		RoomId:  roomID,
		Payload: string(responseJSON),
	}

	members, _ := s.db.GetChatParticipants(roomID)
	var memberIDs []string
	for _, m := range members {
		memberIDs = append(memberIDs, s.resolveUserId(m))
	}
	s.hub.BroadcastConference(msg, memberIDs)
}

func (s *server) sendConferencePush(targetUserID, text, roomID string, startTime time.Time) {
	// Implementation for FCM push
	log.Printf("[PUSH] Sending conference invitation to %s: %s (at %v)", targetUserID, text, startTime)

	// Construct push data
	data := map[string]string{
		"type":          "conference_invite",
		"room_id":       roomID,
		"text":          text,
		"start_time_ms": fmt.Sprintf("%d", startTime.UnixMilli()),
		"is_conference": "true",
	}

	s.sendPushInternal(targetUserID, "Новая конференция", text, data)
}

func (s *server) sendPushInternal(targetUserID, title, body string, data map[string]string) {
	// Look up token
	token, err := s.db.GetUserTokenByUserID(targetUserID)
	if err != nil || token == "" {
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		log.Printf("[FCM] Error getting messaging client: %v", err)
		return
	}

	pushMsg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	_, err = client.Send(ctx, pushMsg)
	if err != nil {
		log.Printf("[FCM] Error sending conference push: %v", err)
	}
}

func (s *server) saveCallSystemMessage(u1, u2, icon, text, senderName, senderId string) {
	chatID, err := s.db.GetDirectChatBetweenUsers(u1, u2)
	if err != nil {
		log.Printf("[CALL] Failed to find chat for system message: %v", err)
		return
	}

	msgId := uuid.New().String()
	// Use UTC for consistent timing across regions
	createdAt := time.Now().UTC()
	displayText := icon + " " + text

	// Save to DB using the performer's name instead of SYSTEM
	// This ensures the message appears as a bubble on the correct side
	encryptedText, _ := encrypt(displayText)
	err = s.db.SaveMessage(msgId, senderName, senderId, encryptedText, createdAt, "", "", "", chatID, "", "[]", "", 0)
	if err != nil {
		log.Printf("[CALL] Failed to save call system message: %v", err)
		return
	}

	// Broadcast to the room
	broadcastMsg := &gen.Message{
		Id:        msgId,
		User:      senderName,
		UserId:    senderId,
		Text:      displayText,
		CreatedAt: timestamppb.New(createdAt),
		RoomId:    chatID,
	}
	s.hub.Broadcast(broadcastMsg)
}

func (s *server) handleAbruptDisconnect(userId string) {
	log.Printf("[CALL] Handling abrupt disconnect for %s", userId)

	// Resolve userId to UUID if it's a username
	resolvedUserId := s.resolveUserId(userId)

	// Find all active/pending calls for this user
	activeCalls, err := s.db.GetActiveCallsByUser(resolvedUserId)
	if err != nil {
		log.Printf("[CALL] Failed to get active calls for %s: %v", userId, err)
		return
	}

	for _, call := range activeCalls {
		// Determine the other party
		otherPartyId := call.CallerID
		if call.CallerID == resolvedUserId {
			otherPartyId = call.ReceiverID
		}

		// Mark call as completed (ended due to disconnect)
		_ = s.db.UpdateCallStatus(call.CallID, "completed")

		// Send HANGUP signal to the other party via call stream
		hangupSignal := &gen.CallMessage{
			CallId:     call.CallID,
			SenderId:   resolvedUserId,
			ReceiverId: otherPartyId,
			Type:       gen.CallMessage_HANGUP,
		}

		// Try to deliver via hub to the receiver's call stream
		delivered := s.hub.BroadcastCall(hangupSignal)
		if !delivered {
			log.Printf("[CALL] HANGUP not delivered to %s for call %s (receiver offline)", otherPartyId, call.CallID)
		} else {
			log.Printf("[CALL] HANGUP sent to %s for call %s", otherPartyId, call.CallID)
		}

		// Also try to resolve username and send via their chat stream as system message
		otherUsername := s.resolveUsername(otherPartyId)
		if otherUsername != "" {
			// Send HANGUP back to the disconnected user too (in case they have multiple streams)
			hangupToSender := &gen.CallMessage{
				CallId:     call.CallID,
				SenderId:   otherPartyId,
				ReceiverId: resolvedUserId,
				Type:       gen.CallMessage_HANGUP,
			}
			s.hub.BroadcastCall(hangupToSender)

			// Save system message to chat
			senderName := s.resolveUsername(resolvedUserId)
			duration, _ := s.db.GetCallDuration(call.CallID)
			durationText := ""
			if duration > 0 {
				minutes := duration / 60
				seconds := duration % 60
				durationText = fmt.Sprintf(" (%d:%02d)", minutes, seconds)
			}
			s.saveCallSystemMessage(senderName, otherUsername, "📞↘️", "Соединение потеряно"+durationText, senderName, resolvedUserId)
		}
	}
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
