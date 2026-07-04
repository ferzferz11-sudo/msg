package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"strings"
	"time"
)

// profileServerV2 implements ProfileService — profile management with JWT Bearer auth.
// All methods extract user_id from gRPC context (set by AuthInterceptor).
type profileServerV2 struct {
	gen.UnimplementedProfileServiceServer
	db *DB
}

func newProfileServerV2(db *DB) *profileServerV2 {
	return &profileServerV2{db: db}
}

// GetProfile returns full profile for the authenticated user.
func (p *profileServerV2) GetProfile(ctx context.Context, _ *gen.GetProfileRequest) (*gen.GetProfileResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetProfileResponse{}, nil
	}

	// Resolve username from user_id
	username, err := p.db.GetUsernameByID(userID)
	if err != nil {
		logger.Errorf("ProfileV2: GetUsernameByID(%s) error: %v", userID, err)
		return &gen.GetProfileResponse{}, nil
	}

	// Fetch extended profile
	var email, bio, status, avatarURL, fullAvatarURL string
	var createdAt, lastSeenAt time.Time
	row := p.db.QueryRow(
		"SELECT COALESCE(email, ''), COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, ''), COALESCE(full_avatar_url, ''), created_at, last_seen_at FROM users WHERE id=$1::uuid",
		userID,
	)
	if err := row.Scan(&email, &bio, &status, &avatarURL, &fullAvatarURL, &createdAt, &lastSeenAt); err != nil {
		logger.Errorf("ProfileV2: GetProfile(%s) error: %v", userID, err)
		return &gen.GetProfileResponse{}, nil
	}

	isSuperAdmin := p.db.IsSuperAdmin(username)

	// Locale from user_settings (fallback to "en")
	locale := "en"
	var localeNull sql.NullString
	if err := p.db.QueryRow("SELECT locale FROM user_settings WHERE user_id=$1::uuid", userID).Scan(&localeNull); err == nil && localeNull.Valid {
		locale = localeNull.String
	}

	// Company info (from primary_company_id, fallback to highest level)
	var companyID, companyName, positionTitle sql.NullString
	var positionLevel sql.NullInt32

	// Try primary_company_id first
	var primaryCompanyID sql.NullString
	_ = p.db.QueryRow(`SELECT primary_company_id FROM users WHERE id=$1::uuid`, userID).Scan(&primaryCompanyID)

	if primaryCompanyID.Valid {
		_ = p.db.QueryRow(`
			SELECT c.id, c.name, cp.title, cp.level
			FROM company_members cm
			JOIN companies c ON c.id = cm.company_id
			JOIN company_positions cp ON cp.id = cm.position_id
			WHERE cm.user_id=$1::uuid AND cm.company_id=$2::uuid`, userID, primaryCompanyID.String).
			Scan(&companyID, &companyName, &positionTitle, &positionLevel)
	}

	// Fallback: highest position across all companies
	if !companyID.Valid {
		_ = p.db.QueryRow(`
			SELECT c.id, c.name, cp.title, cp.level
			FROM company_members cm
			JOIN companies c ON c.id = cm.company_id
			JOIN company_positions cp ON cp.id = cm.position_id
			WHERE cm.user_id=$1::uuid
			ORDER BY cp.level DESC LIMIT 1`, userID).Scan(&companyID, &companyName, &positionTitle, &positionLevel)
	}

	resp := &gen.GetProfileResponse{
		UserId:        userID,
		Username:      username,
		Email:         email,
		AvatarUrl:     avatarURL,
		FullAvatarUrl: fullAvatarURL,
		Bio:           bio,
		Status:        status,
		Locale:        locale,
		IsSuperAdmin:  isSuperAdmin,
		CreatedAt:     createdAt.Format(time.RFC3339),
		LastSeenAt:    lastSeenAt.Format(time.RFC3339),
	}
	if companyID.Valid {
		resp.CompanyId = companyID.String
		resp.CompanyName = companyName.String
	}
	if positionTitle.Valid {
		resp.PositionTitle = positionTitle.String
	}
	if positionLevel.Valid {
		resp.PositionLevel = int32(positionLevel.Int32)
	}

	return resp, nil
}

// UpdateProfile updates bio, status, locale, and optionally username.
func (p *profileServerV2) UpdateProfile(ctx context.Context, req *gen.UpdateProfileV2Request) (*gen.UpdateProfileV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateProfileV2Response{Success: false, Message: "unauthenticated"}, nil
	}

	username, err := p.db.GetUsernameByID(userID)
	if err != nil {
		return &gen.UpdateProfileV2Response{Success: false, Message: "user not found"}, nil
	}

	// Update username if provided
	if req.Username != "" && req.Username != username {
		if err := p.db.UpdateUsername(username, req.Username); err != nil {
			logger.Errorf("ProfileV2: UpdateUsername(%s -> %s) error: %v", username, req.Username, err)
			return &gen.UpdateProfileV2Response{Success: false, Message: err.Error()}, nil
		}
		username = req.Username
		logger.Infof("ProfileV2: Username updated: %s -> %s", req.Username, username)
	}

	// Update bio + status
	bio := req.Bio
	status := req.Status
	if err := p.db.UpdateProfile(username, bio, status); err != nil {
		logger.Errorf("ProfileV2: UpdateProfile(%s) error: %v", username, err)
		return &gen.UpdateProfileV2Response{Success: false, Message: err.Error()}, nil
	}

	// Update locale in user_settings
	locale := req.Locale
	if locale != "" {
		_, _ = p.db.Exec(`INSERT INTO user_settings (user_id, locale) VALUES ($1::uuid, $2) ON CONFLICT (user_id) DO UPDATE SET locale=$2`, userID, locale)
	}

	logger.Infof("ProfileV2: Profile updated for %s (bio=%d chars, status=%d chars, locale=%s)", username, len(bio), len(status), locale)

	// Return updated profile
	profileResp, _ := p.GetProfile(ctx, &gen.GetProfileRequest{})
	return &gen.UpdateProfileV2Response{
		Success: true,
		Message: "Profile updated",
		Profile: profileResp,
	}, nil
}

// UpdateAvatar updates avatar URLs for the authenticated user.
func (p *profileServerV2) UpdateAvatar(ctx context.Context, req *gen.UpdateAvatarV2Request) (*gen.UpdateAvatarV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateAvatarV2Response{Success: false, Message: "unauthenticated"}, nil
	}

	username, err := p.db.GetUsernameByID(userID)
	if err != nil {
		return &gen.UpdateAvatarV2Response{Success: false, Message: "user not found"}, nil
	}

	// Get old avatar for cleanup
	oldThumb, oldFull, _ := p.db.GetUserAvatarWithFull(username)

	// Update in DB
	if err := p.db.UpdateAvatarWithFull(username, req.AvatarUrl, req.FullAvatarUrl); err != nil {
		logger.Errorf("ProfileV2: UpdateAvatar(%s) error: %v", username, err)
		return &gen.UpdateAvatarV2Response{Success: false, Message: err.Error()}, nil
	}

	// Cleanup old files (async)
	go func() {
		if oldThumb != "" && !strings.Contains(req.AvatarUrl, oldThumb) {
			_ = DeleteImageFile(oldThumb)
		}
		if oldFull != "" && oldFull != oldThumb && !strings.Contains(req.FullAvatarUrl, oldFull) {
			_ = DeleteImageFile(oldFull)
		}
	}()

	logger.Infof("ProfileV2: Avatar updated for %s", username)
	return &gen.UpdateAvatarV2Response{
		Success:       true,
		Message:       "Avatar updated",
		AvatarUrl:     req.AvatarUrl,
		FullAvatarUrl: req.FullAvatarUrl,
	}, nil
}

// DeleteProfile deletes the authenticated user's profile after password confirmation.
func (p *profileServerV2) DeleteProfile(ctx context.Context, req *gen.DeleteProfileV2Request) (*gen.DeleteProfileV2Response, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.DeleteProfileV2Response{Success: false, Message: "unauthenticated"}, nil
	}

	username, err := p.db.GetUsernameByID(userID)
	if err != nil {
		return &gen.DeleteProfileV2Response{Success: false, Message: "user not found"}, nil
	}

	// Verify password — required for account deletion
	if req.Password == "" {
		return &gen.DeleteProfileV2Response{Success: false, Message: "password required to delete profile"}, nil
	}
	storedHash, err := p.db.GetUserPasswordHash(username)
	if err != nil || !CheckPassword(req.Password, storedHash) {
		return &gen.DeleteProfileV2Response{Success: false, Message: "invalid password"}, nil
	}

	if err := p.db.DeleteProfile(username); err != nil {
		logger.Errorf("ProfileV2: DeleteProfile(%s) error: %v", username, err)
		return &gen.DeleteProfileV2Response{Success: false, Message: err.Error()}, nil
	}

	// Note: force disconnect on prod is handled by the legacy ChatService path.
	// For v2, the client should send a separate SignOut after delete.
	logger.Infof("ProfileV2: Profile deleted for %s", username)
	return &gen.DeleteProfileV2Response{Success: true, Message: "Profile deleted"}, nil
}

// GetUserSettings returns user settings (locale, theme, push, custom).
func (p *profileServerV2) GetUserSettings(ctx context.Context, _ *gen.GetUserSettingsRequest) (*gen.GetUserSettingsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetUserSettingsResponse{}, nil
	}

	var locale, themeID string
	var pushEnabled bool
	var localeNull, themeIDNull sql.NullString
	var pushNull sql.NullBool

	row := p.db.QueryRow("SELECT locale, theme_id, push_enabled FROM user_settings WHERE user_id=$1::uuid", userID)
	if err := row.Scan(&localeNull, &themeIDNull, &pushNull); err == nil {
		if localeNull.Valid {
			locale = localeNull.String
		}
		if themeIDNull.Valid {
			themeID = themeIDNull.String
		}
		if pushNull.Valid {
			pushEnabled = pushNull.Bool
		}
	}

	if locale == "" {
		locale = "en"
	}

	return &gen.GetUserSettingsResponse{
		Locale:      locale,
		ThemeId:     themeID,
		PushEnabled: pushEnabled,
		Custom:      map[string]string{},
	}, nil
}

// UpdateUserSettings updates user settings.
func (p *profileServerV2) UpdateUserSettings(ctx context.Context, req *gen.UpdateUserSettingsRequest) (*gen.UpdateUserSettingsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateUserSettingsResponse{Success: false, Message: "unauthenticated"}, nil
	}

	// Upsert user_settings
	_, err := p.db.Exec(`INSERT INTO user_settings (user_id, locale, theme_id, push_enabled) VALUES ($1::uuid, $2, $3, $4) ON CONFLICT (user_id) DO UPDATE SET locale=COALESCE($2, user_settings.locale), theme_id=COALESCE($3, user_settings.theme_id), push_enabled=COALESCE($4, user_settings.push_enabled)`,
		userID, req.Locale, req.ThemeId, req.PushEnabled)
	if err != nil {
		logger.Errorf("ProfileV2: UpdateUserSettings(%s) error: %v", userID, err)
		return &gen.UpdateUserSettingsResponse{Success: false, Message: err.Error()}, nil
	}

	logger.Infof("ProfileV2: Settings updated for %s (locale=%s, theme=%s, push=%v)", userID, req.Locale, req.ThemeId, req.PushEnabled)
	return &gen.UpdateUserSettingsResponse{Success: true, Message: "Settings updated"}, nil
}
