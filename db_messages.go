package main

import (
	"database/sql"
	"strings"
	"time"
)

func (db *DB) SaveMessage(mid, user, uid string, enc []byte, created time.Time, rmid, ruser, rtext, room, img, imgUrls, voice string, dur int32, isE2EE ...bool) error {
	isRead := strings.HasPrefix(room, "favorites_")
	e2ee := false
	if len(isE2EE) > 0 && isE2EE[0] {
		e2ee = true
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO messages (message_id, username, user_id, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url, image_urls, voice_url, duration, is_e2ee)
	      VALUES ($1, $2::text, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		  ON CONFLICT (message_id) DO UPDATE SET
		  encrypted_text = EXCLUDED.encrypted_text,
		  edited = TRUE`,
		mid, user, uid, enc, created, rmid, ruser, rtext, room, isRead, img, imgUrls, voice, dur, e2ee)
	if err != nil {
		return err
	}

	if room != "" && !isRead {
		_, _ = tx.Exec(`UPDATE users SET chat_list_version=chat_list_version+1
			WHERE id IN (SELECT unnest(participant_ids) FROM chats WHERE id=$1)`, room)

		hasImage := img != "" || (imgUrls != "" && imgUrls != "[]")
		var preview string
		if e2ee {
			preview = "Encrypted message"
		} else if voice != "" {
			preview = "Voice message"
		} else if hasImage {
			preview = "Image"
		} else {
			preview, _ = decrypt(enc)
			if len(preview) > 500 {
				preview = preview[:500]
			}
		}
		_, _ = tx.Exec(`UPDATE chats SET
			last_message_text = $1,
			last_message_time = $2,
			last_message_username = $3,
			last_message_has_image = $4
		WHERE id = $5`, preview, created, user, hasImage, room)
	}

	return tx.Commit()
}

func (db *DB) GetMessages(limit int, room string) ([]MessageRow, error) {
	var rows *sql.Rows
	var err error
	if strings.HasPrefix(room, "favorites_") {
		username := strings.TrimPrefix(room, "favorites_")
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, COALESCE(f.created_at, m.created_at), COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), COALESCE(m.is_read, FALSE) as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = (SELECT id FROM users WHERE username = $1) WHERE m.room_id = $2 OR f.message_id IS NOT NULL ORDER BY 4 ASC LIMIT $3`
		rows, err = db.Query(q, username, room, limit)
	} else {
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), m.is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id WHERE m.room_id = $1 ORDER BY m.created_at DESC LIMIT $2`
		rows, err = db.Query(q, room, limit)
	}
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

func (db *DB) SetReaction(mid, user, emoji string) error {
	q := `INSERT INTO reactions (message_id, username, emoji) VALUES ($1, $2, $3) ON CONFLICT (message_id, username) DO UPDATE SET emoji = EXCLUDED.emoji`
	_, err := db.Exec(q, mid, user, emoji)
	return err
}

func (db *DB) SetReactionByUserID(mid, userID, emoji string) error {
	q := `INSERT INTO reactions (message_id, user_id, username, emoji)
		VALUES ($1, $2::uuid, (SELECT username FROM users WHERE id=$2::uuid), $3)
		ON CONFLICT (message_id, username) DO UPDATE SET emoji = EXCLUDED.emoji, user_id = EXCLUDED.user_id`
	_, err := db.Exec(q, mid, userID, emoji)
	return err
}

func (db *DB) RemoveReactionByUserID(mid, userID string) error {
	_, err := db.Exec(`DELETE FROM reactions WHERE message_id=$1 AND (user_id=$2::uuid OR username=$2)`, mid, userID)
	return err
}

func (db *DB) GetReactionsForMessage(mid string) ([]struct{ Username, Emoji string }, error) {
	rows, err := db.Query(`SELECT username, emoji FROM reactions WHERE message_id=$1`, mid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct{ Username, Emoji string }
	for rows.Next() {
		var r struct{ Username, Emoji string }
		rows.Scan(&r.Username, &r.Emoji)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) GetMessageByUUID(id string) (MessageRow, error) {
	var r MessageRow
	err := db.QueryRow(`SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), COALESCE(m.is_read, FALSE) as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id WHERE m.message_id = $1`, id).Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.ImageURLs, &r.Edited, &r.VoiceURL, &r.Duration, &r.IsE2EE)
	return r, err
}

func (db *DB) DeleteMessageByUUID(id string) error {
	_, err := db.Exec(`DELETE FROM messages WHERE message_id = $1`, id)
	return err
}

func (db *DB) DeleteMessageByID(id int) error {
	_, err := db.Exec(`DELETE FROM messages WHERE id = $1`, id)
	return err
}

func (db *DB) GetMessagesByUserAndTime(u string, t time.Time) ([]struct {
	ID                                     int
	Encrypted                              []byte
	ImageURL, ImageURLs, MessageID, RoomID string
}, error) {
	rows, err := db.Query(`SELECT id, encrypted_text, COALESCE(image_url, ''), COALESCE(image_urls, '[]'), message_id, room_id FROM messages WHERE username = $1 AND created_at >= $2 AND created_at <= $3`, u, t.Add(-2*time.Second), t.Add(2*time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID                                     int
		Encrypted                              []byte
		ImageURL, ImageURLs, MessageID, RoomID string
	}
	for rows.Next() {
		var r struct {
			ID                                     int
			Encrypted                              []byte
			ImageURL, ImageURLs, MessageID, RoomID string
		}
		rows.Scan(&r.ID, &r.Encrypted, &r.ImageURL, &r.ImageURLs, &r.MessageID, &r.RoomID)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) UpdateMessageText(id, text string) error {
	enc, _ := encrypt(text)
	_, err := db.Exec(`UPDATE messages SET encrypted_text=$1, edited=TRUE WHERE message_id=$2`, enc, id)
	return err
}

func (db *DB) GetMessageImageURL(mid string) (string, error) {
	var u string
	db.QueryRow(`SELECT COALESCE(image_url,'') FROM messages WHERE message_id=$1`, mid).Scan(&u)
	return u, nil
}

func (db *DB) GetChatMessagesImageURLs(room string) ([]string, error) {
	rows, _ := db.Query(`SELECT image_url FROM messages WHERE room_id=$1 AND image_url!=''`, room)
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

func (db *DB) CleanupEmptyMessages() (int64, error) {
	q := `DELETE FROM messages WHERE encrypted_text = 'DECRYPTION_FAILED'::bytea OR encrypted_text = 'CORRUPTED_FIX'::bytea`
	res, err := db.Exec(q)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (db *DB) GetChatMessages(room string) ([]MessageRow, error) {
	return db.GetMessages(100, room)
}

func (db *DB) GetFavorites(uid string) ([]MessageRow, error) {
	username := uid
	if len(uid) > 20 {
		_ = db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid OR username = $1`, uid).Scan(&username)
	}

	q := `SELECT COALESCE(mv.id, ''), COALESCE(u.username, mv.sender_id::text), mv.text, COALESCE(f.created_at, mv.created_at), mv.reply_to_id, '', COALESCE(mv.reply_preview, ''), mv.room_id, mv.is_read, COALESCE(u.avatar_url, ''), mv.media_url, COALESCE(mv.media_urls, '[]')::text, mv.edited, '', mv.duration, mv.is_e2ee
	      FROM messages_v2 mv
	      LEFT JOIN users u ON mv.sender_id = u.id
	      LEFT JOIN favorites f ON f.message_id = mv.id AND f.user_id = (SELECT id FROM users WHERE username = $1::text)
	      WHERE mv.room_id = 'favorites_' || $1::text OR (f.message_id IS NOT NULL AND f.user_id = (SELECT id FROM users WHERE username = $1::text))
	      ORDER BY 4 ASC`
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

func (db *DB) AddFavorite(uid, mid string) error {
	query := `INSERT INTO favorites (user_id, message_id)
	          VALUES (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END, $2)
	          ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, uid, mid)
	return err
}

func (db *DB) RemoveFavorite(uid, mid string) error {
	query := `DELETE FROM favorites
	          WHERE user_id = (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END)
	          AND message_id = $2`
	_, err := db.Exec(query, uid, mid)
	return err
}

func (db *DB) GetFavoritesMessages(uid string) ([]MessageRow, error) {
	return db.GetFavorites(uid)
}
