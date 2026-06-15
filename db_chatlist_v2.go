package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ======= ChatList v2: Database methods =======

// ChatV2Row represents a chat row with v2 fields (pinned, muted, archived)
type ChatV2Row struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
	IsPinned                                                                                              bool
	IsMuted                                                                                               bool
	IsArchived                                                                                            bool
	PinnedAt                                                                                              int64
}

// PinnedMessageRow represents a pinned message with its metadata.
type PinnedMessageRow struct {
	MessageID string
	PinnedAt  int64
	User      string
	Text      string
	CreatedAt time.Time
}

// MigrateChatListV2 adds columns and tables needed for ChatList v2.
// Called from ConnectDB during initialization.
func MigrateChatListV2(db *sql.DB) {
	queries := []string{
		// user_chat_metadata: add pinned, muted, archived columns
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='pinned') THEN
				ALTER TABLE user_chat_metadata ADD COLUMN pinned BOOLEAN DEFAULT FALSE;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='pinned_at') THEN
				ALTER TABLE user_chat_metadata ADD COLUMN pinned_at BIGINT DEFAULT 0;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='archived') THEN
				ALTER TABLE user_chat_metadata ADD COLUMN archived BOOLEAN DEFAULT FALSE;
			END IF;
		END $$`,
		// Add user_id to user_chat_metadata
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='user_id') THEN
				ALTER TABLE user_chat_metadata ADD COLUMN user_id UUID;
				UPDATE user_chat_metadata ucm SET user_id = (SELECT id FROM users u WHERE u.username = ucm.username) WHERE user_id IS NULL;
			END IF;
			-- Drop old username column if exists
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='username') THEN
				ALTER TABLE user_chat_metadata ALTER COLUMN username DROP NOT NULL;
			END IF;
		END $$`,
		// Drop old primary key, add new one with user_id
		`DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name='user_chat_metadata' AND constraint_name='user_chat_metadata_pkey') THEN
				ALTER TABLE user_chat_metadata DROP CONSTRAINT user_chat_metadata_pkey;
			END IF;
			ALTER TABLE user_chat_metadata ADD PRIMARY KEY (user_id, room_id);
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
		// muted_chats: add user_id column
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='muted_chats' AND column_name='user_id') THEN
				ALTER TABLE muted_chats ADD COLUMN user_id UUID;
				UPDATE muted_chats mc SET user_id = (SELECT id FROM users u WHERE u.username = mc.username) WHERE user_id IS NULL;
			END IF;
		END $$`,
		// Index for pinned chats lookup
		`CREATE INDEX IF NOT EXISTS idx_user_chat_metadata_pinned ON user_chat_metadata(user_id, pinned) WHERE pinned = TRUE`,
		// Index for muted chats
		`CREATE INDEX IF NOT EXISTS idx_muted_chats_user ON muted_chats(user_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "must be owner of table") &&
				!strings.Contains(err.Error(), "already exists") &&
				!strings.Contains(err.Error(), "duplicate column") &&
				!strings.Contains(err.Error(), "duplicate key") {
				logger.Errorf("ChatList v2 migration error: %v", err)
			}
		}
	}
}

// PinChat marks a chat as pinned for the user
func (db *DB) PinChat(userID, chatID string) error {
	_, err := db.Exec(`
		INSERT INTO user_chat_metadata (user_id, room_id, pinned, pinned_at, last_read_at)
		VALUES ($1::uuid, $2, TRUE, $3, NOW())
		ON CONFLICT (user_id, room_id) DO UPDATE SET pinned = TRUE, pinned_at = $3`,
		userID, chatID, time.Now().Unix())
	return err
}

// UnPinChat removes pinned status for a chat
func (db *DB) UnPinChat(userID, chatID string) error {
	_, err := db.Exec(`
		UPDATE user_chat_metadata SET pinned = FALSE, pinned_at = 0
		WHERE user_id = $1::uuid AND room_id = $2`,
		userID, chatID)
	return err
}

// SearchChats searches user's chats by name or participant
func (db *DB) SearchChats(userID, query string, limit, offset int) ([]ChatV2Row, error) {
	if query == "" {
		return nil, nil
	}

	searchPattern := "%" + strings.ToLower(query) + "%"

	rows, err := db.Query(`
		SELECT DISTINCT c.id, c.name, c.type, c.participants, c.created_at,
		       COALESCE(c.creator_username, ''), COALESCE(c.creator_id::text, ''),
		       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
		       COALESCE(c.allow_members_to_add, FALSE), COALESCE(c.is_secret, FALSE),
		       COALESCE(c.last_message_text, ''), COALESCE(c.last_message_time, c.created_at),
		       COALESCE(ucm.pinned, FALSE), COALESCE(ucm.archived, FALSE), COALESCE(ucm.pinned_at, 0)
		FROM chats c
		LEFT JOIN user_chat_metadata ucm ON ucm.room_id = c.id AND ucm.user_id = $2::uuid
		WHERE (
			LOWER(c.name) LIKE $1
			OR LOWER(c.participants) LIKE $1
		)
		AND c.participants LIKE '%' || $2 || '%'
		ORDER BY COALESCE(c.last_message_time, c.created_at) DESC NULLS LAST
		LIMIT $3 OFFSET $4`,
		searchPattern, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatV2Row
	for rows.Next() {
		var c ChatV2Row
		var creatorId string
		var isSecret bool
		err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.Creator, &creatorId, &c.AvatarURL, &c.FullAvatarURL,
			&c.AllowMembersToAdd, &isSecret,
			&c.LastMessageText, &c.LastMessageTime,
			&c.IsPinned, &c.IsArchived, &c.PinnedAt,
		)
		if err != nil {
			logger.Errorf("SearchChats scan error: %v", err)
			continue
		}
		c.IsMuted = db.isChatMuted(userID, c.ID)
		result = append(result, c)
	}

	return result, nil
}

// ArchiveChat marks a chat as archived
func (db *DB) ArchiveChat(userID, chatID string) error {
	_, err := db.Exec(`
		INSERT INTO user_chat_metadata (user_id, room_id, archived, last_read_at)
		VALUES ($1::uuid, $2, TRUE, NOW())
		ON CONFLICT (user_id, room_id) DO UPDATE SET archived = TRUE`,
		userID, chatID)
	return err
}

// UnarchiveChat removes archived status
func (db *DB) UnarchiveChat(userID, chatID string) error {
	_, err := db.Exec(`
		UPDATE user_chat_metadata SET archived = FALSE
		WHERE user_id = $1::uuid AND room_id = $2`,
		userID, chatID)
	return err
}

// GetUserChatsV2 returns chats with v2 fields (pinned, muted, archived) and pagination
func (db *DB) GetUserChatsV2(userID, username string, limit, offset int, filter string) ([]ChatV2Row, error) {
	if limit <= 0 {
		limit = 100
	}

	// Build WHERE clause based on filter
	whereExtra := ""
	switch filter {
	case "pinned":
		whereExtra = "AND COALESCE(ucm.pinned, FALSE) = TRUE"
	case "archived":
		whereExtra = "AND COALESCE(ucm.archived, FALSE) = TRUE"
	case "muted":
		whereExtra = "AND EXISTS (SELECT 1 FROM muted_chats mc WHERE mc.user_id = $1::uuid AND mc.room_id = c.id AND mc.muted = TRUE)"
	}

	orderBy := "ORDER BY COALESCE(ucm.pinned_at, 0) DESC, COALESCE(c.last_message_time, c.created_at) DESC NULLS LAST"

	// If filter is "pinned", order by pinned_at first
	if filter == "pinned" {
		orderBy = "ORDER BY ucm.pinned_at DESC NULLS LAST"
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.name, c.type, c.participants, c.created_at,
		       COALESCE(c.creator_username, ''), COALESCE(c.creator_id::text, ''),
		       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
		       COALESCE(c.allow_members_to_add, FALSE), COALESCE(c.is_secret, FALSE),
		       COALESCE(c.last_message_text, ''), COALESCE(c.last_message_time, c.created_at),
		       COALESCE(ucm.pinned, FALSE), COALESCE(ucm.archived, FALSE),
		       COALESCE(ucm.pinned_at, 0)
		FROM chats c
		LEFT JOIN user_chat_metadata ucm ON ucm.room_id = c.id AND ucm.user_id = $1::uuid
		LEFT JOIN muted_chats mc ON mc.room_id = c.id AND mc.user_id = $1::uuid
		WHERE c.participants LIKE '%%' || $1 || '%%'
		%s
		%s
		LIMIT $2 OFFSET $3`, whereExtra, orderBy)

	rows, err := db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatV2Row
	for rows.Next() {
		var c ChatV2Row
		var creatorId string
		var isSecret bool

		err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.Creator, &creatorId, &c.AvatarURL, &c.FullAvatarURL,
			&c.AllowMembersToAdd, &isSecret,
			&c.LastMessageText, &c.LastMessageTime,
			&c.IsPinned, &c.IsArchived, &c.PinnedAt,
		)
		if err != nil {
			logger.Errorf("GetUserChatsV2 scan error: %v", err)
			continue
		}
		c.IsMuted = db.isChatMuted(userID, c.ID)
		result = append(result, c)
	}

	return result, nil
}

// isChatMuted checks if a chat is muted for a user
func (db *DB) isChatMuted(userID, chatID string) bool {
	var muted bool
	err := db.QueryRow(`
		SELECT COALESCE(muted, FALSE) FROM muted_chats
		WHERE user_id = $1::uuid AND room_id = $2`,
		userID, chatID).Scan(&muted)
	if err != nil {
		return false
	}
	return muted
}

// EnsureUserChatMetadata creates or updates user_chat_metadata entry
func (db *DB) EnsureUserChatMetadata(userID, roomID string) error {
	_, err := db.Exec(`
		INSERT INTO user_chat_metadata (user_id, room_id, last_read_at)
		VALUES ($1::uuid, $2, NOW())
		ON CONFLICT (user_id, room_id) DO NOTHING`,
		userID, roomID)
	return err
}

// ======= Pin Message: Database methods =======

// MigratePinnedMessages adds the pinned_messages table.
// Called from MigrateChatListV2 during initialization.
func MigratePinnedMessages(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pinned_messages (
			user_id UUID NOT NULL,
			room_id UUID NOT NULL,
			message_id VARCHAR(255) NOT NULL,
			pinned_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, room_id, message_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pinned_messages_room ON pinned_messages(user_id, room_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				logger.Errorf("PinnedMessages migration error: %v", err)
			}
		}
	}
}

// PinMessage pins a message in a chat for a user.
// Returns error if the message is already pinned.
func (db *DB) PinMessage(userID, chatID, messageID string) error {
	// Verify user is participant of the chat
	var isParticipant bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM chats
			WHERE id = $1 AND participants LIKE '%' || $2 || '%'
		)`, chatID, userID).Scan(&isParticipant)
	if err != nil || !isParticipant {
		return fmt.Errorf("user is not a participant of this chat")
	}

	// Verify message exists in the chat
	var msgExists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM messages
			WHERE id = $1 AND room_id = $2
		)`, messageID, chatID).Scan(&msgExists)
	if err != nil || !msgExists {
		return fmt.Errorf("message not found in this chat")
	}

	_, err = db.Exec(`
		INSERT INTO pinned_messages (user_id, room_id, message_id, pinned_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		ON CONFLICT (user_id, room_id, message_id) DO NOTHING`,
		userID, chatID, messageID, time.Now().Unix())
	return err
}

// UnPinMessage removes a pinned message.
func (db *DB) UnPinMessage(userID, chatID, messageID string) error {
	_, err := db.Exec(`
		DELETE FROM pinned_messages
		WHERE user_id = $1::uuid AND room_id = $2::uuid AND message_id = $3`,
		userID, chatID, messageID)
	return err
}

// GetPinnedMessages returns all pinned messages for a user in a chat.
func (db *DB) GetPinnedMessages(userID, chatID string) ([]PinnedMessageRow, error) {
	rows, err := db.Query(`
		SELECT pm.message_id, pm.pinned_at, m.user, m.text, m.created_at
		FROM pinned_messages pm
		JOIN messages m ON m.id = pm.message_id AND m.room_id = pm.room_id
		WHERE pm.user_id = $1::uuid AND pm.room_id = $2::uuid
		ORDER BY pm.pinned_at DESC`,
		userID, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PinnedMessageRow
	for rows.Next() {
		var r PinnedMessageRow
		err := rows.Scan(&r.MessageID, &r.PinnedAt, &r.User, &r.Text, &r.CreatedAt)
		if err != nil {
			logger.Errorf("GetPinnedMessages scan error: %v", err)
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

// IsMessagePinned checks if a message is pinned by a user in a chat.
func (db *DB) IsMessagePinned(userID, chatID, messageID string) bool {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pinned_messages
			WHERE user_id = $1::uuid AND room_id = $2::uuid AND message_id = $3
		)`, userID, chatID, messageID).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}
