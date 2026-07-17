package main

import (
	"LavenderMessenger/gen"
	"context"
	"time"

	"github.com/google/uuid"
)

func (s *server) GetFCMLogs(_ context.Context, _ *gen.GetFCMLogsRequest) (*gen.GetFCMLogsResponse, error) {
	s.fcmLogsMu.Lock()
	defer s.fcmLogsMu.Unlock()

	// Return a copy to avoid concurrent issues
	logs := make([]*gen.FCMLogEntry, len(s.fcmLogs))
	for i, l := range s.fcmLogs {
		logs[i] = &gen.FCMLogEntry{
			Timestamp: l.Timestamp,
			Level:     l.Level,
			Message:   l.Message,
		}
	}
	return &gen.GetFCMLogsResponse{Logs: logs}, nil
}

func (s *server) SaveDraft(ctx context.Context, req *gen.SaveDraftRequest) (*gen.SaveDraftResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.SaveDraftResponse{Success: false, Message: "empty user id"}, nil
	}

	var err error
	if _, uuidErr := uuid.Parse(userID); uuidErr == nil {
		err = s.db.SaveDraftByUserID(userID, req.RoomId, req.DraftText, req.RepliedToMessageId, req.RepliedToUser, req.RepliedToText)
	} else {
		err = s.db.SaveDraft(userID, req.RoomId, req.DraftText, req.RepliedToMessageId, req.RepliedToUser, req.RepliedToText)
	}

	if err != nil {
		s.logErrorOnce("SaveDraft:"+userID, "Failed to save draft for user %s in room %s: %v", userID, req.RoomId, err)
		return &gen.SaveDraftResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Debugf("Draft saved for user %s in room %s (length: %d)", userID, req.RoomId, len(req.DraftText))
	return &gen.SaveDraftResponse{Success: true, Message: "Draft saved successfully"}, nil
}

func (s *server) GetDraft(ctx context.Context, req *gen.GetDraftRequest) (*gen.GetDraftResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.GetDraftResponse{HasDraft: false}, nil
	}

	var draft struct {
		DraftText          string
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		UpdatedAt          time.Time
	}
	var err error

	if _, uuidErr := uuid.Parse(userID); uuidErr == nil {
		draft, err = s.db.GetDraftByUserID(userID, req.RoomId)
	} else {
		draft, err = s.db.GetDraft(userID, req.RoomId)
	}

	if err != nil {
		s.logErrorOnce("GetDraft:"+userID, "Failed to get draft for user %s in room %s: %v", userID, req.RoomId, err)
		return &gen.GetDraftResponse{HasDraft: false}, nil
	}

	hasDraft := draft.DraftText != "" || draft.RepliedToMessageID != ""

	return &gen.GetDraftResponse{
		DraftText:          draft.DraftText,
		RepliedToMessageId: draft.RepliedToMessageID,
		RepliedToUser:      draft.RepliedToUser,
		RepliedToText:      draft.RepliedToText,
		HasDraft:           hasDraft,
	}, nil
}

func (s *server) DeleteDraft(ctx context.Context, req *gen.DeleteDraftRequest) (*gen.DeleteDraftResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}
	if userID == "" {
		return &gen.DeleteDraftResponse{Success: false}, nil
	}

	var deleted bool
	var err error
	if _, uuidErr := uuid.Parse(userID); uuidErr == nil {
		deleted, err = s.db.DeleteDraftByUserID(userID, req.RoomId)
	} else {
		err = s.db.DeleteDraft(userID, req.RoomId)
		deleted = err == nil
	}

	if err != nil {
		s.logErrorOnce("DeleteDraft:"+userID, "Failed to delete draft for user %s in room %s: %v", userID, req.RoomId, err)
		return &gen.DeleteDraftResponse{Success: false}, nil
	}
	// Only log if we actually deleted something (not for empty/duplicate deletions)
	if deleted {
		logger.Debugf("Draft deleted for user %s in room %s", userID, req.RoomId)
	}
	return &gen.DeleteDraftResponse{Success: true}, nil
}
