package main

import (
	"LavenderMessenger/gen"
	"context"
	"strings"
	"time"
)

type stickerServer struct {
	gen.UnimplementedStickerServiceServer
	db *DB
}

func newStickerServer(db *DB) *stickerServer {
	return &stickerServer{db: db}
}

func (s *stickerServer) CreateStickerPack(ctx context.Context, req *gen.CreateStickerPackRequest) (*gen.CreateStickerPackResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.CreateStickerPackResponse{Success: false, Error: "unauthorized"}, nil
	}

	if strings.TrimSpace(req.Title) == "" {
		return &gen.CreateStickerPackResponse{Success: false, Error: "title is required"}, nil
	}

	var packID string
	err := s.db.QueryRow(
		`INSERT INTO sticker_packs (title, name, creator_user_id) VALUES ($1, $2, $3::uuid) RETURNING id`,
		req.Title, req.Name, userID,
	).Scan(&packID)
	if err != nil {
		logger.Errorf("Sticker: CreateStickerPack error: %v", err)
		return &gen.CreateStickerPackResponse{Success: false, Error: "database error"}, nil
	}

	pack := &gen.StickerPack{
		Id:             packID,
		Title:          req.Title,
		Name:           req.Name,
		CreatorUserId:  userID,
		Status:         "draft",
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}

	logger.Infof("Sticker: Created pack %q (id=%s) by user %s", req.Title, packID, userID)

	return &gen.CreateStickerPackResponse{Success: true, Pack: pack}, nil
}

func (s *stickerServer) AddSticker(ctx context.Context, req *gen.AddStickerRequest) (*gen.AddStickerResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.AddStickerResponse{Success: false, Error: "unauthorized"}, nil
	}

	if !s.isPackOwner(req.PackId, userID) {
		return &gen.AddStickerResponse{Success: false, Error: "not pack owner"}, nil
	}

	var stickerID string
	var createdAt time.Time
	err := s.db.QueryRow(
		`INSERT INTO stickers (pack_id, lottie_url, thumbnail_url, emoji, width, height, sort_order)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, (SELECT COALESCE(MAX(sort_order),0)+1 FROM stickers WHERE pack_id=$1::uuid))
		 RETURNING id, created_at`,
		req.PackId, req.LottieUrl, req.ThumbnailUrl, req.Emoji, req.Width, req.Height,
	).Scan(&stickerID, &createdAt)
	if err != nil {
		logger.Errorf("Sticker: AddSticker error: %v", err)
		return &gen.AddStickerResponse{Success: false, Error: "database error"}, nil
	}

	sticker := &gen.Sticker{
		Id:           stickerID,
		PackId:       req.PackId,
		LottieUrl:    req.LottieUrl,
		ThumbnailUrl: req.ThumbnailUrl,
		Emoji:        req.Emoji,
		Width:        req.Width,
		Height:       req.Height,
		CreatedAt:    createdAt.Unix(),
	}

	return &gen.AddStickerResponse{Success: true, Sticker: sticker}, nil
}

func (s *stickerServer) RemoveSticker(ctx context.Context, req *gen.RemoveStickerRequest) (*gen.RemoveStickerResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.RemoveStickerResponse{Success: false}, nil
	}

	if !s.isPackOwner(req.PackId, userID) {
		return &gen.RemoveStickerResponse{Success: false}, nil
	}

	_, err := s.db.Exec(`DELETE FROM stickers WHERE id=$1::uuid AND pack_id=$2::uuid`, req.StickerId, req.PackId)
	if err != nil {
		logger.Errorf("Sticker: RemoveSticker error: %v", err)
		return &gen.RemoveStickerResponse{Success: false}, nil
	}

	return &gen.RemoveStickerResponse{Success: true}, nil
}

func (s *stickerServer) DeleteStickerPack(ctx context.Context, req *gen.DeleteStickerPackRequest) (*gen.DeleteStickerPackResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.DeleteStickerPackResponse{Success: false}, nil
	}

	if !s.isPackOwner(req.PackId, userID) {
		return &gen.DeleteStickerPackResponse{Success: false}, nil
	}

	_, err := s.db.Exec(`DELETE FROM sticker_packs WHERE id=$1::uuid`, req.PackId)
	if err != nil {
		logger.Errorf("Sticker: DeleteStickerPack error: %v", err)
		return &gen.DeleteStickerPackResponse{Success: false}, nil
	}

	return &gen.DeleteStickerPackResponse{Success: true}, nil
}

func (s *stickerServer) GetUserStickerPacks(ctx context.Context, req *gen.GetUserStickerPacksRequest) (*gen.GetUserStickerPacksResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetUserStickerPacksResponse{}, nil
	}

	packs, err := s.getPacksByCreator(userID)
	if err != nil {
		logger.Errorf("Sticker: GetUserStickerPacks error: %v", err)
		return &gen.GetUserStickerPacksResponse{}, nil
	}

	return &gen.GetUserStickerPacksResponse{Packs: packs}, nil
}

func (s *stickerServer) GetPublicStickerPacks(ctx context.Context, req *gen.GetPublicStickerPacksRequest) (*gen.GetPublicStickerPacksResponse, error) {
	_ = GetUserID(ctx)
	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `SELECT id, title, name, creator_user_id, cover_sticker_id, status, rejection_reason, is_featured, created_at, updated_at
		FROM sticker_packs WHERE status='approved'`
	var args []interface{}

	if req.Cursor != "" {
		query += ` AND created_at < (SELECT created_at FROM sticker_packs WHERE id=$1::uuid)`
		args = append(args, req.Cursor)
	}

	query += ` ORDER BY is_featured DESC, created_at DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logger.Errorf("Sticker: GetPublicStickerPacks error: %v", err)
		return &gen.GetPublicStickerPacksResponse{}, nil
	}
	defer rows.Close()

	var packs []*gen.StickerPack
	var lastID string
	for rows.Next() {
		p := &gen.StickerPack{}
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.Id, &p.Title, &p.Name, &p.CreatorUserId, &p.CoverStickerId,
			&p.Status, &p.RejectionReason, &p.IsFeatured, &createdAt, &updatedAt); err != nil {
			continue
		}
		p.CreatedAt = createdAt.Unix()
		p.UpdatedAt = updatedAt.Unix()
		p.Stickers = s.getStickersForPack(p.Id)
		packs = append(packs, p)
		lastID = p.Id
	}

	hasMore := int32(len(packs)) > int32(limit)
	if hasMore {
		packs = packs[:limit]
	}

	var cursor string
	if hasMore {
		cursor = lastID
	}

	return &gen.GetPublicStickerPacksResponse{Packs: packs, NextCursor: cursor, HasMore: hasMore}, nil
}

func (s *stickerServer) GetStickerPack(ctx context.Context, req *gen.GetStickerPackRequest) (*gen.GetStickerPackResponse, error) {
	pack := s.getPackByID(req.PackId)
	if pack == nil {
		return &gen.GetStickerPackResponse{}, nil
	}
	pack.Stickers = s.getStickersForPack(pack.Id)
	return &gen.GetStickerPackResponse{Pack: pack}, nil
}

func (s *stickerServer) SubmitForApproval(ctx context.Context, req *gen.SubmitForApprovalRequest) (*gen.SubmitForApprovalResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.SubmitForApprovalResponse{Success: false, Error: "unauthorized"}, nil
	}

	if !s.isPackOwner(req.PackId, userID) {
		return &gen.SubmitForApprovalResponse{Success: false, Error: "not pack owner"}, nil
	}

	res, err := s.db.Exec(
		`UPDATE sticker_packs SET status='pending', updated_at=NOW() WHERE id=$1::uuid AND status='draft'`,
		req.PackId,
	)
	if err != nil {
		logger.Errorf("Sticker: SubmitForApproval error: %v", err)
		return &gen.SubmitForApprovalResponse{Success: false, Error: "database error"}, nil
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return &gen.SubmitForApprovalResponse{Success: false, Error: "pack not found or not in draft status"}, nil
	}

	return &gen.SubmitForApprovalResponse{Success: true}, nil
}

func (s *stickerServer) ApproveStickerPack(ctx context.Context, req *gen.ApproveStickerPackRequest) (*gen.ApproveStickerPackResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.ApproveStickerPackResponse{Success: false}, nil
	}

	if !s.isSuperAdmin(userID) {
		return &gen.ApproveStickerPackResponse{Success: false}, nil
	}

	status := "rejected"
	if req.Approved {
		status = "approved"
	}

	_, err := s.db.Exec(
		`UPDATE sticker_packs SET status=$1, rejection_reason=$2, updated_at=NOW() WHERE id=$3::uuid AND status='pending'`,
		status, req.Reason, req.PackId,
	)
	if err != nil {
		logger.Errorf("Sticker: ApproveStickerPack error: %v", err)
		return &gen.ApproveStickerPackResponse{Success: false}, nil
	}

	return &gen.ApproveStickerPackResponse{Success: true}, nil
}

func (s *stickerServer) GetPendingStickerPacks(ctx context.Context, req *gen.GetPendingStickerPacksRequest) (*gen.GetPendingStickerPacksResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" || !s.isSuperAdmin(userID) {
		return &gen.GetPendingStickerPacksResponse{}, nil
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `SELECT id, title, name, creator_user_id, cover_sticker_id, status, rejection_reason, is_featured, created_at, updated_at
		FROM sticker_packs WHERE status='pending'`
	var args []interface{}

	if req.Cursor != "" {
		query += ` AND created_at < (SELECT created_at FROM sticker_packs WHERE id=$1::uuid)`
		args = append(args, req.Cursor)
	}

	query += ` ORDER BY created_at DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logger.Errorf("Sticker: GetPendingStickerPacks error: %v", err)
		return &gen.GetPendingStickerPacksResponse{}, nil
	}
	defer rows.Close()

	var packs []*gen.StickerPack
	var lastID string
	for rows.Next() {
		p := &gen.StickerPack{}
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.Id, &p.Title, &p.Name, &p.CreatorUserId, &p.CoverStickerId,
			&p.Status, &p.RejectionReason, &p.IsFeatured, &createdAt, &updatedAt); err != nil {
			continue
		}
		p.CreatedAt = createdAt.Unix()
		p.UpdatedAt = updatedAt.Unix()
		p.Stickers = s.getStickersForPack(p.Id)
		packs = append(packs, p)
		lastID = p.Id
	}

	hasMore := int32(len(packs)) > int32(limit)
	if hasMore {
		packs = packs[:limit]
	}

	var cursor string
	if hasMore {
		cursor = lastID
	}

	return &gen.GetPendingStickerPacksResponse{Packs: packs, NextCursor: cursor, HasMore: hasMore}, nil
}

func (s *stickerServer) SearchStickerPacks(ctx context.Context, req *gen.SearchStickerPacksRequest) (*gen.SearchStickerPacksResponse, error) {
	_ = GetUserID(ctx)
	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `SELECT id, title, name, creator_user_id, cover_sticker_id, status, rejection_reason, is_featured, created_at, updated_at
		FROM sticker_packs WHERE status='approved' AND title ILIKE $1
		ORDER BY created_at DESC LIMIT $2`

	rows, err := s.db.Query(query, "%"+req.Query+"%", limit)
	if err != nil {
		logger.Errorf("Sticker: SearchStickerPacks error: %v", err)
		return &gen.SearchStickerPacksResponse{}, nil
	}
	defer rows.Close()

	var packs []*gen.StickerPack
	for rows.Next() {
		p := &gen.StickerPack{}
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.Id, &p.Title, &p.Name, &p.CreatorUserId, &p.CoverStickerId,
			&p.Status, &p.RejectionReason, &p.IsFeatured, &createdAt, &updatedAt); err != nil {
			continue
		}
		p.CreatedAt = createdAt.Unix()
		p.UpdatedAt = updatedAt.Unix()
		p.Stickers = s.getStickersForPack(p.Id)
		packs = append(packs, p)
	}

	return &gen.SearchStickerPacksResponse{Packs: packs}, nil
}

func (s *stickerServer) UpdateStickerPack(ctx context.Context, req *gen.UpdateStickerPackRequest) (*gen.UpdateStickerPackResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateStickerPackResponse{Success: false}, nil
	}

	if !s.isPackOwner(req.PackId, userID) {
		return &gen.UpdateStickerPackResponse{Success: false}, nil
	}

	_, err := s.db.Exec(
		`UPDATE sticker_packs SET title=COALESCE(NULLIF($1,''), title), cover_sticker_id=COALESCE(NULLIF($2,''), cover_sticker_id), updated_at=NOW() WHERE id=$3::uuid`,
		req.Title, req.CoverStickerId, req.PackId,
	)
	if err != nil {
		logger.Errorf("Sticker: UpdateStickerPack error: %v", err)
		return &gen.UpdateStickerPackResponse{Success: false}, nil
	}

	return &gen.UpdateStickerPackResponse{Success: true}, nil
}

func (s *stickerServer) SetFeaturedStickerPack(ctx context.Context, req *gen.SetFeaturedStickerPackRequest) (*gen.SetFeaturedStickerPackResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" || !s.isSuperAdmin(userID) {
		return &gen.SetFeaturedStickerPackResponse{Success: false}, nil
	}

	_, err := s.db.Exec(
		`UPDATE sticker_packs SET is_featured=$1, updated_at=NOW() WHERE id=$2::uuid`,
		req.Featured, req.PackId,
	)
	if err != nil {
		logger.Errorf("Sticker: SetFeaturedStickerPack error: %v", err)
		return &gen.SetFeaturedStickerPackResponse{Success: false}, nil
	}

	return &gen.SetFeaturedStickerPackResponse{Success: true}, nil
}

// --- helpers ---

func (s *stickerServer) isPackOwner(packID, userID string) bool {
	var ownerID string
	err := s.db.QueryRow(`SELECT creator_user_id FROM sticker_packs WHERE id=$1::uuid`, packID).Scan(&ownerID)
	if err != nil {
		return false
	}
	return ownerID == userID
}

func (s *stickerServer) isSuperAdmin(userID string) bool {
	var isSuper bool
	err := s.db.QueryRow(`SELECT COALESCE(is_super_admin, FALSE) FROM users WHERE id=$1::uuid`, userID).Scan(&isSuper)
	if err != nil {
		return false
	}
	return isSuper
}

func (s *stickerServer) getPackByID(packID string) *gen.StickerPack {
	p := &gen.StickerPack{}
	var createdAt, updatedAt time.Time
	err := s.db.QueryRow(
		`SELECT id, title, name, creator_user_id, cover_sticker_id, status, rejection_reason, is_featured, created_at, updated_at
		 FROM sticker_packs WHERE id=$1::uuid`, packID,
	).Scan(&p.Id, &p.Title, &p.Name, &p.CreatorUserId, &p.CoverStickerId,
		&p.Status, &p.RejectionReason, &p.IsFeatured, &createdAt, &updatedAt)
	if err != nil {
		return nil
	}
	p.CreatedAt = createdAt.Unix()
	p.UpdatedAt = updatedAt.Unix()

	// creator username
	var username string
	_ = s.db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, p.CreatorUserId).Scan(&username)
	p.CreatorUsername = username

	return p
}

func (s *stickerServer) getPacksByCreator(userID string) ([]*gen.StickerPack, error) {
	rows, err := s.db.Query(
		`SELECT id, title, name, creator_user_id, cover_sticker_id, status, rejection_reason, is_featured, created_at, updated_at
		 FROM sticker_packs WHERE creator_user_id=$1::uuid ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []*gen.StickerPack
	for rows.Next() {
		p := &gen.StickerPack{}
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.Id, &p.Title, &p.Name, &p.CreatorUserId, &p.CoverStickerId,
			&p.Status, &p.RejectionReason, &p.IsFeatured, &createdAt, &updatedAt); err != nil {
			continue
		}
		p.CreatedAt = createdAt.Unix()
		p.UpdatedAt = updatedAt.Unix()
		p.Stickers = s.getStickersForPack(p.Id)
		packs = append(packs, p)
	}

	return packs, nil
}

func (s *stickerServer) getStickersForPack(packID string) []*gen.Sticker {
	rows, err := s.db.Query(
		`SELECT id, pack_id, lottie_url, thumbnail_url, emoji, width, height, created_at
		 FROM stickers WHERE pack_id=$1::uuid ORDER BY sort_order ASC`, packID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var stickers []*gen.Sticker
	for rows.Next() {
		st := &gen.Sticker{}
		var createdAt time.Time
		if err := rows.Scan(&st.Id, &st.PackId, &st.LottieUrl, &st.ThumbnailUrl, &st.Emoji, &st.Width, &st.Height, &createdAt); err != nil {
			continue
		}
		st.CreatedAt = createdAt.Unix()
		stickers = append(stickers, st)
	}

	return stickers
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
