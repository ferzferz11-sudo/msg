package main

import (
	"database/sql"
)

// GetSavedMessages returns saved messages (MessageRowV2) from messages_v2 via the saved_messages table.
func (db *DB) GetSavedMessages(uid string) ([]MessageRowV2, error) {
	var userID string
	if len(uid) > 20 {
		userID = uid
	} else {
		err := db.QueryRow(`SELECT id::text FROM users WHERE username = $1`, uid).Scan(&userID)
		if err != nil {
			return nil, err
		}
	}

	q := `SELECT mv.id, mv.room_id, mv.sender_id::text, COALESCE(u.username, ''), mv.content_type, mv.text,
	             mv.media_url, COALESCE(mv.media_urls, '[]')::text, mv.duration,
	             mv.reply_to_id, mv.reply_preview, mv.reply_sender_id,
	             mv.edited, mv.is_read, mv.is_e2ee, mv.e2ee_payload,
	             COALESCE(mv.reactions, '{}'), mv.forwarded_from, mv.created_at
	      FROM saved_messages f
	      JOIN messages_v2 mv ON mv.id = f.message_id
	      LEFT JOIN users u ON mv.sender_id = u.id
	      WHERE f.user_id = $1::uuid
	      ORDER BY f.created_at ASC`
	rows, err := db.Query(q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MessageRowV2
	for rows.Next() {
		var r MessageRowV2
		var replyToID, replyPreview, replySenderID sql.NullString
		var mentions sql.NullString
		err := rows.Scan(
			&r.ID, &r.RoomID, &r.SenderID, &r.SenderName, &r.ContentType, &r.Text,
			&r.MediaURL, &r.MediaURLs, &r.Duration,
			&replyToID, &replyPreview, &replySenderID,
			&r.Edited, &r.IsRead, &r.IsE2EE, &r.E2EEPayload,
			&r.Reactions, &r.ForwardedFrom, &r.CreatedAt,
		)
		if err != nil {
			continue
		}
		r.ReplyToID = replyToID
		r.ReplyPreview = replyPreview
		r.ReplySenderID = replySenderID
		r.Mentions = mentions
		res = append(res, r)
	}
	return res, nil
}

// AddSavedMessage adds a message to the saved_messages table.
func (db *DB) AddSavedMessage(uid, mid string) error {
	query := `INSERT INTO saved_messages (user_id, message_id)
	          VALUES (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END, $2)
	          ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, uid, mid)
	return err
}

// RemoveSavedMessage removes a message from the saved_messages table.
func (db *DB) RemoveSavedMessage(uid, mid string) error {
	query := `DELETE FROM saved_messages
	          WHERE user_id = (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END)
	          AND message_id = $2`
	_, err := db.Exec(query, uid, mid)
	return err
}

// GetChatMessagesImageURLs returns image URLs for a room from messages_v2.
func (db *DB) GetChatMessagesImageURLs(room string) ([]string, error) {
	rows, _ := db.Query(`SELECT media_url FROM messages_v2 WHERE room_id=$1 AND content_type='image' AND media_url!=''`, room)
	var res []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var u string
			rows.Scan(&u)
			res = append(res, u)
		}
	}
	return res, nil
}
