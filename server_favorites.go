package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *server) GetUserId(_ context.Context, req *gen.GetUserIdRequest) (*gen.GetUserIdResponse, error) {
	userID, err := s.db.GetUserIdByUsername(req.Username)
	if err != nil {
		logger.Infof("Failed to get user ID for %s: %v", req.Username, err)
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
		logger.Infof("Failed to add favorite: %v", err)
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
		logger.Infof("Failed to remove favorite: %v", err)
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
		logger.Infof("Failed to get favorites: %v", err)
		return &gen.GetFavoritesResponse{Messages: nil}, nil
	}

	var messages []*gen.Message
	for _, m := range favs {
		var decryptedText string
		var e2eePayload string
		isE2EE := m.IsE2EE
		if isE2EE {
			e2eePayload = string(m.Encrypted)
		} else {
			var err error
			decryptedText, err = decrypt(m.Encrypted)
			if err != nil {
				decryptedText = "не удалось расшифровать"
			}
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
			IsE2Ee:             isE2EE,
			E2EePayload:        e2eePayload,
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

	// 3. Get User UUID
	userID, err := s.db.GetUserIdByUsername(req.User)
	if err != nil {
		return &gen.AddFavoriteResponse{Success: false, Message: "user not found"}, nil
	}

	// 4. Save message to messages_v2 in a special "favorites" room for consistency
	// Room ID is "favorites_" + username
	favRoomID := "favorites_" + req.User
	v2Row := &MessageRowV2{
		ID:          req.Id,
		RoomID:      favRoomID,
		SenderID:    req.UserId,
		ContentType: "text",
		Text:        req.Text,
		IsRead:      true,
		CreatedAt:   req.CreatedAt.AsTime(),
	}
	if req.ImageUrl != "" {
		v2Row.MediaURL = req.ImageUrl
		v2Row.ContentType = "image"
	}
	if len(req.ImageUrls) > 0 {
		b, _ := json.Marshal(req.ImageUrls)
		v2Row.MediaURLs = string(b)
		v2Row.ContentType = "image"
	}
	if req.VoiceUrl != "" {
		v2Row.MediaURL = req.VoiceUrl
		v2Row.Duration = req.Duration
		v2Row.ContentType = "voice"
	}
	if req.RepliedToMessageId != "" {
		v2Row.ReplyToID = sql.NullString{String: req.RepliedToMessageId, Valid: true}
		v2Row.ReplyPreview = sql.NullString{String: req.RepliedToText, Valid: true}
	}
	err = s.db.SaveMessageV2(v2Row)
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
		logger.Infof("Password reset requested for non-existent email: %s", req.Email)
		return &gen.RequestPasswordResetResponse{Success: true, Message: "If the email exists, a reset link has been sent"}, nil
	}

	// Generate reset token
	token, err := GenerateResetToken()
	if err != nil {
		logger.Infof("Failed to generate reset token: %v", err)
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to generate reset token"}, nil
	}

	// Token expires in 1 hour
	expiresAt := time.Now().Add(1 * time.Hour)

	// Save token to database
	err = s.db.CreatePasswordResetToken(token, userId, expiresAt)
	if err != nil {
		logger.Infof("Failed to save reset token: %v", err)
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to save reset token"}, nil
	}

	// Send email with token
	err = SendPasswordResetEmail(req.Email, token)
	if err != nil {
		logger.Infof("Failed to send reset email: %v", err)
		if err.Error() == "SMTP_NOT_CONFIGURED" {
			return &gen.RequestPasswordResetResponse{Success: false, Message: "SMTP_NOT_CONFIGURED"}, nil
		}
		return &gen.RequestPasswordResetResponse{Success: false, Message: "Failed to send reset email"}, nil
	}

	logger.Infof("Password reset initiated for email: %s", req.Email)
	return &gen.RequestPasswordResetResponse{Success: true, Message: "If the email exists, a reset link has been sent"}, nil
}

func (s *server) ResetPassword(_ context.Context, req *gen.ResetPasswordRequest) (*gen.ResetPasswordResponse, error) {
	if req.Token == "" || req.NewPassword == "" {
		return &gen.ResetPasswordResponse{Success: false, Message: "Token and new password are required"}, nil
	}

	// Validate token and get user ID
	userId, err := s.db.ValidatePasswordResetToken(req.Token)
	if err != nil {
		logger.Infof("Invalid or expired reset token: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "Invalid or expired reset token"}, nil
	}

	// Get username from user ID
	var username string
	err = s.db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, userId).Scan(&username)
	if err != nil {
		logger.Infof("Failed to get username from user ID: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "User not found"}, nil
	}

	// Update password
	err = s.db.UpdatePassword(username, req.NewPassword)
	if err != nil {
		logger.Infof("Failed to update password: %v", err)
		return &gen.ResetPasswordResponse{Success: false, Message: "Failed to update password"}, nil
	}

	// Delete used token
	_ = s.db.DeletePasswordResetToken(req.Token)

	logger.Infof("Password reset successful for user: %s", username)
	return &gen.ResetPasswordResponse{Success: true, Message: "Password reset successfully"}, nil
}
