package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
		if err != nil {
			return err
		}

		msg.SenderName = resolveDisplayName(s.db, msg.SenderId)
		msg.ReceiverName = resolveDisplayName(s.db, msg.ReceiverId)

		if currentUserId == "" && msg.SenderId != "" {
			currentUserId = msg.SenderId
			s.hub.UpdateCallName(stream, currentUserId)
			username := resolveDisplayName(s.db, currentUserId)
			logger.Infof("[CALL] Stream identified: %s (%s)", currentUserId, username)
		}

		// Legacy: ICE_CANDIDATE with "IDENTITY" payload (old clients)
		if msg.ReceiverId == "" && msg.Payload == "IDENTITY" {
			continue
		}

		// IDENTITY signal: lobby registration (new clients)
		if msg.Type == gen.CallMessage_IDENTITY {
			logger.Infof("[CALL] Identity registered: %s (%s)", msg.SenderId, msg.SenderName)
			continue
		}

		senderName := resolveDisplayName(s.db, msg.SenderId)
		receiverName := resolveDisplayName(s.db, msg.ReceiverId)
		logger.Infof("[CALL] Signal: %s | From: %s (%s) | To: %s (%s) | CallID: %s",
			msg.Type.String(), msg.SenderId, senderName, msg.ReceiverId, receiverName, msg.CallId)

		switch msg.Type {
		case gen.CallMessage_INITIATE:
			if !isUUID(msg.ReceiverId) {
				uid, err := s.db.GetUserIDByUsername(msg.ReceiverId)
				if err == nil && uid != "" {
					msg.ReceiverId = uid
					msg.ReceiverName = resolveDisplayName(s.db, uid)
				}
			}
			callId, err := s.db.CreateCall(msg.SenderId, msg.ReceiverId, "video", "")
			if err != nil {
				logger.Errorf("[CALL] Failed to create call in DB: %v", err)
			} else {
				msg.CallId = callId
				logger.Infof("[CALL] New call created: %s", callId)
			}

			delivered := s.hub.BroadcastCall(msg)
			logger.Infof("[CALL] INITIATE from %s to %s delivered: %v", msg.SenderId, msg.ReceiverId, delivered)

			originalReceiver := msg.ReceiverId
			msg.ReceiverId = msg.SenderId
			s.hub.BroadcastCall(msg)
			msg.ReceiverId = originalReceiver

			if !delivered {
				s.sendCallPushNotification(msg.ReceiverId, msg.SenderId, msg.CallId)
			}
			if msg.IsVideo {
				s.saveCallSystemMessage(senderName, receiverName, "📹", "Видеозвонок", senderName, msg.SenderId)
			} else {
				s.saveCallSystemMessage(senderName, receiverName, "📞", "Звонок", senderName, msg.SenderId)
			}
			continue

		case gen.CallMessage_ACCEPT:
			if msg.CallId == "" {
				continue
			}
			if status, err := s.db.GetCallStatus(msg.CallId); err == nil && (status == "completed" || status == "rejected") {
				logger.Warnf("[CALL] Ignoring stale ACCEPT for %s (status: %s)", msg.CallId, status)
				continue
			}
			logger.Infof("[CALL] Accepted: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "active")
		case gen.CallMessage_REJECT:
			if msg.CallId == "" {
				continue
			}
			if status, err := s.db.GetCallStatus(msg.CallId); err == nil && (status == "completed" || status == "rejected") {
				logger.Warnf("[CALL] Ignoring stale REJECT for %s (status: %s)", msg.CallId, status)
				continue
			}
			logger.Infof("[CALL] Rejected: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "rejected")
			s.saveCallSystemMessage(senderName, receiverName, "📞↘️", "Пропущенный вызов", receiverName, msg.ReceiverId)
		case gen.CallMessage_HANGUP:
			logger.Infof("[CALL] Hung up: %s", msg.CallId)
			_ = s.db.UpdateCallStatus(msg.CallId, "completed")

			duration, err := s.db.GetCallDuration(msg.CallId)
			if err == nil && duration > 0 {
				minutes := duration / 60
				seconds := duration % 60
				s.saveCallSystemMessage(senderName, receiverName, "📞↗️", fmt.Sprintf("Звонок завершен (%d:%02d)", minutes, seconds), receiverName, msg.ReceiverId)
			} else {
				s.saveCallSystemMessage(senderName, receiverName, "📞↘️", "Не отвечено", receiverName, msg.ReceiverId)
			}

		case gen.CallMessage_INITIATE_CONFERENCE:
			if s.hub.GetConferenceCreator(msg.RoomId) == "" {
				s.hub.InitiateConference(msg.RoomId, msg.SenderId, msg.SenderName)
				logger.Infof("[CONF] Lobby created for room %s by %s", msg.RoomId, msg.SenderId)
			}
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
			s.hub.InviteToConference(msg.RoomId, msg.ReceiverId, msg.ReceiverName)
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

		// Resolve ReceiverId to UUID if it's a username (client may send username)
		if !isUUID(msg.ReceiverId) {
			if uid, err := s.db.GetUserIDByUsername(msg.ReceiverId); err == nil && uid != "" {
				msg.ReceiverId = uid
			}
		}

		delivered := s.hub.BroadcastCall(msg)
		logger.Infof("[CALL] Signal %s delivered to %s: %v", msg.Type.String(), msg.ReceiverId, delivered)
		if !delivered {
			logger.Warnf("[CALL] WARNING: Signal %s NOT delivered to %s — receiver has no active call stream",
				msg.Type.String(), msg.ReceiverId)
			if msg.Type == gen.CallMessage_HANGUP || msg.Type == gen.CallMessage_REJECT {
				s.sendCallEndedPushNotification(msg.ReceiverId, msg.SenderId, msg.CallId)
			}
		}
	}
}

func (s *server) GetClients(_ context.Context, _ *gen.ClientListRequest) (*gen.ClientListResponse, error) {
	users := s.hub.GetOnlineUsers()
	return &gen.ClientListResponse{Clients: users}, nil
}

func (s *server) cleanupRecentMsgs() {
	s.recentMsgs.Range(func(key, value interface{}) bool {
		if time.Since(value.(time.Time)) > 10*time.Second {
			s.recentMsgs.Delete(key)
		}
		return true
	})
}

func (s *server) cleanupRecentMsgsForRoom(roomID string) {
	s.recentMsgs.Range(func(key, value interface{}) bool {
		k := key.(string)
		if strings.Contains(k, roomID) {
			s.recentMsgs.Delete(key)
		}
		return true
	})
}
