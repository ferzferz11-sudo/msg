package main

import (
	"database/sql"
	"LavenderMessenger/gen"
	"context"
	"log"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *server) GetAllUsers(ctx context.Context, req *gen.GetAllUsersRequest) (*gen.GetAllUsersResponse, error) {
	_ = ctx // ctx is required by gRPC interface but not used here
	_ = req // req is required by gRPC interface but not used here
	users, err := s.db.GetAllUsers()
	if err != nil {
		log.Printf("Error fetching all users: %v", err)
		return nil, err
	}

	var userInfos []*gen.UserInfo
	for _, u := range users {
		var lastSeen *timestamppb.Timestamp
		if u.LastSeenAt.Valid {
			lastSeen = timestamppb.New(u.LastSeenAt.Time)
		}

		userInfos = append(userInfos, &gen.UserInfo{
			Username:          u.Username,
			AvatarUrl:         u.AvatarURL,
			LastClientVersion: u.LastClientVersion,
			LastSeenAt:        lastSeen,
			Email:             u.Email,
		})
	}

	// Add server time to response for admin panel
	serverTime := timestamppb.Now()

	return &gen.GetAllUsersResponse{
		Users:      userInfos,
		ServerTime: serverTime,
	}, nil
}

func (s *server) UpdateProfile(_ context.Context, req *gen.UpdateProfileRequest) (*gen.UpdateProfileResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolvedUsername := s.resolveUsername(req.UserId)
		if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	err := s.db.UpdateProfile(username, req.Bio, req.Status)
	if err != nil {
		log.Printf("Failed to update profile for %s: %v", username, err)
		return &gen.UpdateProfileResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("Updated profile for %s", username)
	return &gen.UpdateProfileResponse{Success: true, Message: "Profile updated successfully"}, nil
}

func (s *server) GetUserProfile(_ context.Context, req *gen.GetUserProfileRequest) (*gen.GetUserProfileResponse, error) {
	var profile struct {
		Username, Bio, Status, AvatarURL, FullAvatarURL string
		LastSeenAt                                      sql.NullTime
	}
	var err error

	// Если передан user_id, используем его, иначе username (для обратной совместимости)
	if req.UserId != "" {
		profile, err = s.db.GetUserProfileById(req.UserId)
		if err != nil {
			log.Printf("Failed to get profile for user_id %s: %v", req.UserId, err)
			return &gen.GetUserProfileResponse{}, nil
		}
	} else if req.Username != "" {
		profile, err = s.db.GetUserProfile(req.Username)
		if err != nil {
			log.Printf("Failed to get profile for username %s: %v", req.Username, err)
			return &gen.GetUserProfileResponse{}, nil
		}
	} else {
		log.Printf("Failed to get profile: neither user_id nor username provided")
		return &gen.GetUserProfileResponse{}, nil
	}

	var lastSeen *timestamppb.Timestamp
	if profile.LastSeenAt.Valid {
		lastSeen = timestamppb.New(profile.LastSeenAt.Time)
	}

	return &gen.GetUserProfileResponse{
		Username:      profile.Username,
		Bio:           profile.Bio,
		Status:        profile.Status,
		AvatarUrl:     profile.AvatarURL,
		LastSeenAt:    lastSeen,
		FullAvatarUrl: profile.FullAvatarURL,
	}, nil
}

func (s *server) GetUserAvatar(_ context.Context, req *gen.GetUserAvatarRequest) (*gen.GetUserAvatarResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	avatarURL, fullAvatarURL, err := s.db.GetUserAvatarWithFull(username)
	if err != nil {
		log.Printf("Failed to get avatar for %s: %v", username, err)
		return &gen.GetUserAvatarResponse{AvatarUrl: "", FullAvatarUrl: ""}, nil
	}

	return &gen.GetUserAvatarResponse{AvatarUrl: avatarURL, FullAvatarUrl: fullAvatarURL}, nil
}
