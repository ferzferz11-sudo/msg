package main

import (
	"path/filepath"
	"fmt"
	"LavenderMessenger/gen"
	"context"
	"log"
	"strings"
)

func (s *server) UpdateUsername(_ context.Context, req *gen.UpdateUsernameRequest) (*gen.UpdateUsernameResponse, error) {
	oldUsername := req.OldUsername
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			oldUsername = resolved
		}
	}

	err := s.db.UpdateUsername(oldUsername, req.NewUsername)
	if err != nil {
		log.Printf("Failed to update username from %s to %s: %v", oldUsername, req.NewUsername, err)
		return &gen.UpdateUsernameResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Username updated from %s to %s", oldUsername, req.NewUsername)
	return &gen.UpdateUsernameResponse{
		Success: true,
		Message: "Username updated successfully",
	}, nil
}

func (s *server) UpdatePassword(_ context.Context, req *gen.UpdatePasswordRequest) (*gen.UpdatePasswordResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	// Проверяем старый пароль
	storedHash, err := s.db.GetUserPasswordHash(username)
	if err != nil {
		log.Printf("Failed to get password hash for %s: %v", username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "User not found",
		}, err
	}

	if !CheckPassword(req.OldPassword, storedHash) {
		log.Printf("Old password verification failed for user: %s", username)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: "Old password is incorrect",
		}, fmt.Errorf("old password verification failed")
	}

	// Обновляем пароль
	err = s.db.UpdatePassword(username, req.NewPassword)
	if err != nil {
		log.Printf("Failed to update password for %s: %v", username, err)
		return &gen.UpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Password updated for user: %s", username)
	return &gen.UpdatePasswordResponse{
		Success: true,
		Message: "Password updated successfully",
	}, nil
}

func (s *server) AdminUpdatePassword(_ context.Context, req *gen.AdminUpdatePasswordRequest) (*gen.AdminUpdatePasswordResponse, error) {
	adminUsername := req.AdminUsername
	if req.AdminUserId != "" {
		resolved := s.resolveUsername(req.AdminUserId)
		if resolved != "" {
			adminUsername = resolved
		}
	}

	// Verify admin status
	if !s.db.IsSuperAdmin(adminUsername) {
		log.Printf("Unauthorized AdminUpdatePassword attempt by %s", adminUsername)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: "Unauthorized: only super admins can reset passwords",
		}, nil
	}

	// Update password
	err := s.db.UpdatePassword(req.TargetUsername, req.NewPassword)
	if err != nil {
		log.Printf("Failed to admin-reset password for %s: %v", req.TargetUsername, err)
		return &gen.AdminUpdatePasswordResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	log.Printf("Admin %s reset password for user: %s", adminUsername, req.TargetUsername)
	return &gen.AdminUpdatePasswordResponse{
		Success: true,
		Message: "Password reset successfully",
	}, nil
}

func (s *server) MarkRead(_ context.Context, req *gen.MarkReadRequest) (*gen.MarkReadResponse, error) {
	if strings.HasPrefix(req.RoomId, "favorites_") {
		return &gen.MarkReadResponse{Success: true}, nil
	}

	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	changed, err := s.db.MarkReadAndCheck(req.RoomId, username)
	if err != nil {
		log.Printf("Failed to mark read for %s in room %s: %v", username, req.RoomId, err)
		return &gen.MarkReadResponse{Success: false}, err
	}

	if changed {
		log.Printf("Marked read for %s in room %s", username, req.RoomId)
		// Broadcast read signal to the room
		s.hub.Broadcast(&gen.Message{
			User:   "SYSTEM",
			Text:   "READ_ALL:" + username,
			RoomId: req.RoomId,
		})
	}

	return &gen.MarkReadResponse{Success: true}, nil
}

func (s *server) UpdateAvatar(_ context.Context, req *gen.UpdateAvatarRequest) (*gen.UpdateAvatarResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	// 1. Get old avatar URLs for deletion
	oldThumb, oldFull, err := s.db.GetUserAvatarWithFull(username)
	if err == nil {
		// Extract filenames from new URLs to avoid deleting newly uploaded files
		// (when the same image is re-uploaded, the hash/filename stays the same)
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

	// 2. Save both thumbnail and full avatar URLs
	err = s.db.UpdateAvatarWithFull(username, req.AvatarUrl, req.FullAvatarUrl)
	if err != nil {
		log.Printf("Failed to update avatar for %s: %v", username, err)
		return &gen.UpdateAvatarResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated avatar for %s (thumb: %s, full: %s)", username, req.AvatarUrl, req.FullAvatarUrl)
	return &gen.UpdateAvatarResponse{Success: true, Message: "Avatar updated successfully"}, nil
}

func (s *server) DeleteProfile(ctx context.Context, req *gen.DeleteProfileRequest) (*gen.DeleteProfileResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	if username == "" {
		return &gen.DeleteProfileResponse{Success: false, Message: "Username or User ID is required"}, nil
	}

	log.Printf("DeleteProfile: Request to delete user %s", username)

	err := s.db.DeleteProfile(username)
	if err != nil {
		log.Printf("Failed to delete profile for %s: %v", username, err)
		return &gen.DeleteProfileResponse{Success: false, Message: err.Error()}, nil
	}

	// Force disconnect the user if they are currently online
	s.hub.BroadcastGlobal(&gen.Message{
		User: "SYSTEM",
		Text: "FORCE_DISCONNECT:" + username,
	})

	log.Printf("Successfully deleted profile for user: %s", username)
	s.broadcastOnlineUsers()

	return &gen.DeleteProfileResponse{Success: true, Message: "Profile deleted successfully"}, nil
}
