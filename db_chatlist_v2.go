package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ======= ChatList v2: Database methods =======

type chatCursor struct {
	PinnedAt        int64     `json:"p"`
	LastMessageTime time.Time `json:"t"`
}

func encodeCursor(c chatCursor) string {
	data, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (chatCursor, bool) {
	if cursor == "" {
		return chatCursor{}, false
	}
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return chatCursor{}, false
	}
	var c chatCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return chatCursor{}, false
	}
	return c, true
}

// escapeLike escapes SQL LIKE special characters
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

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

	IsSecret                bool
	PeerPublicKey           string
	E2eeReady               bool
	ActiveAgentId           string
	AgentMode               string
	CompanyId               string
	CompanyChatAccess       string
	CompanyMinPositionLevel int32
	SelfDestructTimer       int32
}

// ChatV2Result extends ChatV2Row with pagination metadata
type ChatV2Result struct {
	Chats      []ChatV2Row
	NextCursor string
	HasMore    bool
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
		// Add agent_id and agent_mode to chats table (for AI chat agent info)
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='agent_id') THEN
				ALTER TABLE chats ADD COLUMN agent_id VARCHAR(255) DEFAULT '';
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='agent_mode') THEN
				ALTER TABLE chats ADD COLUMN agent_mode VARCHAR(50) DEFAULT 'single';
			END IF;
		END $$`,
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
			END IF;
		END $$`,
		// Fill user_id from users table (handle both username and UUID-as-username cases)
		`UPDATE user_chat_metadata ucm SET user_id = u.id FROM users u WHERE ucm.user_id IS NULL AND ucm.username = u.username`,
		// For records where username is actually a UUID, try matching by id
		`UPDATE user_chat_metadata SET user_id = username::uuid WHERE user_id IS NULL AND username ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'`,
		// Drop old primary key if exists (only if user_id has no nulls)
		`DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name='user_chat_metadata' AND constraint_name='user_chat_metadata_pkey') THEN
				IF NOT EXISTS (SELECT 1 FROM user_chat_metadata WHERE user_id IS NULL) THEN
					ALTER TABLE user_chat_metadata DROP CONSTRAINT user_chat_metadata_pkey;
				END IF;
			END IF;
		END $$`,
		// Add new primary key with user_id (only if no nulls in user_id)
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name='user_chat_metadata' AND constraint_name='user_chat_metadata_pkey') THEN
				IF NOT EXISTS (SELECT 1 FROM user_chat_metadata WHERE user_id IS NULL) THEN
					ALTER TABLE user_chat_metadata ADD PRIMARY KEY (user_id, room_id);
				END IF;
			END IF;
		END $$`,
		// Deprecated: Make username nullable (keep for backward compat with old queries/data).
		// The username column is no longer part of PK. Will be removed when all data migrates to user_id.
		`DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='username') THEN
				IF NOT EXISTS (SELECT 1 FROM information_schema.constraint_column_usage WHERE table_name='user_chat_metadata' AND column_name='username' AND constraint_name LIKE '%pkey%') THEN
					ALTER TABLE user_chat_metadata ALTER COLUMN username DROP NOT NULL;
				END IF;
			END IF;
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
		// Composite index for unread count CTE (messages by room + sender + time)
		`CREATE INDEX IF NOT EXISTS idx_messages_v2_room_sender_time ON messages_v2(room_id, sender_id, created_at)`,
		// GIN index for chats participant_ids array containment check
		`CREATE INDEX IF NOT EXISTS idx_chats_participant_ids ON chats USING GIN(participant_ids)`,
		// Index for chat list ordering by last_message_time
		`CREATE INDEX IF NOT EXISTS idx_chats_last_message_time ON chats(last_message_time DESC NULLS LAST)`,
		// Backfill participant_ids for company chats that are missing it
		`UPDATE chats c SET participant_ids = (
			SELECT array_agg(u.id ORDER BY u.username)
			FROM users u
			WHERE u.username = ANY(SELECT json_array_elements_text(c.participants::json))
		)
		WHERE c.type = 'company' AND (c.participant_ids IS NULL OR c.participant_ids = '{}')`,
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

	searchPattern := "%" + strings.ToLower(escapeLike(query)) + "%"

	rows, err := db.Query(`
		WITH user_last_read AS (
			SELECT room_id, COALESCE(last_read_at, '1970-01-01') as last_read FROM user_chat_metadata WHERE user_id = $2::uuid
		),
		user_info AS (
			SELECT username FROM users WHERE id = $2::uuid
		),
		unread_counts AS (
			SELECT mv.room_id, COUNT(*) as count
			FROM messages_v2 mv
			LEFT JOIN user_last_read ulr ON ulr.room_id = mv.room_id
			WHERE mv.sender_id != $2::uuid
			AND mv.created_at > ulr.last_read
			GROUP BY mv.room_id
		)
		SELECT DISTINCT c.id, c.name, c.type, c.participants, c.created_at,
		       COALESCE(c.creator_username, ''), COALESCE(c.creator_id::text, ''),
		       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
		       COALESCE(c.allow_members_to_add, FALSE), COALESCE(c.is_secret, FALSE),
		       COALESCE(c.public_key_a, ''), COALESCE(c.e2ee_ready, FALSE),
		       COALESCE(c.last_message_text, ''), COALESCE(c.last_message_time, c.created_at),
		       COALESCE(ucm.pinned, FALSE), COALESCE(ucm.archived, FALSE), COALESCE(ucm.pinned_at, 0),
		       COALESCE(c.last_message_username, ''),
		       COALESCE(c.last_message_has_image, FALSE),
		       COALESCE(uc2.count, 0),
		       COALESCE(c.agent_id, ''), COALESCE(c.agent_mode, 'single'),
		       COALESCE(cc.company_id::text, ''), COALESCE(cc.access_level, 'member'),
		       COALESCE(cc.min_position_level, 0),
		       COALESCE(c.self_destruct_timer, 0)
		FROM chats c
		LEFT JOIN user_chat_metadata ucm ON ucm.room_id = c.id AND ucm.user_id = $2::uuid
		LEFT JOIN unread_counts uc2 ON c.id = uc2.room_id
		LEFT JOIN company_chats cc ON cc.chat_id = c.id
		WHERE (
			LOWER(c.name) LIKE $1
			OR LOWER(c.participants) LIKE $1
		)
		AND c.participants LIKE '%' || (SELECT username FROM user_info) || '%'
		ORDER BY COALESCE(c.last_message_time, c.created_at) DESC NULLS LAST
		LIMIT $3 OFFSET $4`,
		searchPattern, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatV2Row
	mutedSet := db.getMutedRoomsSet(userID)
	for rows.Next() {
		var c ChatV2Row
		var creatorId string
		err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.Creator, &creatorId, &c.AvatarURL, &c.FullAvatarURL,
			&c.AllowMembersToAdd, &c.IsSecret,
			&c.PeerPublicKey, &c.E2eeReady,
			&c.LastMessageText, &c.LastMessageTime,
			&c.IsPinned, &c.IsArchived, &c.PinnedAt,
			&c.LastMessageUsername, &c.LastMessageHasImage,
			&c.UnreadCount,
			&c.ActiveAgentId, &c.AgentMode,
			&c.CompanyId, &c.CompanyChatAccess, &c.CompanyMinPositionLevel,
			&c.SelfDestructTimer,
		)
		if err != nil {
			logger.Errorf("SearchChats scan error: %v", err)
			continue
		}
		c.IsMuted = mutedSet[c.ID]
		result = append(result, c)
	}

	if err := rows.Err(); err != nil {
		logger.Errorf("SearchChats rows error: %v", err)
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

// GetUserChatsV2Cursor returns chats with cursor-based pagination
func (db *DB) GetUserChatsV2Cursor(userID, username string, limit int, cursor, filter string) (*ChatV2Result, error) {
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

	// Cursor-based filtering
	cursorClause := ""
	bindArgs := []interface{}{userID, limit, username}
	if cur, ok := decodeCursor(cursor); ok {
		if filter == "pinned" {
			cursorClause = "AND COALESCE(ucm.pinned_at, 0) < $4"
			bindArgs = append(bindArgs, cur.PinnedAt)
		} else {
			cursorClause = "AND (COALESCE(ucm.pinned_at, 0), COALESCE(c.last_message_time, c.created_at)) < ($4, $5)"
			bindArgs = append(bindArgs, cur.PinnedAt, cur.LastMessageTime)
		}
	}

	// Fetch limit+1 to detect if there are more results
	fetchLimit := limit + 1
	bindArgs[1] = fetchLimit

	query := fmt.Sprintf(`
		WITH user_last_read AS (
			SELECT room_id, COALESCE(last_read_at, '1970-01-01') as last_read FROM user_chat_metadata WHERE user_id = $1::uuid
		),
		unread_counts AS (
			SELECT mv.room_id, COUNT(*) as count
			FROM messages_v2 mv
			LEFT JOIN user_last_read ulr ON ulr.room_id = mv.room_id
			WHERE mv.sender_id != $1::uuid
			AND mv.created_at > ulr.last_read
			GROUP BY mv.room_id
		)
		SELECT c.id, c.name, c.type, c.participants, c.created_at,
		       COALESCE(c.creator_username, ''), COALESCE(c.creator_id::text, ''),
		       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
		       COALESCE(c.allow_members_to_add, FALSE), COALESCE(c.is_secret, FALSE),
		       COALESCE(c.public_key_a, ''), COALESCE(c.e2ee_ready, FALSE),
		       COALESCE(c.last_message_text, ''), COALESCE(c.last_message_time, c.created_at),
		       COALESCE(ucm.pinned, FALSE), COALESCE(ucm.archived, FALSE),
		       COALESCE(ucm.pinned_at, 0),
		       COALESCE(c.last_message_username, ''),
		       COALESCE(c.last_message_has_image, FALSE),
		       COALESCE(uc2.count, 0),
		       COALESCE(c.agent_id, ''), COALESCE(c.agent_mode, 'single'),
		       COALESCE(cc.company_id::text, ''), COALESCE(cc.access_level, 'member'),
		       COALESCE(cc.min_position_level, 0),
		       COALESCE(c.self_destruct_timer, 0)
		FROM chats c
		LEFT JOIN user_chat_metadata ucm ON ucm.room_id = c.id AND ucm.user_id = $1::uuid
		LEFT JOIN muted_chats mc ON mc.room_id = c.id AND mc.user_id = $1::uuid
		LEFT JOIN unread_counts uc2 ON c.id = uc2.room_id
		LEFT JOIN company_chats cc ON cc.chat_id = c.id
		WHERE c.type NOT IN ('ai', 'owl', 'hermes')
		AND (c.participant_ids @> ARRAY[$1::uuid] OR c.participants::jsonb @> jsonb_build_array($3::text))
		%s
		%s
		%s
		LIMIT $2`, whereExtra, cursorClause, orderBy)

	rows, err := db.Query(query, bindArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatV2Row
	mutedSet := db.getMutedRoomsSet(userID)
	for rows.Next() {
		var c ChatV2Row
		var creatorId string

		err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.Creator, &creatorId, &c.AvatarURL, &c.FullAvatarURL,
			&c.AllowMembersToAdd, &c.IsSecret,
			&c.PeerPublicKey, &c.E2eeReady,
			&c.LastMessageText, &c.LastMessageTime,
			&c.IsPinned, &c.IsArchived, &c.PinnedAt,
			&c.LastMessageUsername, &c.LastMessageHasImage,
			&c.UnreadCount,
			&c.ActiveAgentId, &c.AgentMode,
			&c.CompanyId, &c.CompanyChatAccess, &c.CompanyMinPositionLevel,
			&c.SelfDestructTimer,
		)
		if err != nil {
			logger.Errorf("GetUserChatsV2 scan error: %v", err)
			continue
		}
		c.IsMuted = mutedSet[c.ID]
		result = append(result, c)
	}

	if err := rows.Err(); err != nil {
		logger.Errorf("GetUserChatsV2 rows error: %v", err)
	}

	// Determine if there are more results
	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}

	// Build next cursor from last row
	var nextCursor string
	if hasMore && len(result) > 0 {
		last := result[len(result)-1]
		nextCursor = encodeCursor(chatCursor{
			PinnedAt:        last.PinnedAt,
			LastMessageTime: last.LastMessageTime,
		})
	}

	return &ChatV2Result{
		Chats:      result,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
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

// getMutedRoomsSet returns all muted room IDs for a user in one query
func (db *DB) getMutedRoomsSet(userID string) map[string]bool {
	set := make(map[string]bool)
	rows, err := db.Query(`
		SELECT room_id FROM muted_chats
		WHERE user_id = $1::uuid AND muted = TRUE`,
		userID)
	if err != nil {
		return set
	}
	defer rows.Close()
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err == nil {
			set[roomID] = true
		}
	}
	return set
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
			room_id VARCHAR(255) NOT NULL,
			message_id VARCHAR(255) NOT NULL,
			pinned_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, room_id, message_id)
		)`,
		`DO $$ BEGIN ALTER TABLE pinned_messages ALTER COLUMN room_id TYPE VARCHAR(255) USING room_id::text; EXCEPTION WHEN duplicate_column THEN NULL; END $$`,
		`CREATE INDEX IF NOT EXISTS idx_pinned_messages_room ON pinned_messages(user_id, room_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "already has type") {
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
			WHERE message_id = $1 AND room_id = $2
		)`, messageID, chatID).Scan(&msgExists)
	if err != nil || !msgExists {
		return fmt.Errorf("message not found in this chat")
	}

	_, err = db.Exec(`
		INSERT INTO pinned_messages (user_id, room_id, message_id, pinned_at)
		VALUES ($1::uuid, $2, $3, $4)
		ON CONFLICT (user_id, room_id, message_id) DO NOTHING`,
		userID, chatID, messageID, time.Now().Unix())
	return err
}

// UnPinMessage removes a pinned message.
func (db *DB) UnPinMessage(userID, chatID, messageID string) error {
	_, err := db.Exec(`
		DELETE FROM pinned_messages
		WHERE user_id = $1::uuid AND room_id = $2 AND message_id = $3`,
		userID, chatID, messageID)
	return err
}

// GetPinnedMessages returns pinned messages for a user in a chat with pagination.
func (db *DB) GetPinnedMessages(userID, chatID string, limit, offset int) ([]PinnedMessageRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(`
		SELECT pm.message_id, pm.pinned_at, COALESCE(u.username, mv.sender_id::text), mv.text, mv.created_at
		FROM pinned_messages pm
		JOIN messages_v2 mv ON mv.id = pm.message_id AND mv.room_id = pm.room_id
		LEFT JOIN users u ON u.id = mv.sender_id
		WHERE pm.user_id = $1::uuid AND pm.room_id = $2
		ORDER BY pm.pinned_at DESC
		LIMIT $3 OFFSET $4`,
		userID, chatID, limit, offset)
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
			WHERE user_id = $1::uuid AND room_id = $2 AND message_id = $3
		)`, userID, chatID, messageID).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}
