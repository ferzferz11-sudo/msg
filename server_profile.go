package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (s *server) UpdateUsername(ctx context.Context, req *gen.UpdateUsernameRequest) (*gen.UpdateUsernameResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	username := resolveDisplayName(s.db, userID)

	err := s.db.UpdateUsername(username, req.NewUsername)
	if err != nil {
		logger.Infof("Failed to update username from %s to %s: %v", username, req.NewUsername, err)
		return &gen.UpdateUsernameResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	logger.Infof("Username updated from %s to %s", username, req.NewUsername)
	return &gen.UpdateUsernameResponse{
		Success: true,
		Message: "Username updated successfully",
	}, nil
}

func (s *server) UpdatePassword(ctx context.Context, req *gen.UpdatePasswordRequest) (*gen.UpdatePasswordResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	username := resolveDisplayName(s.db, userID)

	storedHash, err := s.db.GetUserPasswordHash(username)
	if err != nil {
		logger.Infof("Failed to get password hash for %s: %v", username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "User not found",
		}, err
	}

	if !CheckPassword(req.OldPassword, storedHash) {
		logger.Infof("Old password verification failed for user: %s", username)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "Old password is incorrect",
		}, fmt.Errorf("old password verification failed")
	}

	err = s.db.UpdatePassword(username, req.NewPassword)
	if err != nil {
		logger.Infof("Failed to update password for %s: %v", username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	logger.Infof("Password updated for user: %s", username)
	return &gen.UpdatePasswordResponse{
		Success: true,
		Message: "Password updated successfully",
	}, nil
}

func (s *server) AdminUpdatePassword(ctx context.Context, req *gen.AdminUpdatePasswordRequest) (*gen.AdminUpdatePasswordResponse, error) {
	adminUserID := GetUserID(ctx)
	if adminUserID == "" {
		adminUserID = req.AdminUserId
	}
	adminUsername := resolveDisplayName(s.db, adminUserID)

	if !s.db.IsSuperAdmin(adminUsername) {
		logger.Infof("Unauthorized AdminUpdatePassword attempt by %s", adminUsername)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: "Unauthorized: only super admins can reset passwords",
		}, nil
	}

	err := s.db.UpdatePassword(req.TargetUsername, req.NewPassword)
	if err != nil {
		logger.Infof("Failed to admin-reset password for %s: %v", req.TargetUsername, err)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	logger.Infof("Admin %s reset password for user: %s", adminUsername, req.TargetUsername)
	return &gen.AdminUpdatePasswordResponse{
		Success: true,
		Message: "Password reset successfully",
	}, nil
}

func (s *server) MarkRead(ctx context.Context, req *gen.MarkReadRequest) (*gen.MarkReadResponse, error) {
	if strings.HasPrefix(req.RoomId, "favorites_") {
		return &gen.MarkReadResponse{Success: true}, nil
	}

	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.MarkReadResponse{Success: true}, nil
	}

	changed, err := s.db.MarkReadAndCheck(req.RoomId, userID)
	if err != nil {
		logger.Infof("Failed to mark read for %s in room %s: %v", userID, req.RoomId, err)
		return &gen.MarkReadResponse{Success: false}, err
	}

	if changed {
		logger.Infof("Marked read for %s in room %s", userID, req.RoomId)
		s.hub.Broadcast(&gen.Message{
			User:   "SYSTEM",
			Text:   "READ_ALL:" + userID,
			RoomId: req.RoomId,
		})
	}

	return &gen.MarkReadResponse{Success: true}, nil
}

func (s *server) UpdateAvatar(ctx context.Context, req *gen.UpdateAvatarRequest) (*gen.UpdateAvatarResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	username := resolveDisplayName(s.db, userID)

	oldThumb, oldFull, err := s.db.GetUserAvatarWithFull(username)
	if err == nil {
		newThumb := filepath.Base(req.AvatarUrl)
		newFull := filepath.Base(req.FullAvatarUrl)
		go func(t, f, nt, nf string) {
			if t != "" && filepath.Base(t) != nt {
				_ = DeleteImageFile(t)
			}
			if f != "" && f != t && filepath.Base(f) != nf {
				_ = DeleteImageFile(f)
			}
		}(oldThumb, oldFull, newThumb, newFull)
	}

	err = s.db.UpdateAvatarWithFull(username, req.AvatarUrl, req.FullAvatarUrl)
	if err != nil {
		logger.Infof("Failed to update avatar for %s: %v", username, err)
		return &gen.UpdateAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	logger.Infof("Updated avatar for %s (thumb: %s, full: %s)", username, req.AvatarUrl, req.FullAvatarUrl)
	return &gen.UpdateAvatarResponse{Success: true, Message: "Avatar updated successfully"}, nil
}

func (s *server) DeleteProfile(ctx context.Context, req *gen.DeleteProfileRequest) (*gen.DeleteProfileResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	username := resolveDisplayName(s.db, userID)

	if username == "" {
		return &gen.DeleteProfileResponse{Success: false, Message: "Username or User ID is required"}, nil
	}

	logger.Infof("DeleteProfile: Request to delete user %s", username)

	err := s.db.DeleteProfile(username)
	if err != nil {
		logger.Infof("Failed to delete profile for %s: %v", username, err)
		return &gen.DeleteProfileResponse{Success: false, Message: err.Error()}, nil
	}

	s.hub.BroadcastGlobal(&gen.Message{
		User: "SYSTEM",
		Text: "FORCE_DISCONNECT:" + username,
	})

	logger.Infof("Successfully deleted profile for user: %s", username)
	s.broadcastOnlineUsers()

	return &gen.DeleteProfileResponse{Success: true, Message: "Profile deleted successfully"}, nil
}
