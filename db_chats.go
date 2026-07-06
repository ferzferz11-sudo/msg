package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

func (db *DB) GetChat(id string) (struct {
	ID, Name, Type, Participants, CreatorUsername string
	CreatedAt                                     time.Time
	CreatorId                                     string
	AllowMembersToAdd                             bool
	IsSecret                                      bool
}, error) {
	var c struct {
		ID, Name, Type, Participants, CreatorUsername string
		CreatedAt                                     time.Time
		CreatorId                                     string
		AllowMembersToAdd                             bool
		IsSecret                                      bool
	}
	err := db.QueryRow(`SELECT id, name, type, participants, COALESCE(creator_username, ''), COALESCE(creator_id::text, ''), created_at, COALESCE(allow_members_to_add, FALSE), COALESCE(is_secret, FALSE) FROM chats WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatorUsername, &c.CreatorId, &c.CreatedAt, &c.AllowMembersToAdd, &c.IsSecret)
	return c, err
}

func (db *DB) GetAllChats() ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `SELECT c.id, c.name, c.type, c.participants, c.created_at,
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Creator, &c.LastTime, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, 0, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) GetUserChats(uid, user string) ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `WITH user_last_read AS (
		SELECT room_id, COALESCE(last_read_at, '1970-01-01') as last_read FROM user_chat_metadata WHERE user_id = $2::uuid
	),
	unread_counts AS (
		SELECT mv.room_id, COUNT(*) as count
		FROM messages_v2 mv
		LEFT JOIN user_last_read ulr ON ulr.room_id = mv.room_id
		WHERE mv.sender_id != $2::uuid
		AND mv.created_at > ulr.last_read
		GROUP BY mv.room_id
	)
	SELECT c.id, c.name, c.type, c.participants, c.created_at,
	       COALESCE(uc.count, 0),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	LEFT JOIN unread_counts uc ON c.id = uc.room_id
	WHERE c.type NOT IN ('ai', 'owl', 'hermes')
	  AND c.participants::jsonb @> jsonb_build_array($1::text)
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query, user, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			Unread                                                              int
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Unread, &c.LastTime, &c.Creator, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, c.Unread, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) GetUserChatsByUserID(userID string) ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `WITH user_last_read AS (
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
	       COALESCE(uc.count, 0),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	LEFT JOIN unread_counts uc ON c.id = uc.room_id
	WHERE c.type NOT IN ('owl', 'hermes', 'ai')
		AND (c.participant_ids @> ARRAY[$1::uuid] OR c.participants::jsonb @> jsonb_build_array((SELECT username FROM users WHERE id=$1::uuid)))
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			Unread                                                              int
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Unread, &c.LastTime, &c.Creator, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, c.Unread, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) IncrementParticipantsChatListVersion(id string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id IN (SELECT unnest(participant_ids) FROM chats WHERE id=$1)`, id)
	return err
}

func (db *DB) IncrementParticipantsChatListVersionByChatID(chatID string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id IN (
		SELECT unnest(participant_ids) FROM chats WHERE id=$1
		UNION
		SELECT id FROM users WHERE username IN (SELECT json_array_elements_text(participants::json) FROM chats WHERE id=$1)
	)`, chatID)
	return err
}

// IncrementChatListVersionByUsernames batch-updates chat_list_version for a list of usernames
func (db *DB) IncrementChatListVersionByUsernames(usernames []string) error {
	if len(usernames) == 0 {
		return nil
	}
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE username = ANY($1::text[])`, pq.Array(usernames))
	return err
}

func (db *DB) CreateChat(id, name, t, p, creatorUsername, creatorId string) error {
	_, err := db.Exec(`WITH parts AS (SELECT json_array_elements_text($1::json) AS username)
		INSERT INTO chats (id, name, type, participants, creator_username, creator_id, participant_ids)
		VALUES ($2, $3, $4, $1, $5, $6,
			(SELECT array_agg(u.id ORDER BY u.username) FROM users u JOIN parts p ON p.username = u.username)
		)`, p, id, name, t, creatorUsername, creatorId)
	if err == nil {
		_ = db.IncrementParticipantsChatListVersion(id)
	}
	return err
}

func (db *DB) GetChatParticipants(id string) ([]string, error) {
	var participantsJSON string
	err := db.QueryRow(`SELECT participants FROM chats WHERE id=$1`, id).Scan(&participantsJSON)
	if err != nil {
		return nil, err
	}
	var participants []string
	err = json.Unmarshal([]byte(participantsJSON), &participants)
	return participants, err
}

func (db *DB) GetDirectChatBetweenUsers(u1, u2 string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM chats WHERE type='direct' AND participants::jsonb @> jsonb_build_array($1::text, $2::text)`, u1, u2).Scan(&id)
	if err == nil {
		return id, nil
	}
	baseId := u1 + "_" + u2 + "_direct"
	if u1 > u2 {
		baseId = u2 + "_" + u1 + "_direct"
	}
	id = baseId + "_" + fmt.Sprintf("%d", time.Now().Unix())
	db.CreateChat(id, u1+" & "+u2, "direct", `["`+u1+`","`+u2+`"]`, u1, "")
	return id, nil
}

func (db *DB) UpdateChatName(id, name string) error {
	_, err := db.Exec(`UPDATE chats SET name=$1 WHERE id=$2`, name, id)
	return err
}

func (db *DB) UpdateChatAvatarWithFull(id, a, f string) error {
	_, err := db.Exec(`UPDATE chats SET avatar_url=$1, full_avatar_url=$2 WHERE id=$3`, a, f, id)
	return err
}

func (db *DB) UpdateChatSettings(id string, allowAdd bool) error {
	_, err := db.Exec(`UPDATE chats SET allow_members_to_add=$1 WHERE id=$2`, allowAdd, id)
	return err
}

func (db *DB) UpdateChatParticipants(id, p string) error {
	_, err := db.Exec(`UPDATE chats SET participants=$1,
		participant_ids=(SELECT array_agg(u.id ORDER BY u.username) FROM users u WHERE u.username = ANY(SELECT json_array_elements_text($2::json)))
		WHERE id=$3`, p, p, id)
	return err
}

func (db *DB) DeleteChat(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.Exec(`DELETE FROM messages_v2 WHERE room_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM user_chat_metadata WHERE room_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM muted_chats WHERE room_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM draft_messages WHERE room_id = $1`, id)
	_, err = tx.Exec(`DELETE FROM chats WHERE id = $1`, id)

	if err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) AddContact(user, contact string) error {
	_, err := db.Exec(`INSERT INTO contacts (username, contact_username) VALUES ($1, $2) ON CONFLICT DO NOTHING`, user, contact)
	return err
}

func (db *DB) AddContactByUserID(userID, contactID string) error {
	_, err := db.Exec(`INSERT INTO contacts (user_id, contact_user_id, username, contact_username)
		VALUES ($1::uuid, $2::uuid,
			(SELECT username FROM users WHERE id=$1::uuid),
			(SELECT username FROM users WHERE id=$2::uuid))
		ON CONFLICT DO NOTHING`, userID, contactID)
	return err
}

func (db *DB) RemoveContact(user, contact string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE username=$1 AND contact_username=$2`, user, contact)
	return err
}

func (db *DB) RemoveContactByUserID(userID, contactID string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE (user_id=$1::uuid OR username=$1) AND (contact_user_id=$2::uuid OR contact_username=$2)`, userID, contactID)
	return err
}

func (db *DB) GetContacts(user string) ([]string, error) {
	rows, err := db.Query(`SELECT contact_username FROM contacts WHERE username=$1`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err == nil {
			res = append(res, c)
		}
	}
	return res, nil
}

func (db *DB) GetContactsByUserID(userID string) ([]string, error) {
	rows, err := db.Query(`SELECT COALESCE(u2.username, c.contact_username)
		FROM contacts c LEFT JOIN users u2 ON c.contact_user_id = u2.id
		WHERE c.user_id=$1::uuid OR c.username=(SELECT username FROM users WHERE id=$1::uuid)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err == nil {
			res = append(res, c)
		}
	}
	return res, nil
}

func (db *DB) MarkRead(room, user string) error {
	_, err := db.MarkReadAndCheck(room, user)
	return err
}

func (db *DB) MarkReadAndCheck(room, userID string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE user_chat_metadata SET last_read_at=NOW() WHERE user_id=$1::uuid AND room_id=$2`, userID, room)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		_, err = tx.Exec(`INSERT INTO user_chat_metadata (user_id, room_id, last_read_at) VALUES ($1::uuid, $2, NOW()) ON CONFLICT DO NOTHING`, userID, room)
		if err != nil {
			return false, err
		}
	}

	res, err = tx.Exec(`UPDATE messages SET is_read=TRUE WHERE room_id=$1 AND username!=$2 AND is_read=FALSE`, room, userID)
	if err != nil {
		return false, err
	}

	affected, _ = res.RowsAffected()

	// Also mark messages_v2 as read (only messages from other users)
	res2, err := tx.Exec(`UPDATE messages_v2 SET is_read=TRUE WHERE room_id=$1 AND sender_id!=$2 AND is_read=FALSE`, room, userID)
	if err != nil {
		return false, err
	}
	affected2, _ := res2.RowsAffected()
	if affected2 > 0 {
		affected += affected2
	}

	err = tx.Commit()
	if err == nil && affected > 0 {
		_ = db.IncrementUserChatListVersionByUserID(userID)
	}
	return affected > 0, err
}

func (db *DB) SaveDraftByUserID(uid, room, text, mid, user, rtext string) error {
	q := `INSERT INTO draft_messages (user_id, room_id, draft_text, replied_to_message_id, replied_to_user, replied_to_text, username) VALUES ($1::uuid, $2, $3, $4, $5, $6, (SELECT username FROM users WHERE id=$1::uuid)) ON CONFLICT (username, room_id) DO UPDATE SET draft_text=EXCLUDED.draft_text, replied_to_message_id=EXCLUDED.replied_to_message_id, replied_to_user=EXCLUDED.replied_to_user, replied_to_text=EXCLUDED.replied_to_text`
	_, err := db.Exec(q, uid, room, text, mid, user, rtext)
	return err
}

func (db *DB) SaveDraft(uid, room, text, mid, user, rtext string) error {
	return db.SaveDraftByUserID(uid, room, text, mid, user, rtext)
}

func (db *DB) GetDraftByUserID(uid, room string) (struct {
	DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
	UpdatedAt                                                   time.Time
}, error) {
	var r struct {
		DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
		UpdatedAt                                                   time.Time
	}
	q := `SELECT draft_text, COALESCE(replied_to_message_id,''), COALESCE(replied_to_user,''), COALESCE(replied_to_text,''), updated_at FROM draft_messages WHERE (user_id=$1::uuid OR username=$1::text) AND room_id=$2`
	db.QueryRow(q, uid, room).Scan(&r.DraftText, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.UpdatedAt)
	return r, nil
}

func (db *DB) GetDraft(uid, room string) (struct {
	DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
	UpdatedAt                                                   time.Time
}, error) {
	return db.GetDraftByUserID(uid, room)
}

func (db *DB) DeleteDraftByUserID(uid, room string) (bool, error) {
	res, err := db.Exec(`DELETE FROM draft_messages WHERE (user_id=$1::uuid OR username=$1::text) AND room_id=$2`, uid, room)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (db *DB) DeleteDraft(uid, room string) error {
	_, err := db.DeleteDraftByUserID(uid, room)
	return err
}

func (db *DB) CreateSecretChat(id, name, creator string, participants []string) error {
	participantsJSON, _ := json.Marshal(participants)
	_, err := db.Exec(`INSERT INTO chats (id, name, type, participants, creator_username, is_secret) VALUES ($1, $2, 'secret', $3, $4, TRUE)`, id, name, string(participantsJSON), creator)
	if err == nil {
		_ = db.IncrementParticipantsChatListVersion(id)
	}
	return err
}

func (db *DB) GetSecretChat(chatID string) (struct {
	ID           string
	Name         string
	Type         string
	Participants string
	Creator      string
	IsSecret     bool
	PublicKeyA   string
	PublicKeyB   string
	E2EEReady    bool
	CreatedAt    time.Time
}, error) {
	var c struct {
		ID           string
		Name         string
		Type         string
		Participants string
		Creator      string
		IsSecret     bool
		PublicKeyA   string
		PublicKeyB   string
		E2EEReady    bool
		CreatedAt    time.Time
	}
	err := db.QueryRow(`SELECT id, name, type, participants, COALESCE(creator_username, ''), COALESCE(is_secret, FALSE), COALESCE(public_key_a, ''), COALESCE(public_key_b, ''), COALESCE(e2ee_ready, FALSE), created_at FROM chats WHERE id=$1`, chatID).Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.Creator, &c.IsSecret, &c.PublicKeyA, &c.PublicKeyB, &c.E2EEReady, &c.CreatedAt)
	return c, err
}

func (db *DB) StoreSecretChatKey(chatID, userID, publicKey string) error {
	_, err := db.Exec(`INSERT INTO secret_chat_keys (chat_id, user_id, public_key) VALUES ($1, $2::uuid, $3) ON CONFLICT (chat_id, user_id) DO UPDATE SET public_key = EXCLUDED.public_key, created_at = NOW()`, chatID, userID, publicKey)
	return err
}

func (db *DB) GetSecretChatKey(chatID, userID string) (string, error) {
	var publicKey string
	err := db.QueryRow(`SELECT public_key FROM secret_chat_keys WHERE chat_id=$1 AND user_id=$2::uuid`, chatID, userID).Scan(&publicKey)
	return publicKey, err
}

func (db *DB) GetAllSecretChatKeys(chatID string) (map[string]string, error) {
	rows, err := db.Query(`SELECT user_id::text, public_key FROM secret_chat_keys WHERE chat_id=$1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make(map[string]string)
	for rows.Next() {
		var userID, publicKey string
		if err := rows.Scan(&userID, &publicKey); err == nil {
			keys[userID] = publicKey
		}
	}
	return keys, nil
}

func (db *DB) SetSecretChatE2EEReady(chatID string, ready bool) error {
	_, err := db.Exec(`UPDATE chats SET e2ee_ready=$1 WHERE id=$2`, ready, chatID)
	return err
}

func (db *DB) CreateCall(callerID, receiverID, callType, roomID string) (string, error) {
	var id string
	err := db.QueryRow(`INSERT INTO calls (caller_id, receiver_id, type, room_id, status) VALUES (
		CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END,
		CASE WHEN $2 ~ '^[0-9a-fA-F-]{36}$' THEN $2::uuid ELSE (SELECT id FROM users WHERE username=$2::text) END,
		$3, $4, 'pending') RETURNING id`, callerID, receiverID, callType, roomID).Scan(&id)
	return id, err
}

func (db *DB) UpdateCallStatus(callID, status string) error {
	var query string
	if status == "active" {
		query = `UPDATE calls SET status = $1, started_at = NOW() WHERE id = $2::uuid`
	} else if status == "completed" || status == "rejected" || status == "missed" || status == "busy" {
		query = `UPDATE calls SET status = $1, ended_at = NOW() WHERE id = $2::uuid`
	} else {
		query = `UPDATE calls SET status = $1 WHERE id = $2::uuid`
	}
	_, err := db.Exec(query, status, callID)
	return err
}

func (db *DB) GetCallDuration(callID string) (int, error) {
	var duration float64
	err := db.QueryRow(`SELECT EXTRACT(EPOCH FROM (ended_at - started_at)) FROM calls WHERE id = $1::uuid AND started_at IS NOT NULL AND ended_at IS NOT NULL`, callID).Scan(&duration)
	return int(duration), err
}

func (db *DB) GetCallStatus(callID string) (string, error) {
	var status string
	err := db.QueryRow(`SELECT status FROM calls WHERE id = $1::uuid`, callID).Scan(&status)
	return status, err
}

func (db *DB) GetActiveCallsByUser(userID string) ([]struct {
	CallID     string
	CallerID   string
	ReceiverID string
	RoomID     string
}, error) {
	rows, err := db.Query(`SELECT id, caller_id::text, receiver_id::text, COALESCE(room_id, '') FROM calls WHERE (caller_id = $1::uuid OR receiver_id = $1::uuid) AND status IN ('pending', 'active')`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []struct {
		CallID     string
		CallerID   string
		ReceiverID string
		RoomID     string
	}
	for rows.Next() {
		var c struct {
			CallID     string
			CallerID   string
			ReceiverID string
			RoomID     string
		}
		if err := rows.Scan(&c.CallID, &c.CallerID, &c.ReceiverID, &c.RoomID); err == nil {
			calls = append(calls, c)
		}
	}
	return calls, nil
}

func (db *DB) CreateServer(name, host string, port int, isDefault bool) (string, error) {
	var id string
	err := db.QueryRow(`INSERT INTO servers (name, host, port, is_default) VALUES ($1, $2, $3, $4) RETURNING id`, name, host, port, isDefault).Scan(&id)
	return id, err
}

func (db *DB) GetAllServers() ([]struct {
	ID          string
	Name        string
	Host        string
	Port        int
	IsDefault   bool
	IsProtected bool
	CreatedAt   time.Time
}, error) {
	rows, err := db.Query(`SELECT id, name, host, port, is_default, COALESCE(is_protected, FALSE), created_at FROM servers ORDER BY is_default DESC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID          string
		Name        string
		Host        string
		Port        int
		IsDefault   bool
		IsProtected bool
		CreatedAt   time.Time
	}
	for rows.Next() {
		var s struct {
			ID          string
			Name        string
			Host        string
			Port        int
			IsDefault   bool
			IsProtected bool
			CreatedAt   time.Time
		}
		rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.IsDefault, &s.IsProtected, &s.CreatedAt)
		res = append(res, s)
	}
	return res, nil
}

func (db *DB) GetDefaultServer() (struct {
	ID   string
	Name string
	Host string
	Port int
}, error) {
	var s struct {
		ID   string
		Name string
		Host string
		Port int
	}
	err := db.QueryRow(`SELECT id, name, host, port FROM servers WHERE is_default = TRUE LIMIT 1`).Scan(&s.ID, &s.Name, &s.Host, &s.Port)
	return s, err
}

func (db *DB) UpdateServer(id, name, host string, port int) error {
	_, err := db.Exec(`UPDATE servers SET name=$1, host=$2, port=$3 WHERE id=$4`, name, host, port, id)
	return err
}

func (db *DB) SetDefaultServer(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE servers SET is_default = FALSE`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE servers SET is_default = TRUE WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteServer(id string) error {
	var isDefault, isProtected bool
	err := db.QueryRow(`SELECT is_default, COALESCE(is_protected, FALSE) FROM servers WHERE id = $1`, id).Scan(&isDefault, &isProtected)
	if err != nil {
		return err
	}
	if isDefault {
		return fmt.Errorf("cannot delete default server")
	}
	if isProtected {
		return fmt.Errorf("cannot delete protected server")
	}
	_, err = db.Exec(`DELETE FROM servers WHERE id = $1`, id)
	return err
}
