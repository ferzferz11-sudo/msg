package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"time"
)

// MessageRowV2 represents a row from messages_v2 table.
type MessageRowV2 struct {
	ID           string
	RoomID       string
	SenderID     string
	SenderName   string // resolved from users table
	SenderAvatar string // resolved from users table
	ContentType  string
	Text         string
	MediaURL     string
	MediaURLs    string // JSON array
	Duration     int32
	ReplyToID    sql.NullString
	ReplyPreview sql.NullString // resolved from messages_v2.text
	Edited       bool
	IsRead       bool
	IsE2EE       bool
	E2EEPayload  []byte
	Reactions    string // JSON object
	CreatedAt    time.Time
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
	_, err := db.Exec(`INSERT INTO messages_v2 (id, room_id, sender_id, content_type, text, media_url, media_urls, duration, reply_to_id, reply_preview, edited, is_read, is_e2ee, e2ee_payload, reactions, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15::jsonb, $16)
		ON CONFLICT (id) DO NOTHING`,
		m.ID, m.RoomID, m.SenderID, m.ContentType, m.Text, m.MediaURL, mediaURLs, m.Duration, m.ReplyToID, m.ReplyPreview, m.Edited, m.IsRead, m.IsE2EE, m.E2EEPayload, reactions, m.CreatedAt)
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
		m.duration, m.reply_to_id, m.reply_preview, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), m.created_at
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
			&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.Edited, &r.IsRead, &r.IsE2EE,
			&r.E2EEPayload, &r.Reactions, &r.CreatedAt); err != nil {
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
		m.duration, m.reply_to_id, m.reply_preview, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), m.created_at
		FROM messages_v2 m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.id = $1`, id).Scan(&r.ID, &r.RoomID, &r.SenderID, &r.SenderName, &r.SenderAvatar,
		&r.ContentType, &r.Text, &r.MediaURL, &r.MediaURLs,
		&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.Edited, &r.IsRead, &r.IsE2EE,
		&r.E2EEPayload, &r.Reactions, &r.CreatedAt)
	return r, err
}

// EditMessageV2 updates the text content of a message.
func (db *DB) EditMessageV2(id, text string) error {
	_, err := db.Exec(`UPDATE messages_v2 SET text = $1, edited = TRUE WHERE id = $2`, text, id)
	return err
}

// DeleteMessageV2 soft-deletes messages by setting content_type to 'deleted'.
func (db *DB) DeleteMessageV2(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE messages_v2 SET content_type = 'deleted', text = '', media_url = '', media_urls = '[]', duration = 0, e2ee_payload = NULL, reactions = '{}' WHERE id = ANY($1)`
	_, err := db.Exec(query, ids)
	return err
}

// SetReactionV2 sets or removes a reaction on a message. emoji="" removes.
func (db *DB) SetReactionV2(messageID, userID, emoji string) (string, error) {
	// Read current reactions
	var reactionsJSON string
	err := db.QueryRow(`SELECT COALESCE(reactions, '{}') FROM messages_v2 WHERE id = $1`, messageID).Scan(&reactionsJSON)
	if err != nil {
		return "", err
	}

	var reactions map[string]string
	if err := json.Unmarshal([]byte(reactionsJSON), &reactions); err != nil {
		reactions = make(map[string]string)
	}

	if emoji == "" {
		delete(reactions, userID)
	} else {
		reactions[userID] = emoji
	}

	newJSON, _ := json.Marshal(reactions)
	_, err = db.Exec(`UPDATE messages_v2 SET reactions = $1 WHERE id = $2`, string(newJSON), messageID)
	return string(newJSON), err
}

// GetMessagesV2ByIDs returns messages by a list of IDs (for reactions batch load).
func (db *DB) GetMessagesV2ByIDs(ids []string) ([]MessageRowV2, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT m.id, m.room_id, m.sender_id::text,
		COALESCE(u.username, ''), COALESCE(u.avatar_url, ''),
		m.content_type, COALESCE(m.text, ''), COALESCE(m.media_url, ''), COALESCE(m.media_urls, '[]'),
		m.duration, m.reply_to_id, m.reply_preview, m.edited, m.is_read, m.is_e2ee,
		COALESCE(m.e2ee_payload, NULL), COALESCE(m.reactions, '{}'), m.created_at
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
			&r.Duration, &r.ReplyToID, &r.ReplyPreview, &r.Edited, &r.IsRead, &r.IsE2EE,
			&r.E2EEPayload, &r.Reactions, &r.CreatedAt); err != nil {
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
