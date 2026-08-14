package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ChatV2 is a bidirectional stream for v2 messages (oneof payload).
func (s *server) ChatV2(stream gen.ChatService_ChatV2Server) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic recovered in ChatV2 stream: %v", r)
		}
	}()

	var connectedUserID string
	var connectedUser string
	var currentRoom string
	authDone := false

	s.hub.RegisterV2(stream)
	defer s.hub.UnregisterV2(stream)

	// Deadline detection: track last activity and close inactive streams
	const streamInactivityTimeout = 90 * time.Second
	lastActivityTime := time.Now()
	deadlineDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		logged := false
		for {
			select {
			case <-deadlineDone:
				return
			case <-stream.Context().Done():
				return
			case <-ticker.C:
				if time.Since(lastActivityTime) > streamInactivityTimeout {
					if !logged {
						logger.Infof("[ChatV2] Stream inactive >%v for user=%s room=%s — gRPC keepalive will close",
							streamInactivityTimeout, connectedUser, currentRoom)
						logged = true
					}
				} else {
					logged = false
				}
			}
		}
	}()
	defer close(deadlineDone)

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		lastActivityTime = time.Now()

		if !authDone && msg.JwtToken != "" {
			claims, err := ValidateToken(msg.JwtToken)
			if err != nil || claims.Type != "access" {
				_ = stream.Send(&gen.ChatV2Message{
					Payload: &gen.ChatV2Message_System{
						System: &gen.ChatV2System{Type: "AUTH_FAILED", Message: "invalid token"},
					},
				})
				return fmt.Errorf("auth failed")
			}
			connectedUserID = claims.UserID
			connectedUser = claims.Username
			currentRoom = msg.RoomId
			authDone = true
			s.hub.SetV2UserId(stream, connectedUserID)
			s.hub.SetV2Username(stream, connectedUser)
			s.hub.SetV2Room(stream, currentRoom)
			s.hub.ClearGracePeriod(connectedUser)
			logger.Debugf("[ChatV2] %s connected to room %s", connectedUser, currentRoom)

			_ = s.db.UpdateLastSeen(connectedUser)
			if msg.ClientVersion != "" {
				_ = s.db.UpdateClientVersion(connectedUser, msg.ClientVersion)
				s.hub.SetClientVersion(connectedUserID, msg.ClientVersion)
			}

			go func() {
				ticker := time.NewTicker(60 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if connectedUser != "" {
							_ = s.db.UpdateLastSeen(connectedUser)
						}
					case <-stream.Context().Done():
						return
					}
				}
			}()

			if connectedUserID != "" && s.db.IsSuperAdmin(connectedUserID) {
				_ = stream.Send(&gen.ChatV2Message{
					Payload: &gen.ChatV2Message_System{
						System: &gen.ChatV2System{Type: "SET_SUPER_ADMIN", Message: "true"},
					},
				})
			}

			continue
		}

		if !authDone {
			_ = stream.Send(&gen.ChatV2Message{
				Payload: &gen.ChatV2Message_System{
					System: &gen.ChatV2System{Type: "AUTH_REQUIRED", Message: "send jwt_token first"},
				},
			})
			continue
		}

		// Handle room switch: client sends ChatV2Message with just roomId (no payload)
		if msg.RoomId != "" && msg.RoomId != currentRoom {
			currentRoom = msg.RoomId
			s.hub.SetV2Room(stream, currentRoom)
		}

		switch p := msg.Payload.(type) {
		case *gen.ChatV2Message_Message:
			v2msg := p.Message
			if v2msg == nil {
				continue
			}
			if currentRoom != "" {
				v2msg.RoomId = currentRoom
			}
			if v2msg.RoomId == "" {
				continue
			}

			_ = s.db.UpdateLastSeen(connectedUser)
			if msg.ClientVersion != "" {
				_ = s.db.UpdateClientVersion(connectedUser, msg.ClientVersion)
				s.hub.SetClientVersion(connectedUserID, msg.ClientVersion)
			}

			row := &MessageRowV2{
				ID:          v2msg.Id,
				RoomID:      v2msg.RoomId,
				SenderID:    connectedUserID,
				ContentType: "text",
				IsRead:      false,
				CreatedAt:   time.Now().UTC(),
			}

			switch c := v2msg.Content.(type) {
			case *gen.MessageV2_Text:
				if !utf8.ValidString(c.Text) {
					continue
				}
				row.Text = c.Text
				row.ContentType = "text"

				if strings.HasPrefix(strings.TrimSpace(c.Text), "/") {
					parts := strings.Fields(c.Text)
					cmd := parts[0]
					var args []string
					if len(parts) > 1 {
						args = parts[1:]
					}

					botReq := &gen.BotCommandRequest{
						UserId:   connectedUserID,
						Username: connectedUser,
						ChatId:   currentRoom,
						Command:  cmd,
						Args:     args,
					}
					botResp, err := s.ProcessBotCommand(nil, botReq)
					if err != nil {
						logger.Errorf("[ChatV2 BotCommand] Error: %v", err)
					} else if botResp != nil {
						botText := botResp.ResponseText
						if botResp.IsError && botResp.ErrorMessage != "" {
							botText = "⚠️ " + botResp.ErrorMessage
						}
						_ = stream.Send(&gen.ChatV2Message{
							Payload: &gen.ChatV2Message_System{
								System: &gen.ChatV2System{Type: "BOT_RESPONSE", Message: botText},
							},
						})
					}
					continue
				}
			case *gen.MessageV2_Media:
				row.MediaURL = c.Media.Url
				if len(c.Media.Urls) > 0 {
					b, _ := json.Marshal(c.Media.Urls)
					row.MediaURLs = string(b)
				}
				row.Duration = c.Media.Duration
				row.ContentType = c.Media.Type
			}

			if v2msg.Reply != nil {
				row.ReplyToID = sql.NullString{String: v2msg.Reply.MessageId, Valid: true}
				preview := v2msg.Reply.Preview
				if !utf8.ValidString(preview) {
					preview = strings.ToValidUTF8(preview, "")
				}
				row.ReplyPreview = sql.NullString{String: preview, Valid: true}
				row.ReplySenderID = sql.NullString{String: v2msg.Reply.SenderId, Valid: true}
			}

			if v2msg.IsE2Ee {
				row.IsE2EE = true
				row.E2EEPayload = []byte(v2msg.E2EePayload)
			}

			if !utf8.ValidString(row.MediaURL) {
				row.MediaURL = strings.ToValidUTF8(row.MediaURL, "")
			}
			if row.ReplyPreview.Valid && !utf8.ValidString(row.ReplyPreview.String) {
				row.ReplyPreview.String = strings.ToValidUTF8(row.ReplyPreview.String, "")
			}

			if err := s.db.SaveMessageV2(row); err != nil {
				logger.Errorf("[ChatV2] save error: %v", err)
				continue
			}

			preview := row.Text
			if len(preview) > 500 {
				preview = preview[:500]
			}
			if row.ContentType == "image" {
				preview = "Image"
			} else if row.ContentType == "voice" {
				preview = "Voice message"
			}
			_, _ = s.db.Exec(`UPDATE chats SET last_message_text=$1, last_message_time=$2, last_message_username=$3, last_message_has_image=$4 WHERE id=$5`,
				preview, row.CreatedAt, connectedUser, row.ContentType == "image", row.RoomID)
			_ = s.db.IncrementParticipantsChatListVersion(row.RoomID)

			protoMsg := rowToProtoV2(row)
			wrappedMsg := &gen.ChatV2Message{
				Payload: &gen.ChatV2Message_Message{Message: protoMsg},
			}
			// Broadcast order: messages are sent to room streams in arrival order,
			// which matches created_at since each message is timestamped before save.
			hubSnapshot := s.hub.SnapshotRoomStreams(currentRoom)
			for _, target := range hubSnapshot {
				_ = target.Send(wrappedMsg)
			}

			if !strings.HasPrefix(currentRoom, "favorites_") {
				senderNotifiesOthers := s.db.GetUserPushStatus(connectedUser)
				if senderNotifiesOthers {
					chat, err := s.db.GetChat(currentRoom)
					if err == nil {
						var participants []string
						if err := json.Unmarshal([]byte(chat.Participants), &participants); err == nil {
							var recipients []string
							for _, pp := range participants {
								if pp != connectedUser {
									recipients = append(recipients, pp)
								}
							}
							if len(recipients) > 0 {
								dbTargets, err := s.db.GetPushTokensByUsernames(recipients)
								if err == nil && len(dbTargets) > 0 {
									var targets []pushTarget
									for _, t := range dbTargets {
										targets = append(targets, pushTarget{
											UserId:   t.UserId,
											Username: t.Username,
										})
									}
									pushText := row.Text
									if chat.IsSecret {
										pushText = "New encrypted message"
									}
									s.pushDebouncer.Enqueue(targets, connectedUser, connectedUserID, pushText, currentRoom, row.ID)
								}
							}
						}
					}
				}
			}

		case *gen.ChatV2Message_Typing:
			logger.Debugf("[ChatV2] TYPING from %s in room %s (isTyping=%v)", connectedUser, currentRoom, p.Typing.IsTyping)
			s.hub.BroadcastToRoom(currentRoom, "TYPING", fmt.Sprintf("%s|%v", connectedUser, p.Typing.IsTyping))

		case *gen.ChatV2Message_System:
			continue
		}
	}
}

func (s *server) MarkRead(ctx context.Context, req *gen.MarkReadRequest) (*gen.MarkReadResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.MarkReadResponse{Success: false}, nil
	}

	roomID := req.GetRoomId()
	if roomID == "" {
		return &gen.MarkReadResponse{Success: false}, nil
	}

	changed, err := s.db.MarkReadAndCheck(roomID, userID)
	if err != nil {
		logger.Errorf("MarkRead error: user=%s room=%s err=%v", userID, roomID, err)
		return &gen.MarkReadResponse{Success: false}, nil
	}

	if changed {
		s.hub.BroadcastToRoom(roomID, "READ_ALL", userID)
	}

	return &gen.MarkReadResponse{Success: true}, nil
}
