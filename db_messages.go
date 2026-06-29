package main

// db_messages.go — Legacy v1 functions REMOVED.
// Tables `messages` and `reactions` have been dropped.
// All message operations now use messages_v2 (see db_messages_v2.go).
//
// Kept functions (still used by other code):
// - GetReactionsForMessage (reads from messages_v2.reactions JSONB)
// - GetChatMessagesImageURLs (reads from messages_v2)
// - GetFavorites / GetFavoritesMessages (reads from messages_v2)
// - AddFavorite / RemoveFavorite (favorites table)

import (
	"encoding/json"
)

// GetReactionsForMessage reads reactions from messages_v2.reactions JSONB.
func (db *DB) GetReactionsForMessage(mid string) ([]struct{ Username, Emoji string }, error) {
	var reactionsJSON string
	err := db.QueryRow(`SELECT reactions FROM messages_v2 WHERE id = $1`, mid).Scan(&reactionsJSON)
	if err != nil {
		return nil, err
	}
	if reactionsJSON == "" || reactionsJSON == "{}" {
		return nil, nil
	}
	var reactionMap map[string]string
	if err := json.Unmarshal([]byte(reactionsJSON), &reactionMap); err != nil {
		return nil, err
	}
	var res []struct{ Username, Emoji string }
	for uid, emoji := range reactionMap {
		res = append(res, struct{ Username, Emoji string }{Username: uid, Emoji: emoji})
	}
	return res, nil
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

// GetFavorites returns favorite messages from messages_v2 via the favorites table.
func (db *DB) GetFavorites(uid string) ([]MessageRow, error) {
	username := uid
	if len(uid) > 20 {
		_ = db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid OR username = $1`, uid).Scan(&username)
	}

	q := `SELECT mv.id, COALESCE(u.username, mv.sender_id::text), mv.text, mv.created_at, mv.reply_to_id, '', COALESCE(mv.reply_preview, ''), mv.room_id, mv.is_read, COALESCE(u.avatar_url, ''), mv.media_url, COALESCE(mv.media_urls, '[]')::text, mv.edited, '', mv.duration, mv.is_e2ee
	      FROM favorites f
	      JOIN messages_v2 mv ON mv.id = f.message_id
	      LEFT JOIN users u ON mv.sender_id = u.id
	      WHERE f.user_id = (SELECT id FROM users WHERE username = $1::text)
	      ORDER BY f.created_at ASC`
	rows, err := db.Query(q, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MessageRow
	for rows.Next() {
		var r MessageRow
		rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.ImageURLs, &r.Edited, &r.VoiceURL, &r.Duration, &r.IsE2EE)
		res = append(res, r)
	}
	return res, nil
}

// AddFavorite adds a message to favorites.
func (db *DB) AddFavorite(uid, mid string) error {
	query := `INSERT INTO favorites (user_id, message_id)
	          VALUES (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END, $2)
	          ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, uid, mid)
	return err
}

// RemoveFavorite removes a message from favorites.
func (db *DB) RemoveFavorite(uid, mid string) error {
	query := `DELETE FROM favorites
	          WHERE user_id = (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END)
	          AND message_id = $2`
	_, err := db.Exec(query, uid, mid)
	return err
}

// GetFavoritesMessages is an alias for GetFavorites.
func (db *DB) GetFavoritesMessages(uid string) ([]MessageRow, error) {
	return db.GetFavorites(uid)
}
