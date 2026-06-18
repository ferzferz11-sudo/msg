package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"firebase.google.com/go/v4/messaging"
)

func (s *server) RegisterToken(ctx context.Context, req *gen.TokenRequest) (*gen.TokenResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	var err error
	if isUUID(userID) {
		err = s.db.SaveUserTokenByUserID(userID, req.Token, req.PushEnabled)
	} else {
		err = s.db.SaveUserToken(userID, req.Token, req.PushEnabled)
	}

	if err != nil {
		logger.Infof("Failed to save token for %s: %v", userID, err)
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
		userID, displayToken, receiveStatus, req.PushEnabled)
	return &gen.TokenResponse{Success: true}, nil
}

func (s *server) sendPushNotification(userId, username, title, body, roomID string) {
	if s.firebaseApp == nil {
		s.logFCM("WARN", "Skip %s: Firebase not init", username)
		return
	}

	if s.hub.IsUserOnline(userId, username) {
		s.logFCM("INFO", "Skip %s: user is online", username)
		return
	}

	mutedChats, err := s.db.GetMutedChats(username)
	if err == nil {
		for _, mutedRoomID := range mutedChats {
			if mutedRoomID == roomID {
				s.logFCM("INFO", "Skip %s: Chat %s is muted", username, roomID)
				return
			}
		}
	}

	token, err := s.db.GetUserToken(username)
	if err != nil || token == "" {
		s.logFCM("WARN", "Skip %s: No token", username)
		return
	}

	if token == "DISABLED" {
		s.logFCM("INFO", "Skip %s: User disabled push", username)
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
		Android: &messaging.AndroidConfig{
			Priority:    "high",
			CollapseKey: roomID,
			TTL:         durationPtr(5 * time.Minute),
			Notification: &messaging.AndroidNotification{
				ChannelID: "lavender_messages",
				Priority:  messaging.PriorityHigh,
				Sound:     "default",
			},
		},
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		s.logFCM("ERROR", "Send to %s failed: %v", username, err)
		return
	}

	s.logFCM("SUCCESS", "Sent to %s", username)
}

func (s *server) saveConferenceSystemMessage(roomID, text, senderName, senderId string) {
	msgId := "conf_" + roomID
	createdAt := time.Now().UTC()
	displayText := "📹 " + text

	participants := s.hub.GetConferenceParticipants(roomID)
	if participants != nil && len(participants) > 0 {
		displayText = fmt.Sprintf("📹 Конференция: %d участников. (Войти)", len(participants))
	} else if strings.Contains(text, "завершена") {
		displayText = "📹 Конференция завершена"
	}

	user := "SYSTEM"
	uid := ""
	if senderName != "" {
		user = senderName
		uid = senderId
	}

	encryptedText, _ := encrypt(displayText)

	err := s.db.SaveMessage(msgId, user, uid, encryptedText, createdAt, "", "", "", roomID, "", "[]", "", 0)
	if err != nil {
		logger.Infof("[CONF] Failed to save call system message: %v", err)
		return
	}

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
		Type:    gen.CallMessage_JOIN_CONFERENCE,
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
	logger.Infof("[PUSH] Sending conference invitation to %s: %s (at %v)", targetUserID, text, startTime)

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
	token, err := s.db.GetUserTokenByUserID(targetUserID)
	if err != nil || token == "" {
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		logger.Errorf("[FCM] Error getting messaging client: %v", err)
		return
	}

	pushMsg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	_, err = client.Send(ctx, pushMsg)
	if err != nil {
		logger.Errorf("[FCM] Error sending conference push: %v", err)
	}
}

func (s *server) saveCallSystemMessage(u1, u2, icon, text, senderName, senderId string) {
	chatID, err := s.db.GetDirectChatBetweenUsers(u1, u2)
	if err != nil {
		logger.Infof("[CALL] Failed to find chat for system message: %v", err)
		return
	}

	msgId := uuid.New().String()
	createdAt := time.Now().UTC()
	displayText := icon + " " + text

	encryptedText, _ := encrypt(displayText)
	err = s.db.SaveMessage(msgId, senderName, senderId, encryptedText, createdAt, "", "", "", chatID, "", "[]", "", 0)
	if err != nil {
		logger.Infof("[CALL] Failed to save call system message: %v", err)
		return
	}

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
	logger.Infof("[CALL] Handling abrupt disconnect for %s", userId)

	resolvedUserId := s.resolveUserId(userId)

	activeCalls, err := s.db.GetActiveCallsByUser(resolvedUserId)
	if err != nil {
		logger.Infof("[CALL] Failed to get active calls for %s: %v", userId, err)
		return
	}

	for _, call := range activeCalls {
		otherPartyId := call.CallerID
		if call.CallerID == resolvedUserId {
			otherPartyId = call.ReceiverID
		}

		_ = s.db.UpdateCallStatus(call.CallID, "completed")

		hangupSignal := &gen.CallMessage{
			CallId:     call.CallID,
			SenderId:   resolvedUserId,
			ReceiverId: otherPartyId,
			Type:       gen.CallMessage_HANGUP,
		}

		delivered := s.hub.BroadcastCall(hangupSignal)
		if !delivered {
			logger.Infof("[CALL] HANGUP not delivered to %s for call %s (receiver offline)", otherPartyId, call.CallID)
		} else {
			logger.Infof("[CALL] HANGUP sent to %s for call %s", otherPartyId, call.CallID)
		}

		otherUsername := s.resolveUsername(otherPartyId)
		if otherUsername != "" {
			hangupToSender := &gen.CallMessage{
				CallId:     call.CallID,
				SenderId:   otherPartyId,
				ReceiverId: resolvedUserId,
				Type:       gen.CallMessage_HANGUP,
			}
			s.hub.BroadcastCall(hangupToSender)

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

	token, err := s.db.GetUserTokenByUserID(receiverId)
	if err != nil || token == "" || token == "DISABLED" {
		return
	}

	ctx := context.Background()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		return
	}

	senderUsername := s.resolveUsername(senderName)

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

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
