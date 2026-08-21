package main

import (
	"LavenderMessenger/gen"
	"context"
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

func (s *server) AddSavedMessage(ctx context.Context, req *gen.AddSavedMessageRequest) (*gen.AddSavedMessageResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" || req.MessageId == "" {
		logger.Infof("[SavedMessages] AddSavedMessage rejected: empty user_id=%s or message_id=%s", userID, req.MessageId)
		return &gen.AddSavedMessageResponse{Success: false, Message: "empty user id or message id"}, nil
	}
	logger.Infof("[SavedMessages] AddSavedMessage: user_id=%s message_id=%s", userID, req.MessageId)
	err := s.db.AddSavedMessage(userID, req.MessageId)
	if err != nil {
		logger.Infof("[SavedMessages] AddSavedMessage DB error: %v", err)
		return &gen.AddSavedMessageResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("[SavedMessages] AddSavedMessage success: user_id=%s message_id=%s", userID, req.MessageId)
	return &gen.AddSavedMessageResponse{Success: true}, nil
}

func (s *server) RemoveSavedMessage(ctx context.Context, req *gen.RemoveSavedMessageRequest) (*gen.RemoveSavedMessageResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" || req.MessageId == "" {
		return &gen.RemoveSavedMessageResponse{Success: false}, nil
	}
	err := s.db.RemoveSavedMessage(userID, req.MessageId)
	if err != nil {
		logger.Infof("[SavedMessages] RemoveSavedMessage error: %v", err)
		return &gen.RemoveSavedMessageResponse{Success: false}, nil
	}
	return &gen.RemoveSavedMessageResponse{Success: true}, nil
}

func (s *server) GetSavedMessages(ctx context.Context, req *gen.GetSavedMessagesRequest) (*gen.GetSavedMessagesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.GetSavedMessagesResponse{Messages: nil}, nil
	}
	rows, err := s.db.GetSavedMessages(userID)
	if err != nil {
		logger.Infof("[SavedMessages] GetSavedMessages error: %v", err)
		return &gen.GetSavedMessagesResponse{Messages: nil}, nil
	}

	var messages []*gen.MessageV2
	for _, r := range rows {
		m := rowToProtoV2(&r)
		if m != nil {
			messages = append(messages, m)
		}
	}

	return &gen.GetSavedMessagesResponse{Messages: messages}, nil
}

func (s *server) SaveSavedMessage(ctx context.Context, req *gen.Message) (*gen.AddSavedMessageResponse, error) {
	if req.User == "" {
		logger.Infof("[SavedMessages] SaveSavedMessage rejected: empty username")
		return &gen.AddSavedMessageResponse{Success: false, Message: "username required"}, nil
	}

	// Get User UUID
	userID, err := s.db.GetUserIdByUsername(req.User)
	if err != nil {
		logger.Infof("[SavedMessages] SaveSavedMessage user not found: %s", req.User)
		return &gen.AddSavedMessageResponse{Success: false, Message: "user not found"}, nil
	}

	logger.Infof("[SavedMessages] SaveSavedMessage: user=%s user_id=%s message_id=%s", req.User, userID, req.Id)

	// Add to saved_messages table only — no copy in messages_v2
	// This preserves the original message ID so reactions work correctly
	err = s.db.AddSavedMessage(userID, req.Id)
	if err != nil {
		logger.Infof("[SavedMessages] SaveSavedMessage DB error: %v", err)
		return &gen.AddSavedMessageResponse{Success: false, Message: "failed to link saved message"}, nil
	}

	logger.Infof("[SavedMessages] SaveSavedMessage success: user=%s message_id=%s", req.User, req.Id)
	return &gen.AddSavedMessageResponse{Success: true}, nil
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
