package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
			"body":    truncateForFCM(body),
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

type pushTarget struct {
	UserId   string
	Username string
}

func (s *server) sendBatchPushNotifications(targets []pushTarget, title, body, roomID string) {
	if s.firebaseApp == nil || len(targets) == 0 {
		return
	}

	mutedSet := make(map[string]bool)
	for _, t := range targets {
		chats, err := s.db.GetMutedChats(t.Username)
		if err == nil {
			for _, c := range chats {
				if c == roomID {
					mutedSet[t.UserId] = true
				}
			}
		}
	}

	var tokens []string
	var tokenUserIDs []string
	for _, t := range targets {
		if s.hub.IsUserOnline(t.UserId, t.Username) {
			continue
		}
		if mutedSet[t.UserId] {
			continue
		}
		token, err := s.db.GetUserTokenByUserID(t.UserId)
		if err != nil || token == "" || token == "DISABLED" {
			continue
		}
		tokens = append(tokens, token)
		tokenUserIDs = append(tokenUserIDs, t.UserId)
	}

	if len(tokens) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		s.logFCM("ERROR", "Batch push client err: %v", err)
		return
	}

	const chunkSize = 500
	for i := 0; i < len(tokens); i += chunkSize {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := tokens[i:end]
		chunkIDs := tokenUserIDs[i:end]

		s.sendMulticastWithRetry(client, chunk, chunkIDs, title, body, roomID)
	}
}

func (s *server) sendMulticastWithRetry(client *messaging.Client, tokens, userIDs []string, title, body, roomID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: map[string]string{
			"title":   title,
			"body":    truncateForFCM(body),
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

	const maxRetries = 3
	var resp *messaging.BatchResponse
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * time.Second)
		}
		resp, err = client.SendEachForMulticast(ctx, msg)
		if err == nil {
			break
		}
		if s.isFirebaseCredentialError(err) {
			s.logErrorOnce("fcm_creds", "CRITICAL: Firebase credentials invalid — push disabled until fixed. Replace credentials and restart.")
			return
		}
		st, ok := status.FromError(err)
		if !ok || (st.Code() != codes.Unavailable && st.Code() != codes.ResourceExhausted) {
			s.logFCM("ERROR", "Batch push fatal error: %v", err)
			return
		}
	}

	if err != nil {
		s.logFCM("ERROR", "Batch push failed after retries: %v", err)
		return
	}

	credentialErrors := 0
	for _, r := range resp.Responses {
		if r.Error != nil && s.isFirebaseCredentialError(r.Error) {
			credentialErrors++
		}
	}
	if credentialErrors == len(resp.Responses) && len(resp.Responses) > 0 {
		s.logErrorOnce("fcm_creds_batch", "CRITICAL: All push failed — Firebase credentials invalid. Push disabled until fixed.")
		return
	}

	if resp.FailureCount == 0 {
		s.logFCM("SUCCESS", "Batch push sent: %d success", len(tokens))
	} else {
		s.logFCM("WARN", "Batch push: %d success, %d failure out of %d",
			len(tokens)-resp.FailureCount, resp.FailureCount, len(tokens))
	}

	for i, r := range resp.Responses {
		if r.Error != nil {
			if s.isFirebaseCredentialError(r.Error) {
				continue
			}
			s.logFCM("WARN", "Push to %s failed: %v", userIDs[i], r.Error)
			if s.isInvalidTokenError(r.Error) {
				_ = s.db.DeleteUserTokenByUserID(userIDs[i])
			}
		}
	}
}

func (s *server) isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNREGISTERED") ||
		strings.Contains(errStr, "INVALID_ARGUMENT") ||
		strings.Contains(errStr, "registration token not registered") ||
		strings.Contains(errStr, "Requested entity was not found")
}

func (s *server) isFirebaseCredentialError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Invalid JWT Signature") ||
		strings.Contains(errStr, "invalid_grant") ||
		strings.Contains(errStr, "cannot fetch token")
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

	v2Row := &MessageRowV2{
		ID:          msgId,
		RoomID:      roomID,
		SenderID:    uid,
		ContentType: "text",
		Text:        displayText,
		IsRead:      false,
		CreatedAt:   createdAt,
	}
	if uid == "" {
		v2Row.SenderID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.SaveMessageV2(v2Row)
	if err != nil {
		logger.Infof("[CONF] Failed to save call system message: %v", err)
		return
	}

	_, _ = s.db.Exec(`UPDATE chats SET last_message_text=$1, last_message_time=$2, last_message_username=$3, last_message_has_image=$4 WHERE id=$5`,
		displayText, createdAt, user, false, roomID)

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
		memberIDs = append(memberIDs, m)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
		Data: func() map[string]string {
			safe := make(map[string]string, len(data))
			for k, v := range data {
				if k == "text" {
					safe[k] = truncateForFCM(v)
				} else {
					safe[k] = v
				}
			}
			return safe
		}(),
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

	v2Row := &MessageRowV2{
		ID:          msgId,
		RoomID:      chatID,
		SenderID:    senderId,
		ContentType: "text",
		Text:        displayText,
		IsRead:      false,
		CreatedAt:   createdAt,
	}
	if senderId == "" {
		v2Row.SenderID = "00000000-0000-0000-0000-000000000000"
	}
	err = s.db.SaveMessageV2(v2Row)
	if err != nil {
		logger.Infof("[CALL] Failed to save call system message: %v", err)
		return
	}

	_, _ = s.db.Exec(`UPDATE chats SET last_message_text=$1, last_message_time=$2, last_message_username=$3, last_message_has_image=$4 WHERE id=$5`,
		displayText, createdAt, senderName, false, chatID)

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

	activeCalls, err := s.db.GetActiveCallsByUser(userId)
	if err != nil {
		logger.Infof("[CALL] Failed to get active calls for %s: %v", userId, err)
		return
	}

	for _, call := range activeCalls {
		otherPartyId := call.CallerID
		if call.CallerID == userId {
			otherPartyId = call.ReceiverID
		}

		_ = s.db.UpdateCallStatus(call.CallID, "completed")

		hangupSignal := &gen.CallMessage{
			CallId:     call.CallID,
			SenderId:   userId,
			ReceiverId: otherPartyId,
			Type:       gen.CallMessage_HANGUP,
		}

		delivered := s.hub.BroadcastCall(hangupSignal)
		if !delivered {
			logger.Infof("[CALL] HANGUP not delivered to %s for call %s (receiver offline)", otherPartyId, call.CallID)
		} else {
			logger.Infof("[CALL] HANGUP sent to %s for call %s", otherPartyId, call.CallID)
		}

		otherUsername := resolveDisplayName(s.db, otherPartyId)
		if otherUsername != "" {
			hangupToSender := &gen.CallMessage{
				CallId:     call.CallID,
				SenderId:   otherPartyId,
				ReceiverId: userId,
				Type:       gen.CallMessage_HANGUP,
			}
			s.hub.BroadcastCall(hangupToSender)

			senderName := resolveDisplayName(s.db, userId)
			duration, _ := s.db.GetCallDuration(call.CallID)
			durationText := ""
			if duration > 0 {
				minutes := duration / 60
				seconds := duration % 60
				durationText = fmt.Sprintf(" (%d:%02d)", minutes, seconds)
			}
			s.saveCallSystemMessage(senderName, otherUsername, "📞↘️", "Соединение потеряно"+durationText, senderName, userId)
		}
	}
}

func (s *server) sendCallPushNotification(receiverId, senderId, callId string) {
	if s.firebaseApp == nil {
		return
	}

	token, err := s.db.GetUserTokenByUserID(receiverId)
	if err != nil || token == "" || token == "DISABLED" {
		return
	}

	senderName := resolveDisplayName(s.db, senderId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		return
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "Входящий звонок",
			Body:  senderName + " звонит вам",
		},
		Data: map[string]string{
			"type":        "VOIP_CALL",
			"call_id":     callId,
			"sender_id":   senderId,
			"sender_name": senderName,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "lavender_calls",
				Priority:  messaging.PriorityMax,
				Sound:     "default",
			},
		},
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		s.logFCM("ERROR", "Call Push to %s failed: %v", receiverId, err)
	} else {
		s.logFCM("SUCCESS", "Call Push sent to %s (from %s)", receiverId, senderName)
	}
}

func (s *server) sendCallEndedPushNotification(receiverId, senderId, callId string) {
	if s.firebaseApp == nil {
		return
	}
	token, err := s.db.GetUserTokenByUserID(receiverId)
	if err != nil || token == "" || token == "DISABLED" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := s.firebaseApp.Messaging(ctx)
	if err != nil {
		return
	}
	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "Звонок завершён",
			Body:  "Звонок окончен",
		},
		Data: map[string]string{
			"type":      "CALL_ENDED",
			"call_id":   callId,
			"sender_id": senderId,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "lavender_calls",
				Priority:  messaging.PriorityMax,
				Sound:     "default",
			},
		},
	}
	_, err = client.Send(ctx, message)
	if err != nil {
		s.logFCM("ERROR", "Call Ended Push to %s failed: %v", receiverId, err)
	} else {
		s.logFCM("SUCCESS", "Call Ended Push sent to %s", receiverId)
	}
}

func (s *server) broadcastOnlineUsers() {
	users := s.hub.GetOnlineUsers()
	usersJson, _ := json.Marshal(users)

	// Send to v1 clients (ChatStream)
	msg := &gen.Message{
		User:      "SYSTEM",
		Text:      "ONLINE_USERS_UPDATE:" + string(usersJson),
		Id:        uuid.New().String(),
		CreatedAt: timestamppb.Now(),
	}
	s.hub.BroadcastGlobal(msg)

	// Send to v2 clients (ChatV2)
	s.hub.BroadcastGlobalV2("ONLINE_USERS_UPDATE", string(usersJson))
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

const maxFCMDataBodyLen = 3500

func truncateForFCM(s string) string {
	if len(s) <= maxFCMDataBodyLen {
		return s
	}
	return s[:maxFCMDataBodyLen] + "..."
}
