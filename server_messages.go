package main

import (
	"encoding/base64"
	"LavenderMessenger/gen"
	"context"
	"encoding/json"

	"google.golang.org/protobuf/types/known/timestamppb"
)

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
		logger.Errorf("Error fetching history: %v", err)
		return nil, err
	}

	// Check if this is a secret chat (for backward compat with old messages without is_e2ee flag)
	chat, chatErr := s.db.GetChat(roomID)
	isSecretChat := chatErr == nil && chat.IsSecret

	var messages []*gen.Message
	// Проходим в обратном порядке, чтобы сообщения были от старых к новым
	for i := len(rawMessages) - 1; i >= 0; i-- {
		m := rawMessages[i]

		// Check if encrypted data is empty
		if len(m.Encrypted) == 0 {
			logger.Warnf("Warning: message %s has empty encrypted data", m.MessageID)
			continue // Skip messages with no encrypted data
		}

		// For E2EE messages, skip server-side decryption — client handles it
		// Use per-message flag if set, otherwise fall back to chat-level check for old messages
		msgIsE2EE := m.IsE2EE || isSecretChat
		var decryptedText string
		if msgIsE2EE {
			// Server cannot decrypt E2EE messages, client handles decryption
			decryptedText = ""
		} else {
			// Расшифровываем текст из базы
			var err error
			decryptedText, err = decrypt(m.Encrypted)
			if err != nil {
				msgType := "text"
				if m.VoiceURL != "" {
					msgType = "voice"
				} else if m.ImageURL != "" {
					msgType = "image"
				}
				logger.Infof("Failed to decrypt %s message %s (User: %s, Room: %s): %v", msgType, m.MessageID, m.Username, m.RoomID, err)

				// Show user-friendly error in the chat
				decryptedText = "не удалось расшифровать"
			}
		}

		// Check if decrypted text is empty (skip ONLY if NO media and NOT E2EE)
		if decryptedText == "" && m.ImageURL == "" && m.VoiceURL == "" && !msgIsE2EE {
			logger.Warnf("Warning: message %s decrypted to empty string, skipping", m.MessageID)
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
			IsE2Ee:             msgIsE2EE,
			E2EePayload:        base64.StdEncoding.EncodeToString(m.Encrypted),
		})
	}

	return &gen.GetHistoryResponse{
		Messages: messages,
	}, nil
}

func (s *server) SetReaction(_ context.Context, req *gen.ReactionRequest) (*gen.ReactionResponse, error) {
	// Получаем оригинальное сообщение для логирования текста
	var msgText string = "..."
	var isSecretMsg bool
	m, err := s.db.GetMessageByUUID(req.MessageId)
	if err == nil {
		// Check if message is in a secret chat — don't try to decrypt E2EE messages
		if m.RoomID != "" {
			if chat, chatErr := s.db.GetChat(m.RoomID); chatErr == nil && chat.IsSecret {
				isSecretMsg = true
			}
		}
		if !isSecretMsg {
			decryptedText, err := decrypt(m.Encrypted)
			if err == nil {
				if len(decryptedText) > 15 {
					msgText = decryptedText[:15] + "..."
				} else {
					msgText = decryptedText
				}
			}
		} else {
			msgText = "[E2EE]"
		}
	}

	logger.Infof("[Reaction] %s on %s (%s) by %s", req.Reaction.Emoji, req.MessageId, msgText, req.Reaction.User)

	err = s.db.SetReaction(req.MessageId, req.Reaction.User, req.Reaction.Emoji)
	if err != nil {
		logger.Infof("Failed to set reaction: %v", err)
		return &gen.ReactionResponse{Success: false}, err
	}

	// Broadcast the updated message to all clients in the room
	// 1. Get the full message from DB
	if m.MessageID != "" { // m is already fetched above
		// 2. Decrypt text (skip for E2EE — client handles it)
		decryptIsE2EE := m.IsE2EE
		if !decryptIsE2EE && m.RoomID != "" {
			if chat, chatErr := s.db.GetChat(m.RoomID); chatErr == nil && chat.IsSecret {
				decryptIsE2EE = true
			}
		}
		var decryptedText string
		if decryptIsE2EE {
			decryptedText = string(m.Encrypted)
		} else {
			decryptedText, _ = decrypt(m.Encrypted)
		}

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
		logger.Infof("Broadcasting updated message %s with reactions to room %s", msg.Id, msg.RoomId)
		s.hub.Broadcast(msg)
	}

	return &gen.ReactionResponse{Success: true}, nil
}

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
			logger.Infof("Unauthorized delete attempt by %s for message in %s", req.RequesterUsername, msg.RoomId)
			continue
		}

		// Try deleting by ID first if available
		if msg.Id != "" {
			// Get full message with image URLs before deletion
			fullMsg, err := s.db.GetMessageByUUID(msg.Id)
			if err != nil {
				logger.Infof("Failed to get message %s: %v", msg.Id, err)
			} else {
				// Delete single image file if exists
				if fullMsg.ImageURL != "" {
					if err := DeleteImageFile(fullMsg.ImageURL); err != nil {
						logger.Infof("Failed to delete image file for message %s: %v", msg.Id, err)
						// Continue with message deletion even if image deletion fails
					}
				}

				// Delete all gallery images if exists
				if fullMsg.ImageURLs != "" && fullMsg.ImageURLs != "[]" {
					var imageURLs []string
					if err := json.Unmarshal([]byte(fullMsg.ImageURLs), &imageURLs); err == nil {
						for _, url := range imageURLs {
							if err := DeleteImageFile(url); err != nil {
								logger.Infof("Failed to delete gallery image file for message %s: %v", msg.Id, err)
								// Continue with message deletion even if image deletion fails
							}
						}
					}
				}
			}

			err = s.db.DeleteMessageByUUID(msg.Id)
			if err == nil {
				anyDeleted = true
				logger.Infof("Deleted message by ID: %s", msg.Id)

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
			logger.Infof("Failed to find message for deletion: %v", err)
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
						logger.Infof("Failed to delete image file for candidate message: %v", err)
						// Continue with message deletion even if image deletion fails
					}
				}

				// Delete all gallery images if candidate has them
				if candidate.ImageURLs != "" && candidate.ImageURLs != "[]" {
					var imageURLs []string
					if err := json.Unmarshal([]byte(candidate.ImageURLs), &imageURLs); err == nil {
						for _, url := range imageURLs {
							if err := DeleteImageFile(url); err != nil {
								logger.Infof("Failed to delete gallery image file for candidate message: %v", err)
								// Continue with message deletion even if image deletion fails
							}
						}
					}
				}

				err = s.db.DeleteMessageByID(candidate.ID)
				if err == nil {
					anyDeleted = true
					logger.Infof("Deleted message by content from %s", msg.User)

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

func (s *server) EditMessage(_ context.Context, req *gen.EditMessageRequest) (*gen.EditMessageResponse, error) {
	if req.MessageId == "" {
		return &gen.EditMessageResponse{Success: false, Message: "Message ID is required"}, nil
	}

	err := s.db.UpdateMessageText(req.MessageId, req.Text)
	if err != nil {
		logger.Infof("Failed to edit message %s: %v", req.MessageId, err)
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

	logger.Infof("Edited message %s", req.MessageId)
	return &gen.EditMessageResponse{Success: true, Message: "Message edited successfully"}, nil
}
