package main

import (
	"database/sql"
	"encoding/base64"
	"time"
)

// MessageRowV2 represents a row from messages_v2 table.
type MessageRowV2 struct {
	ID            string
	RoomID        string
	SenderID      string
	SenderName    string
	SenderAvatar  string
	ContentType   string
	Text          string
	MediaURL      string
	MediaURLs     string
	Duration      int32
	ReplyToID     sql.NullString
	ReplyPreview  sql.NullString
	ReplySenderID sql.NullString
	Edited        bool
	IsRead        bool
	IsE2EE        bool
	E2EEPayload   []byte
	Reactions     string
	Mentions      sql.NullString
	ForwardedFrom string
	CreatedAt     time.Time
}

// SaveMessageV2 inserts a new message into messages_v2.
func (db *DB) SaveMessageV2(m *MessageRowV2) error {
	mediaURLs := m.MediaURLs
	if mediaURLs == "" {
		mediaURLs = "[]"
	}
	reactions := m.Reactions
	if reactions == "" {
		reactions = "{}"
	}
	mentions := "[]"
	if m.Mentions.Valid && m.Mentions.String != "" {
		mentions = m.Mentions.String
	}
	_, err := db.Exec(`INSERT INTO messages_v2 (id, room_id, sender_id, content_type, text, media_url, media_urls, duration, reply_to_id, reply_preview, reply_sender_id, edited, is_read, is_e2ee, e2ee_payload, reactions, mentions, created_at, forwarded_from)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17, $18, $19)
		ON CONFLICT (id) DO NOTHING`,
		m.ID, m.RoomID, m.SenderID, m.ContentType, m.Text, m.MediaURL, mediaURLs, m.Duration, m.ReplyToID, m.ReplyPreview, m.ReplySenderID, m.Edited, m.IsRead, m.IsE2EE, m.E2EEPayload, reactions, mentions, m.CreatedAt, m.ForwardedFrom)
	return err
}

// GetMessagesV2Cursor returns messages for a room using cursor-based pagination.
// cursor format: "created_at_rfc3339:message_id" (URL-safe base64 encoded).
func (db *DB) GetMessagesV2Cursor(roomID string, limit int, cursor string) ([]MessageRowV2, string, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var rows *sql.Rows
	var err error

	baseQuery := `SELECT m.id, m.room_id, m.sender_id::text,
		COALESCE(u.username, ''), COALESCE(u.avatar_url, ''),
		m.content_type, COALESCE(m.text, ''), COALESCE(m.media_url, ''), COALESCE(m.media_urls, '[]'),
		m.duration, m.reply_to_id, m.reply_preview, m.reply_sender_id, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), COALESCE(m.mentions, '[]'), m.created_at,
		COALESCE(m.forwarded_from, '')
		FROM messages_v2 m
		LEFT JOIN users u ON m.sender_id = u.id`

	if cursor != "" {
		cursorTime, cursorID := decodeMessageCursor(cursor)
		rows, err = db.Query(baseQuery+` WHERE m.room_id = $1 AND (m.created_at, m.id) < ($2, $3) ORDER BY m.created_at DESC, m.id DESC LIMIT $4`,
			roomID, cursorTime, cursorID, limit+1)
	} else {
		rows, err = db.Query(baseQuery+` WHERE m.room_id = $1 ORDER BY m.created_at DESC, m.id DESC LIMIT $2`,
			roomID, limit+1)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []MessageRowV2
	for rows.Next() {
		var r MessageRowV2
		if err := rows.Scan(&r.ID, &r.RoomID, &r.SenderID, &r.SenderName, &r.SenderAvatar,
			&r.ContentType, &r.Text, &r.MediaURL, &r.MediaURLs,
			&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.ReplySenderID, &r.Edited, &r.IsRead, &r.IsE2EE,
			&r.E2EEPayload, &r.Reactions, &r.Mentions, &r.CreatedAt, &r.ForwardedFrom); err != nil {
			return nil, "", err
		}
		result = append(result, r)
	}

	// Determine if there are more results
	var nextCursor string
	if len(result) > limit {
		result = result[:limit]
		last := result[len(result)-1]
		nextCursor = encodeMessageCursor(last.CreatedAt, last.ID)
	}

	// Reverse to ascending order (we fetched DESC for cursor logic)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nextCursor, nil
}

// GetMessageV2ByUUID returns a single message by ID.
func (db *DB) GetMessageV2ByUUID(id string) (MessageRowV2, error) {
	var r MessageRowV2
	err := db.QueryRow(`SELECT m.id, m.room_id, m.sender_id::text,
		COALESCE(u.username, ''), COALESCE(u.avatar_url, ''),
		m.content_type, COALESCE(m.text, ''), COALESCE(m.media_url, ''), COALESCE(m.media_urls, '[]'),
		m.duration, m.reply_to_id, m.reply_preview, m.reply_sender_id, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), COALESCE(m.mentions, '[]'), m.created_at,
		COALESCE(m.forwarded_from, '')
		FROM messages_v2 m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.id = $1`, id).Scan(&r.ID, &r.RoomID, &r.SenderID, &r.SenderName, &r.SenderAvatar,
		&r.ContentType, &r.Text, &r.MediaURL, &r.MediaURLs,
		&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.ReplySenderID, &r.Edited, &r.IsRead, &r.IsE2EE,
		&r.E2EEPayload, &r.Reactions, &r.Mentions, &r.CreatedAt, &r.ForwardedFrom)
	return r, err
}

// EditMessageV2 updates the text content of a message.
func (db *DB) EditMessageV2(id, text string) error {
	_, err := db.Exec(`UPDATE messages_v2 SET text = $1, edited = TRUE WHERE id = $2`, text, id)
	return err
}

// DeleteMessageV2 permanently deletes messages from the database.
func (db *DB) DeleteMessageV2(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	// Clean up saved_messages references before deleting messages
	db.Exec(`DELETE FROM saved_messages WHERE message_id = ANY($1)`, ids)
	query := `DELETE FROM messages_v2 WHERE id = ANY($1)`
	_, err := db.Exec(query, ids)
	return err
}

// ClearRoomHistory deletes all messages in a room for the requesting user.
func (db *DB) ClearRoomHistory(roomID string) error {
	db.Exec(`DELETE FROM saved_messages WHERE message_id IN (SELECT id FROM messages_v2 WHERE room_id = $1)`, roomID)
	_, err := db.Exec(`DELETE FROM messages_v2 WHERE room_id = $1`, roomID)
	if err != nil {
		return err
	}
	// Also clean up deleted_messages tracking for this room
	db.Exec(`DELETE FROM deleted_messages WHERE room_id = $1`, roomID)
	return nil
}

// UpdateChatLastMessage recalculates and updates the last message fields in chats table
// after a message is deleted or edited.
func (db *DB) UpdateChatLastMessage(roomID string) {
	var text, senderName, contentType string
	var createdAt time.Time
	var hasImage bool

	err := db.QueryRow(`
		SELECT COALESCE(m.text, ''), COALESCE(u.username, ''), m.created_at, m.content_type
		FROM messages_v2 m
		LEFT JOIN users u ON u.id = m.sender_id::uuid
		WHERE m.room_id = $1
		ORDER BY m.created_at DESC
		LIMIT 1
	`, roomID).Scan(&text, &senderName, &createdAt, &contentType)

	if err != nil {
		// No messages left — clear last message
		db.Exec(`UPDATE chats SET last_message_text = '', last_message_time = NULL, last_message_username = '', last_message_has_image = FALSE WHERE id = $1`, roomID)
		return
	}

	preview := text
	if len(preview) > 500 {
		preview = preview[:500]
	}
	if contentType == "image" {
		preview = "Image"
		hasImage = true
	} else if contentType == "voice" {
		preview = "Voice message"
	}

	// Skip system messages (🔥, 📹, 📞) — find the last non-system message
	if isSystemMessage(preview) {
		var altText, altSender, altContentType string
		var altTime time.Time
		err = db.QueryRow(`
			SELECT COALESCE(m.text, ''), COALESCE(u.username, ''), m.created_at, m.content_type
			FROM messages_v2 m
			LEFT JOIN users u ON u.id = m.sender_id::uuid
			WHERE m.room_id = $1 AND m.text NOT LIKE '🔥%' AND m.text NOT LIKE '📹%' AND m.text NOT LIKE '📞%'
			ORDER BY m.created_at DESC
			LIMIT 1
		`, roomID).Scan(&altText, &altSender, &altTime, &altContentType)
		if err == nil {
			text = altText
			senderName = altSender
			createdAt = altTime
			contentType = altContentType
			preview = text
			if len(preview) > 500 {
				preview = preview[:500]
			}
			if contentType == "image" {
				preview = "Image"
				hasImage = true
			} else if contentType == "voice" {
				preview = "Voice message"
			}
		}
	}

	db.Exec(`UPDATE chats SET last_message_text = $1, last_message_time = $2, last_message_username = $3, last_message_has_image = $4 WHERE id = $5`,
		preview, createdAt, senderName, hasImage, roomID)
}

// SetReactionV2 sets or removes a reaction on a message. emoji="" removes.
func (db *DB) SetReactionV2(messageID, userID, emoji string) (string, error) {
	var resultJSON string
	if emoji == "" {
		err := db.QueryRow(`
			UPDATE messages_v2 SET reactions = reactions - $1::text WHERE id = $2::text
			RETURNING COALESCE(reactions, '{}')`, userID, messageID).Scan(&resultJSON)
		return resultJSON, err
	}

	err := db.QueryRow(`
		UPDATE messages_v2 SET reactions = reactions || jsonb_build_object($1::text, $2::text) WHERE id = $3::text
		RETURNING COALESCE(reactions, '{}')`, userID, emoji, messageID).Scan(&resultJSON)
	return resultJSON, err
}

// GetMessagesV2ByIDs returns messages by a list of IDs (for reactions batch load).
func (db *DB) GetMessagesV2ByIDs(ids []string) ([]MessageRowV2, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT m.id, m.room_id, m.sender_id::text,
		COALESCE(u.username, ''), COALESCE(u.avatar_url, ''),
		m.content_type, COALESCE(m.text, ''), COALESCE(m.media_url, ''), COALESCE(m.media_urls, '[]'),
		m.duration, m.reply_to_id, m.reply_preview, m.reply_sender_id, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), COALESCE(m.mentions, '[]'), m.created_at,
		COALESCE(m.forwarded_from, '')
		FROM messages_v2 m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.id = ANY($1) ORDER BY m.created_at ASC`
	rows, err := db.Query(query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []MessageRowV2
	for rows.Next() {
		var r MessageRowV2
		if err := rows.Scan(&r.ID, &r.RoomID, &r.SenderID, &r.SenderName, &r.SenderAvatar,
			&r.ContentType, &r.Text, &r.MediaURL, &r.MediaURLs,
			&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.ReplySenderID, &r.Edited, &r.IsRead, &r.IsE2EE,
			&r.E2EEPayload, &r.Reactions, &r.Mentions, &r.CreatedAt, &r.ForwardedFrom); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// MarkReadV2 marks all messages in a room up to a given message ID as read.
func (db *DB) MarkReadV2(roomID, messageID string) error {
	_, err := db.Exec(`UPDATE messages_v2 SET is_read = TRUE WHERE room_id = $1 AND is_read = FALSE AND created_at <= (SELECT created_at FROM messages_v2 WHERE id = $2)`, roomID, messageID)
	return err
}

// Cursor helpers — encode/decode "timestamp_rfc3339:message_id" as base64

func encodeMessageCursor(t time.Time, id string) string {
	payload := t.Format(time.RFC3339Nano) + "|" + id
	return base64Encode(payload)
}

func decodeMessageCursor(cursor string) (time.Time, string) {
	payload := base64Decode(cursor)
	for i := len(payload) - 1; i >= 0; i-- {
		if payload[i] == '|' {
			t, _ := time.Parse(time.RFC3339Nano, payload[:i])
			return t, payload[i+1:]
		}
	}
	return time.Time{}, ""
}

func base64Encode(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func base64Decode(s string) string {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// ======= Self-Destruct Timer =======

// SetSelfDestructTimer sets the self-destruct timer for a chat.
// When timer > 0, sets self_destruct_set_at = NOW() so only new messages are affected.
// When timer == 0 (disabled), clears self_destruct_set_at.
func (db *DB) SetSelfDestructTimer(roomID string, timerSeconds int) error {
	if timerSeconds > 0 {
		_, err := db.Exec(`UPDATE chats SET self_destruct_timer = $1, self_destruct_set_at = NOW() WHERE id = $2`, timerSeconds, roomID)
		return err
	}
	_, err := db.Exec(`UPDATE chats SET self_destruct_timer = 0, self_destruct_set_at = NULL WHERE id = $1`, roomID)
	return err
}

// GetSelfDestructTimer returns the self-destruct timer value for a chat.
func (db *DB) GetSelfDestructTimer(roomID string) (int, error) {
	var timer int
	err := db.QueryRow(`SELECT COALESCE(self_destruct_timer, 0) FROM chats WHERE id = $1`, roomID).Scan(&timer)
	return timer, err
}

// GetChatsWithSelfDestruct returns all chat IDs that have self-destruct enabled.
func (db *DB) GetChatsWithSelfDestruct() ([]struct {
	RoomID string
	Timer  int
	SetAt  time.Time
}, error) {
	rows, err := db.Query(`SELECT id, self_destruct_timer, self_destruct_set_at FROM chats WHERE self_destruct_timer > 0 AND self_destruct_set_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []struct {
		RoomID string
		Timer  int
		SetAt  time.Time
	}
	for rows.Next() {
		var r struct {
			RoomID string
			Timer  int
			SetAt  time.Time
		}
		if err := rows.Scan(&r.RoomID, &r.Timer, &r.SetAt); err == nil {
			result = append(result, r)
		}
	}
	return result, nil
}

// DeleteExpiredSelfDestructMessages deletes messages older than the timer for chats with self-destruct enabled.
// Only messages created AFTER the timer was set are affected. System messages are never deleted.
func (db *DB) DeleteExpiredSelfDestructMessages() (map[string][]string, error) {
	chats, err := db.GetChatsWithSelfDestruct()
	if err != nil {
		return nil, err
	}

	affected := make(map[string][]string)
	for _, c := range chats {
		rows, err := db.Query(`
			SELECT id FROM messages_v2
			WHERE room_id = $1
			  AND created_at > $2
			  AND created_at < NOW() - INTERVAL '1 second' * $3
			  AND content_type != 'system'`,
			c.RoomID, c.SetAt, c.Timer)
		if err != nil {
			continue
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()

		if len(ids) > 0 {
			for _, id := range ids {
				db.Exec(`INSERT INTO deleted_messages (message_id, room_id, deleted_by) VALUES ($1, $2, 'self_destruct') ON CONFLICT DO NOTHING`, id, c.RoomID)
			}
			db.Exec(`DELETE FROM saved_messages WHERE message_id = ANY($1)`, ids)
			db.Exec(`DELETE FROM messages_v2 WHERE id = ANY($1)`, ids)
			affected[c.RoomID] = ids
		}
	}
	return affected, nil
}

// ======= Deleted Messages =======

// InsertDeletedMessages records deleted message IDs for persistence.
func (db *DB) InsertDeletedMessages(ids []string, roomID, deletedBy string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		_, _ = db.Exec(`INSERT INTO deleted_messages (message_id, room_id, deleted_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, roomID, deletedBy)
	}
	return nil
}

// GetDeletedMessageIDs returns a set of deleted message IDs for a room.
func (db *DB) GetDeletedMessageIDs(roomID string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT message_id FROM deleted_messages WHERE room_id = $1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			set[id] = true
		}
	}
	return set, nil
}

// CleanupDeletedMessages removes entries older than 30 days.
func (db *DB) CleanupDeletedMessages() {
	db.Exec(`DELETE FROM deleted_messages WHERE deleted_at < NOW() - INTERVAL '30 days'`)
}

// SearchResultRow represents a single search result.
type SearchResultRow struct {
	MessageID string
	RoomID    string
	Username  string
	Preview   string
	CreatedAt string
}

// SearchMessages searches messages in messages_v2 table.
func (db *DB) SearchMessages(userID, roomID, query string, limit int) ([]SearchResultRow, error) {
	var rows *sql.Rows
	var err error

	if roomID != "" {
		rows, err = db.Query(`
			SELECT mv.id, mv.room_id,
				COALESCE(u.username, mv.sender_id::text) as username,
				mv.created_at::text,
				LEFT(CASE
					WHEN mv.text != '' THEN mv.text
					WHEN mv.content_type = 'image' THEN '[image]'
					WHEN mv.content_type = 'voice' THEN '[voice]'
					WHEN mv.content_type = 'file' THEN '[file]'
					ELSE ''
				END, 200) as preview
			FROM messages_v2 mv
			LEFT JOIN users u ON u.id = mv.sender_id
			WHERE mv.room_id = $1 AND mv.text ILIKE '%' || $2 || '%'
			ORDER BY mv.created_at DESC
			LIMIT $3`, roomID, query, limit)
	} else {
		rows, err = db.Query(`
			SELECT mv.id, mv.room_id,
				COALESCE(u.username, mv.sender_id::text) as username,
				mv.created_at::text,
				LEFT(CASE
					WHEN mv.text != '' THEN mv.text
					WHEN mv.content_type = 'image' THEN '[image]'
					WHEN mv.content_type = 'voice' THEN '[voice]'
					WHEN mv.content_type = 'file' THEN '[file]'
					ELSE ''
				END, 200) as preview
			FROM messages_v2 mv
			INNER JOIN chats c ON c.id = mv.room_id
			LEFT JOIN users u ON u.id = mv.sender_id
			WHERE c.participants::text LIKE '%' || $1 || '%'
				AND mv.text ILIKE '%' || $2 || '%'
			ORDER BY mv.created_at DESC
			LIMIT $3`, userID, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResultRow
	for rows.Next() {
		var r SearchResultRow
		if err := rows.Scan(&r.MessageID, &r.RoomID, &r.Username, &r.CreatedAt, &r.Preview); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}
