package main

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"os"
	"time"

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
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" || req.MessageId == "" {
		logger.Infof("[Favorites] AddFavorite rejected: empty user_id=%s or message_id=%s", userID, req.MessageId)
		return &gen.AddFavoriteResponse{Success: false, Message: "empty user id or message id"}, nil
	}
	logger.Infof("[Favorites] AddFavorite: user_id=%s message_id=%s", userID, req.MessageId)
	err := s.db.AddFavorite(userID, req.MessageId)
	if err != nil {
		logger.Infof("[Favorites] AddFavorite DB error: %v", err)
		return &gen.AddFavoriteResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("[Favorites] AddFavorite success: user_id=%s message_id=%s", userID, req.MessageId)
	return &gen.AddFavoriteResponse{Success: true}, nil
}

func (s *server) RemoveFavorite(ctx context.Context, req *gen.RemoveFavoriteRequest) (*gen.RemoveFavoriteResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" || req.MessageId == "" {
		return &gen.RemoveFavoriteResponse{Success: false}, nil
	}
	err := s.db.RemoveFavorite(userID, req.MessageId)
	if err != nil {
		logger.Infof("Failed to remove favorite: %v", err)
		return &gen.RemoveFavoriteResponse{Success: false}, nil
	}
	return &gen.RemoveFavoriteResponse{Success: true}, nil
}

func (s *server) GetFavorites(ctx context.Context, req *gen.GetFavoritesRequest) (*gen.GetFavoritesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.GetFavoritesResponse{Messages: nil}, nil
	}
	favs, err := s.db.GetFavorites(userID)
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
		logger.Infof("[Favorites] SaveFavoriteMessage rejected: empty username")
		return &gen.AddFavoriteResponse{Success: false, Message: "username required"}, nil
	}

	// Get User UUID
	userID, err := s.db.GetUserIdByUsername(req.User)
	if err != nil {
		logger.Infof("[Favorites] SaveFavoriteMessage user not found: %s", req.User)
		return &gen.AddFavoriteResponse{Success: false, Message: "user not found"}, nil
	}

	logger.Infof("[Favorites] SaveFavoriteMessage: user=%s user_id=%s message_id=%s", req.User, userID, req.Id)

	// Add to favorites table only — no copy in messages_v2
	// This preserves the original message ID so reactions work correctly
	err = s.db.AddFavorite(userID, req.Id)
	if err != nil {
		logger.Infof("[Favorites] SaveFavoriteMessage DB error: %v", err)
		return &gen.AddFavoriteResponse{Success: false, Message: "failed to link favorite"}, nil
	}

	logger.Infof("[Favorites] SaveFavoriteMessage success: user=%s message_id=%s", req.User, req.Id)
	return &gen.AddFavoriteResponse{Success: true}, nil
}

func (s *server) GetDevices(ctx context.Context, req *gen.GetDevicesRequest) (*gen.GetDevicesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	dbDevices, err := s.db.GetUserDevices(userID)
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
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	err := s.db.DeleteUserDevice(req.DeviceId, userID)
	if err != nil {
		return &gen.DeleteDeviceResponse{Success: false, Message: err.Error()}, nil
	}

	s.hub.BroadcastGlobalV2("FORCE_DISCONNECT_DEVICE", req.DeviceId)

	return &gen.DeleteDeviceResponse{Success: true, Message: "Device removed"}, nil
}

func (s *server) DeleteOtherDevices(ctx context.Context, req *gen.DeleteDeviceRequest) (*gen.DeleteDeviceResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	err := s.db.DeleteOtherDevices(userID, req.DeviceId)
	if err != nil {
		return &gen.DeleteDeviceResponse{Success: false, Message: err.Error()}, nil
	}

	s.hub.BroadcastGlobalV2("FORCE_LOGOUT_EXCEPT", req.DeviceId)

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
