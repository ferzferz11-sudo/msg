package main

import (
	"LavenderMessenger/gen"
	"database/sql"
	"fmt"
	"time"
)

func (db *DB) UserExists(user string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, user).Scan(&e)
	return e, err
}

func (db *DB) UserExistsByID(userID string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid)`, userID).Scan(&e)
	return e, err
}

func (db *DB) GetUserByID(userID string) (username string, err error) {
	err = db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, userID).Scan(&username)
	return
}

func (db *DB) GetUserIDByUsername(username string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id::text FROM users WHERE username=$1`, username).Scan(&id)
	return id, err
}

func (db *DB) GetUserIdByUsername(user string) (string, error) {
	var id string
	db.QueryRow(`SELECT id::text FROM users WHERE username=$1`, user).Scan(&id)
	return id, nil
}

func (db *DB) GetUsernameByID(uid string) (string, error) {
	var name string
	db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, uid).Scan(&name)
	return name, nil
}

func (db *DB) EmailExists(email string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND email != '')`, email).Scan(&e)
	return e, err
}

func (db *DB) GetUserPasswordHash(user string) (string, error) {
	var h string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username=$1`, user).Scan(&h)
	return h, err
}

func (db *DB) GetUserPassword(user string) (string, error) {
	return db.GetUserPasswordHash(user)
}

func (db *DB) SaveUser(user, hash string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES ($1, $2)`, user, hash)
	return err
}

func (db *DB) SaveUserWithEmail(user, hash, email string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash, email) VALUES ($1, $2, $3)`, user, hash, email)
	return err
}

func (db *DB) IsSuperAdmin(user string) bool {
	var a bool
	err := db.QueryRow(`SELECT is_super_admin FROM users WHERE id=$1`, user).Scan(&a)
	if err == nil && a {
		return true
	}
	db.QueryRow(`SELECT is_super_admin FROM users WHERE username=$1`, user).Scan(&a)
	return a
}

func (db *DB) GetAllUsers() ([]struct {
	UserId, Username, AvatarURL, LastClientVersion, Email string
	LastSeenAt                                            sql.NullTime
	IsSuperAdmin                                          bool
}, error) {
	rows, err := db.Query(`SELECT id, username, COALESCE(avatar_url, ''), COALESCE(last_client_version, ''), last_seen_at, COALESCE(email, ''), COALESCE(is_super_admin, FALSE) FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		UserId, Username, AvatarURL, LastClientVersion, Email string
		LastSeenAt                                            sql.NullTime
		IsSuperAdmin                                          bool
	}
	for rows.Next() {
		var u struct {
			UserId, Username, AvatarURL, LastClientVersion, Email string
			LastSeenAt                                            sql.NullTime
			IsSuperAdmin                                          bool
		}
		rows.Scan(&u.UserId, &u.Username, &u.AvatarURL, &u.LastClientVersion, &u.LastSeenAt, &u.Email, &u.IsSuperAdmin)
		res = append(res, u)
	}
	return res, nil
}

func (db *DB) UpdateUsername(old, new string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE users SET username=$1 WHERE username=$2`, new, old)
	if err != nil {
		return err
	}

	if _, err = tx.Exec(`UPDATE messages SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE messages SET replied_to_user=$1 WHERE replied_to_user=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE reactions SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chats SET creator_username=$1 WHERE creator_username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chats SET participants = REPLACE(participants, '"' || $2 || '"', '"' || $1 || '"') WHERE participants LIKE '%' || '"' || $2 || '"' || '%'`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE user_chat_metadata SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE user_tokens SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE user_themes SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE draft_messages SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE draft_messages SET replied_to_user=$1 WHERE replied_to_user=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE muted_chats SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contacts SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contacts SET contact_username=$1 WHERE contact_username=$2`, new, old); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) UpdatePassword(user, pass string) error {
	h, err := HashPassword(pass)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = db.Exec(`UPDATE users SET password_hash=$1 WHERE username=$2`, h, user)
	return err
}

func (db *DB) GetUserAvatar(user string) (string, error) {
	var a string
	db.QueryRow(`SELECT COALESCE(avatar_url, '') FROM users WHERE username=$1`, user).Scan(&a)
	return a, nil
}

func (db *DB) GetUserAvatarWithFull(user string) (string, string, error) {
	var a, f string
	db.QueryRow(`SELECT COALESCE(avatar_url, ''), COALESCE(full_avatar_url, '') FROM users WHERE username=$1`, user).Scan(&a, &f)
	return a, f, nil
}

func (db *DB) GetUserAvatarWithFullURL(user string) (string, string, error) {
	return db.GetUserAvatarWithFull(user)
}

func (db *DB) UpdateAvatarWithFull(user, a, f string) error {
	_, err := db.Exec(`UPDATE users SET avatar_url=$1, full_avatar_url=$2 WHERE username=$3`, a, f, user)
	return err
}

func (db *DB) GetUserProfile(user string) (struct {
	Username, Bio, Status, AvatarURL, FullAvatarURL string
	LastSeenAt                                      sql.NullTime
}, error) {
	var p struct {
		Username, Bio, Status, AvatarURL, FullAvatarURL string
		LastSeenAt                                      sql.NullTime
	}
	err := db.QueryRow(`SELECT username, COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, ''), last_seen_at, COALESCE(full_avatar_url, '') FROM users WHERE username=$1`, user).Scan(&p.Username, &p.Bio, &p.Status, &p.AvatarURL, &p.LastSeenAt, &p.FullAvatarURL)
	return p, err
}

func (db *DB) GetUserProfileById(userId string) (struct {
	Username, Bio, Status, AvatarURL, FullAvatarURL string
	LastSeenAt                                      sql.NullTime
}, error) {
	var p struct {
		Username, Bio, Status, AvatarURL, FullAvatarURL string
		LastSeenAt                                      sql.NullTime
	}
	err := db.QueryRow(`SELECT username, COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, ''), last_seen_at, COALESCE(full_avatar_url, '') FROM users WHERE id=$1::uuid`, userId).Scan(&p.Username, &p.Bio, &p.Status, &p.AvatarURL, &p.LastSeenAt, &p.FullAvatarURL)
	return p, err
}

func (db *DB) UpdateProfile(user, bio, status string) error {
	_, err := db.Exec(`UPDATE users SET bio=$1, status=$2 WHERE username=$3`, bio, status, user)
	return err
}

func (db *DB) DeleteProfile(user string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	userID := `(SELECT id FROM users WHERE username=$1)`

	// AI tables (blocking FK — must delete before users)
	tx.Exec(`DELETE FROM agent_reviews WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM ai_usage_stats WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM ai_chats_v2 WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM agents_v2 WHERE created_by = `+userID, user)

	// Chat metadata & settings
	tx.Exec(`DELETE FROM user_chat_metadata WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM muted_chats WHERE username=$1`, user)
	tx.Exec(`DELETE FROM contacts WHERE username=$1 OR contact_username=$1`, user)
	tx.Exec(`DELETE FROM draft_messages WHERE user_id = (SELECT id::text FROM users WHERE username=$1)`, user)

	// User data
	tx.Exec(`DELETE FROM user_tokens WHERE username=$1`, user)
	tx.Exec(`DELETE FROM user_devices WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM user_themes WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM favorites WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM pinned_messages WHERE username=$1`, user)
	tx.Exec(`DELETE FROM chat_list_v2 WHERE user_id = `+userID, user)
	tx.Exec(`DELETE FROM calls WHERE caller_id = `+userID+` OR receiver_id = `+userID, user)
	tx.Exec(`DELETE FROM secret_chat_keys WHERE user_id = `+userID, user)

	// Hermes/AI session data
	tx.Exec(`DELETE FROM hermes_messages WHERE user_id = $1`, user)
	tx.Exec(`DELETE FROM hermes_sessions WHERE user_id = $1`, user)
	tx.Exec(`DELETE FROM hermes_agent_runs WHERE user_id = $1`, user)
	tx.Exec(`DELETE FROM hermes_custom_agents WHERE created_by = $1 OR user_id = $1`, user)
	tx.Exec(`DELETE FROM ai_chat_sessions WHERE user_id = $1`, user)
	tx.Exec(`DELETE FROM agent_tokens WHERE created_by = $1`, user)

	// Delete the user
	_, err = tx.Exec(`DELETE FROM users WHERE username=$1`, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) GetUserChatListVersion(user string) (int64, error) {
	var v int64
	db.QueryRow(`SELECT chat_list_version FROM users WHERE username=$1`, user).Scan(&v)
	return v, nil
}

func (db *DB) IncrementUserChatListVersion(user string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE username=$1`, user)
	return err
}

func (db *DB) IncrementUserChatListVersionByUserID(userID string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id=$1::uuid`, userID)
	return err
}

func (db *DB) SaveUserTheme(user string, t *gen.CustomTheme) error {
	query := `INSERT INTO user_themes (
		username, theme_id, name, primary_color, on_primary_color,
		surface_color, on_surface_color, background_color,
		text_primary_color, text_secondary_color, is_dark,
		chat_background_image_url, chat_list_background_image_url,
		bottom_panel_color, on_bottom_panel_color, surface_container,
		outgoing_bubble_color, incoming_bubble_color
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
	) ON CONFLICT (username, theme_id) DO UPDATE SET
		name=EXCLUDED.name, primary_color=EXCLUDED.primary_color,
		on_primary_color=EXCLUDED.on_primary_color, surface_color=EXCLUDED.surface_color,
		on_surface_color=EXCLUDED.on_surface_color, background_color=EXCLUDED.background_color,
		text_primary_color=EXCLUDED.text_primary_color, text_secondary_color=EXCLUDED.text_secondary_color,
		is_dark=EXCLUDED.is_dark, chat_background_image_url=EXCLUDED.chat_background_image_url,
		chat_list_background_image_url=EXCLUDED.chat_list_background_image_url,
		bottom_panel_color=EXCLUDED.bottom_panel_color, on_bottom_panel_color=EXCLUDED.on_bottom_panel_color,
		surface_container=EXCLUDED.surface_container, outgoing_bubble_color=EXCLUDED.outgoing_bubble_color,
		incoming_bubble_color=EXCLUDED.incoming_bubble_color`

	_, err := db.Exec(query,
		user, t.Id, t.Name, t.PrimaryColor, t.OnPrimaryColor,
		t.SurfaceColor, t.OnSurfaceColor, t.BackgroundColor,
		t.TextPrimaryColor, t.TextSecondaryColor, t.IsDark,
		t.ChatBackgroundImageUrl, t.ChatListBackgroundImageUrl,
		t.BottomPanelColor, t.OnBottomPanelColor, t.SurfaceContainer,
		t.OutgoingBubbleColor, t.IncomingBubbleColor)
	return err
}

func (db *DB) SetCurrentTheme(user, id string) error {
	_, err := db.Exec(`UPDATE users SET current_theme_id = $1 WHERE username = $2`, id, user)
	return err
}

func (db *DB) DeleteUserTheme(user, id string) error {
	_, err := db.Exec(`DELETE FROM user_themes WHERE username = $1 AND theme_id = $2`, user, id)
	return err
}

func (db *DB) GetUserThemes(user string) (string, []struct {
	ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
	IsDark                                                                                                                                               bool
	ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
}, error) {
	var curr string
	db.QueryRow(`SELECT current_theme_id FROM users WHERE username=$1`, user).Scan(&curr)
	rows, err := db.Query(`SELECT theme_id, name, primary_color, on_primary_color, surface_color, on_surface_color, background_color, text_primary_color, text_secondary_color, is_dark, chat_background_image_url, chat_list_background_image_url, bottom_panel_color, on_bottom_panel_color, surface_container, outgoing_bubble_color, incoming_bubble_color FROM user_themes WHERE username = $1`, user)
	if err != nil {
		return curr, nil, err
	}
	defer rows.Close()
	var res []struct {
		ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
		IsDark                                                                                                                                               bool
		ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
	}
	for rows.Next() {
		var t struct {
			ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
			IsDark                                                                                                                                               bool
			ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
		}
		rows.Scan(&t.ThemeID, &t.Name, &t.PrimaryColor, &t.OnPrimaryColor, &t.SurfaceColor, &t.OnSurfaceColor, &t.BackgroundColor, &t.TextPrimaryColor, &t.TextSecondaryColor, &t.IsDark, &t.ChatBackgroundImageUrl, &t.ChatListBackgroundImageUrl, &t.BottomPanelColor, &t.OnBottomPanelColor, &t.SurfaceContainer, &t.OutgoingBubbleColor, &t.IncomingBubbleColor)
		res = append(res, t)
	}
	return curr, res, nil
}

func (db *DB) GetUserThemesByUserID(userID string) (string, []struct {
	ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
	IsDark                                                                                                                                               bool
	ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
}, error) {
	var curr string
	db.QueryRow(`SELECT current_theme_id FROM users WHERE id=$1::uuid`, userID).Scan(&curr)
	rows, err := db.Query(`SELECT theme_id, name, primary_color, on_primary_color, surface_color, on_surface_color, background_color, text_primary_color, text_secondary_color, is_dark, chat_background_image_url, chat_list_background_image_url, bottom_panel_color, on_bottom_panel_color, surface_container, outgoing_bubble_color, incoming_bubble_color FROM user_themes WHERE user_id = $1::uuid OR username = (SELECT username FROM users WHERE id=$1::uuid)`, userID)
	if err != nil {
		return curr, nil, err
	}
	defer rows.Close()
	var res []struct {
		ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
		IsDark                                                                                                                                               bool
		ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
	}
	for rows.Next() {
		var t struct {
			ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
			IsDark                                                                                                                                               bool
			ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
		}
		rows.Scan(&t.ThemeID, &t.Name, &t.PrimaryColor, &t.OnPrimaryColor, &t.SurfaceColor, &t.OnSurfaceColor, &t.BackgroundColor, &t.TextPrimaryColor, &t.TextSecondaryColor, &t.IsDark, &t.ChatBackgroundImageUrl, &t.ChatListBackgroundImageUrl, &t.BottomPanelColor, &t.OnBottomPanelColor, &t.SurfaceContainer, &t.OutgoingBubbleColor, &t.IncomingBubbleColor)
		res = append(res, t)
	}
	return curr, res, nil
}

func (db *DB) SaveUserThemeByUserID(userID string, t *gen.CustomTheme) error {
	query := `INSERT INTO user_themes (
		user_id, username, theme_id, name, primary_color, on_primary_color,
		surface_color, on_surface_color, background_color,
		text_primary_color, text_secondary_color, is_dark,
		chat_background_image_url, chat_list_background_image_url,
		bottom_panel_color, on_bottom_panel_color, surface_container,
		outgoing_bubble_color, incoming_bubble_color
	) VALUES (
		$1::uuid, (SELECT username FROM users WHERE id=$1::uuid), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
	) ON CONFLICT (username, theme_id) DO UPDATE SET
		user_id=EXCLUDED.user_id, name=EXCLUDED.name, primary_color=EXCLUDED.primary_color,
		on_primary_color=EXCLUDED.on_primary_color, surface_color=EXCLUDED.surface_color,
		on_surface_color=EXCLUDED.on_surface_color, background_color=EXCLUDED.background_color,
		text_primary_color=EXCLUDED.text_primary_color, text_secondary_color=EXCLUDED.text_secondary_color,
		is_dark=EXCLUDED.is_dark, chat_background_image_url=EXCLUDED.chat_background_image_url,
		chat_list_background_image_url=EXCLUDED.chat_list_background_image_url,
		bottom_panel_color=EXCLUDED.bottom_panel_color, on_bottom_panel_color=EXCLUDED.on_bottom_panel_color,
		surface_container=EXCLUDED.surface_container, outgoing_bubble_color=EXCLUDED.outgoing_bubble_color,
		incoming_bubble_color=EXCLUDED.incoming_bubble_color`

	_, err := db.Exec(query,
		userID, t.Id, t.Name, t.PrimaryColor, t.OnPrimaryColor,
		t.SurfaceColor, t.OnSurfaceColor, t.BackgroundColor,
		t.TextPrimaryColor, t.TextSecondaryColor, t.IsDark,
		t.ChatBackgroundImageUrl, t.ChatListBackgroundImageUrl,
		t.BottomPanelColor, t.OnBottomPanelColor, t.SurfaceContainer,
		t.OutgoingBubbleColor, t.IncomingBubbleColor)
	return err
}

func (db *DB) SetCurrentThemeByUserID(userID, id string) error {
	_, err := db.Exec(`UPDATE users SET current_theme_id = $1 WHERE id = $2::uuid`, id, userID)
	return err
}

func (db *DB) DeleteUserThemeByUserID(userID, themeID string) error {
	_, err := db.Exec(`DELETE FROM user_themes WHERE (user_id = $1::uuid OR username = (SELECT username FROM users WHERE id=$1::uuid)) AND theme_id = $2`, userID, themeID)
	return err
}

func (db *DB) UpdateClientVersion(user, v string) error {
	_, err := db.Exec(`UPDATE users SET last_client_version=$1, last_seen_at=NOW() WHERE username=$2`, v, user)
	return err
}

func (db *DB) UpdateLastSeen(user string) error {
	_, err := db.Exec(`UPDATE users SET last_seen_at=NOW() WHERE username=$1`, user)
	return err
}

func (db *DB) queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error) {
	row := db.QueryRow(
		"SELECT COALESCE(email, ''), COALESCE(bio, ''), COALESCE(status, ''), created_at, last_seen_at FROM users WHERE username=$1",
		username,
	)
	err = row.Scan(&email, &bio, &status, &createdAt, &lastSeenAt)
	return
}

func (db *DB) GetUserPushStatus(user string) bool {
	var e bool
	db.QueryRow(`SELECT push_enabled FROM user_tokens WHERE username=$1`, user).Scan(&e)
	return e
}

func (db *DB) GetUserToken(user string) (string, error) {
	var t string
	db.QueryRow(`SELECT fcm_token FROM user_tokens WHERE username=$1`, user).Scan(&t)
	return t, nil
}

func (db *DB) GetUserTokenByUserID(uid string) (string, error) {
	var t string
	err := db.QueryRow(`SELECT fcm_token FROM user_tokens ut JOIN users u ON ut.username = u.username WHERE u.id = $1::uuid`, uid).Scan(&t)
	return t, err
}

func (db *DB) DeleteUserTokenByUserID(uid string) error {
	_, err := db.Exec(`DELETE FROM user_tokens WHERE username = (SELECT username FROM users WHERE id = $1::uuid)`, uid)
	return err
}

func (db *DB) SaveUserToken(user, token string, e bool) error {
	_, err := db.Exec(`INSERT INTO user_tokens (username, fcm_token, push_enabled, updated_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT (username) DO UPDATE SET fcm_token=EXCLUDED.fcm_token, updated_at=NOW()`, user, token, e)
	return err
}

func (db *DB) SaveUserTokenByUserID(userID, token string, e bool) error {
	_, err := db.Exec(`INSERT INTO user_tokens (user_id, username, fcm_token, push_enabled, updated_at)
		VALUES ($1::uuid, (SELECT username FROM users WHERE id=$1::uuid), $2, $3, NOW())
		ON CONFLICT (username) DO UPDATE SET user_id=EXCLUDED.user_id, fcm_token=EXCLUDED.fcm_token, updated_at=NOW()`, userID, token, e)
	return err
}

func (db *DB) GetUserPushStatusByUserID(userID string) bool {
	var e bool
	db.QueryRow(`SELECT push_enabled FROM user_tokens WHERE user_id=$1::uuid OR username=(SELECT username FROM users WHERE id=$1::uuid)`, userID).Scan(&e)
	return e
}

func (db *DB) SetUserPushStatusByUserID(userID string, enabled bool) error {
	_, err := db.Exec(`UPDATE user_tokens SET push_enabled=$1 WHERE user_id=$2::uuid OR username=(SELECT username FROM users WHERE id=$2::uuid)`, enabled, userID)
	return err
}

func (db *DB) GetUserIdByEmail(email string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id::text FROM users WHERE email=$1`, &id).Scan(&id)
	return id, err
}

type PushTarget struct {
	UserId   string
	Username string
	Token    string
}

func (db *DB) GetPushTokensByUsernames(usernames []string) ([]PushTarget, error) {
	if len(usernames) == 0 {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT u.id::text, u.username, COALESCE(ut.fcm_token, '')
		FROM users u
		JOIN user_tokens ut ON ut.username = u.username
		WHERE u.username = ANY($1) AND ut.push_enabled = true AND ut.fcm_token != ''`,
		usernames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []PushTarget
	for rows.Next() {
		var t PushTarget
		if err := rows.Scan(&t.UserId, &t.Username, &t.Token); err != nil {
			continue
		}
		if t.Token != "" {
			targets = append(targets, t)
		}
	}
	return targets, rows.Err()
}

func (db *DB) CreatePasswordResetToken(token, userId string, expiresAt time.Time) error {
	_, err := db.Exec(`INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES ($1, $2::uuid, $3)`, token, userId, expiresAt)
	return err
}

func (db *DB) ValidatePasswordResetToken(token string) (string, error) {
	var userId string
	var expiresAt time.Time
	err := db.QueryRow(`SELECT user_id::text, expires_at FROM password_reset_tokens WHERE token=$1`, token).Scan(&userId, &expiresAt)
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return userId, nil
}

func (db *DB) DeletePasswordResetToken(token string) error {
	_, err := db.Exec(`DELETE FROM password_reset_tokens WHERE token=$1`, token)
	return err
}

func (db *DB) AddUserDevice(userID, deviceID, deviceName, clientVersion, ipAddress string) error {
	_, err := db.Exec(`
		INSERT INTO user_devices (user_id, device_id, device_name, client_version, ip_address, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			device_name = EXCLUDED.device_name,
			client_version = EXCLUDED.client_version,
			ip_address = EXCLUDED.ip_address,
			is_active = TRUE,
			last_seen_at = NOW()
	`, userID, deviceID, deviceName, clientVersion, ipAddress)
	return err
}

func (db *DB) GetUserDevices(userId string) ([]struct {
	DeviceID, DeviceName, ClientVersion, IPAddress string
	LastSeenAt                                     time.Time
}, error) {
	rows, err := db.Query(`SELECT device_id, COALESCE(device_name, ''), COALESCE(client_version, ''), COALESCE(ip_address, ''), last_seen_at FROM user_devices WHERE user_id = $1::uuid ORDER BY last_seen_at DESC`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []struct {
		DeviceID, DeviceName, ClientVersion, IPAddress string
		LastSeenAt                                     time.Time
	}
	for rows.Next() {
		var d struct {
			DeviceID, DeviceName, ClientVersion, IPAddress string
			LastSeenAt                                     time.Time
		}
		rows.Scan(&d.DeviceID, &d.DeviceName, &d.ClientVersion, &d.IPAddress, &d.LastSeenAt)
		res = append(res, d)
	}
	return res, nil
}

func (db *DB) DeleteUserDevice(deviceID, userId string) error {
	_, err := db.Exec(`DELETE FROM user_devices WHERE device_id = $1 AND user_id = $2::uuid`, deviceID, userId)
	return err
}

func (db *DB) DeleteOtherDevices(userId, exceptDeviceId string) error {
	_, err := db.Exec(`DELETE FROM user_devices WHERE user_id = $1::uuid AND device_id != $2`, userId, exceptDeviceId)
	return err
}

func (db *DB) GetMutedChatsByUserID(uid string) ([]string, error) {
	rows, _ := db.Query(`SELECT room_id FROM muted_chats WHERE username=$1`, uid)
	var res []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			rows.Scan(&id)
			res = append(res, id)
		}
	}
	return res, nil
}

func (db *DB) GetMutedChats(uid string) ([]string, error) {
	return db.GetMutedChatsByUserID(uid)
}

func (db *DB) SetMutedChatByUserID(uid, room string, m bool) error {
	if m {
		_, err := db.Exec(`INSERT INTO muted_chats (username, room_id, muted) VALUES ($1, $2, TRUE) ON CONFLICT (username, room_id) DO UPDATE SET muted=TRUE`, uid, room)
		return err
	}
	_, err := db.Exec(`DELETE FROM muted_chats WHERE username=$1 AND room_id=$2`, uid, room)
	return err
}

func (db *DB) SetMutedChat(uid, room string, m bool) error {
	return db.SetMutedChatByUserID(uid, room, m)
}

type AdminUserRow struct {
	UserId, Username, AvatarURL, FullAvatarURL, Email, LastClientVersion string
	LastSeenAt                                                           sql.NullTime
	IsSuperAdmin                                                         bool
	LastMessageText, LastMessageUsername                                 string
	LastMessageTime                                                      sql.NullTime
	ChatCount                                                            int32
}

func (db *DB) GetAdminUserList(query string, limit int, sortBy string, lastMessageTime *time.Time, lastUsername string) ([]AdminUserRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	baseQuery := `
		WITH user_last_messages AS (
			SELECT
				m.sender_id,
				m.text,
				m.created_at,
				ROW_NUMBER() OVER (PARTITION BY m.sender_id ORDER BY m.created_at DESC) as rn
			FROM messages_v2 m
			WHERE m.text != '[deleted]' AND m.text != ''
		),
		user_chat_counts AS (
			SELECT
				uc.user_id,
				COUNT(DISTINCT uc.room_id) as chat_count
			FROM user_chat_metadata uc
			GROUP BY uc.user_id
		),
		user_latest_device AS (
			SELECT DISTINCT ON (user_id) user_id, client_version
			FROM user_devices
			WHERE client_version IS NOT NULL AND client_version != ''
			ORDER BY user_id, last_seen_at DESC
		)
		SELECT
			u.id,
			u.username,
			COALESCE(u.avatar_url, ''),
			COALESCE(u.full_avatar_url, ''),
			COALESCE(u.email, ''),
			COALESCE(u.is_super_admin, FALSE),
			COALESCE(d.client_version, '') as last_client_version,
			u.last_seen_at,
			COALESCE(LEFT(lm.text, 100), '') as last_message_text,
			lm.created_at as last_message_time,
			COALESCE(lm_sender.username, '') as last_message_username,
			COALESCE(cc.chat_count, 0) as chat_count
		FROM users u
		LEFT JOIN user_latest_device d ON d.user_id = u.id
		LEFT JOIN user_last_messages lm ON lm.sender_id = u.id AND lm.rn = 1
		LEFT JOIN users lm_sender ON lm_sender.id = lm.sender_id
		LEFT JOIN user_chat_counts cc ON cc.user_id = u.id
		WHERE ($1::text = '' OR u.username ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%')`

	var cursorClause string
	args := []interface{}{query}

	if lastMessageTime != nil {
		cursorClause = ` AND (
			lm.created_at < $2::timestamp
			OR (lm.created_at = $2::timestamp AND u.username > $3::text)
			OR lm.created_at IS NULL
		)`
		args = append(args, *lastMessageTime, lastUsername)
	}

	var orderBy string
	switch sortBy {
	case "last_seen":
		orderBy = ` ORDER BY u.last_seen_at DESC NULLS LAST, u.username ASC`
	case "username":
		orderBy = ` ORDER BY u.username ASC`
	default: // "last_message"
		orderBy = ` ORDER BY lm.created_at DESC NULLS LAST, u.username ASC`
	}

	limitArg := len(args) + 1
	args = append(args, limit+1)

	fullQuery := baseQuery + cursorClause + orderBy + fmt.Sprintf(` LIMIT $%d`, limitArg)

	rows, err := db.Query(fullQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("GetAdminUserList query error: %w", err)
	}
	defer rows.Close()

	var res []AdminUserRow
	for rows.Next() {
		var u AdminUserRow
		err := rows.Scan(
			&u.UserId, &u.Username, &u.AvatarURL, &u.FullAvatarURL,
			&u.Email, &u.IsSuperAdmin, &u.LastClientVersion,
			&u.LastSeenAt, &u.LastMessageText, &u.LastMessageTime,
			&u.LastMessageUsername, &u.ChatCount,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAdminUserList scan error: %w", err)
		}
		res = append(res, u)
	}
	return res, nil
}
