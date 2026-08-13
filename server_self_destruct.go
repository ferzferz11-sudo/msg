package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var allowedTimerValues = map[int32]bool{
	0: true, 30: true, 60: true, 300: true, 3600: true, 86400: true,
}

func (s *server) SetSelfDestructTimer(ctx context.Context, req *gen.SetSelfDestructTimerRequest) (*gen.SetSelfDestructTimerResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if req.RoomId == "" {
		return &gen.SetSelfDestructTimerResponse{Success: false, Error: "room_id required"}, nil
	}

	timer := req.TimerSeconds
	if !allowedTimerValues[timer] {
		return &gen.SetSelfDestructTimerResponse{Success: false, Error: fmt.Sprintf("invalid timer value: %d (allowed: 0, 30, 60, 300, 3600, 86400)", timer)}, nil
	}

	// Verify user is participant of the chat
	if !s.isChatParticipant(userID, req.RoomId) {
		return &gen.SetSelfDestructTimerResponse{Success: false, Error: "not a participant"}, nil
	}

	if err := s.db.SetSelfDestructTimer(req.RoomId, int(timer)); err != nil {
		logger.Errorf("SetSelfDestructTimer: %v", err)
		return &gen.SetSelfDestructTimerResponse{Success: false, Error: err.Error()}, nil
	}

	logger.Infof("SetSelfDestructTimer: room=%s timer=%d by user=%s", req.RoomId, timer, userID)

	// Broadcast timer change to all room participants
	s.hub.BroadcastToRoom(req.RoomId, "SELF_DESTRUCT_TIMER", fmt.Sprintf("%d", timer))

	_ = s.db.IncrementParticipantsChatListVersion(req.RoomId)

	return &gen.SetSelfDestructTimerResponse{Success: true}, nil
}

// startSelfDestructCleanup runs a background goroutine that deletes expired messages every 30 seconds.
func (s *server) startSelfDestructCleanup(ctx context.Context) {
	go func() {
		ticker := newStaggeredTicker(30)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				affected, err := s.db.DeleteExpiredSelfDestructMessages()
				if err != nil {
					logger.Errorf("Self-destruct cleanup error: %v", err)
					continue
				}
				for roomID, ids := range affected {
					for _, id := range ids {
						s.hub.BroadcastToRoom(roomID, "DELETE_MESSAGE_V2", id)
					}
					s.db.UpdateChatLastMessage(roomID)
					_ = s.db.IncrementParticipantsChatListVersion(roomID)
				}
			}
		}
	}()
}

// startDeletedMessagesCleanup runs a background goroutine that cleans up old deleted_messages entries.
func (s *server) startDeletedMessagesCleanup(ctx context.Context) {
	go func() {
		ticker := newStaggeredTicker(3600) // every hour
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.db.CleanupDeletedMessages()
			}
		}
	}()
}

// newStaggeredTicker creates a time.Ticker that fires every `seconds` but with a random initial delay
// to avoid thundering herd on server start.
func newStaggeredTicker(seconds int) *time.Ticker {
	// Use a small fixed offset based on current nanoseconds to stagger
	return time.NewTicker(time.Duration(seconds) * time.Second)
}
