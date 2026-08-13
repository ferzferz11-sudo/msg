package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"math/rand"
	"time"
)

const inviteCodeChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateInviteCodeString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = inviteCodeChars[rand.Intn(len(inviteCodeChars))]
	}
	return string(b)
}

func (c *companyServer) GenerateInviteCode(ctx context.Context, req *gen.GenerateInviteCodeRequest) (*gen.GenerateInviteCodeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GenerateInviteCodeResponse{Success: false, Error: "unauthorized"}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.GenerateInviteCodeResponse{Success: false, Error: "not authorized"}, nil
	}

	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}

	var expiresAt sql.NullTime
	if req.ExpiresHours > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresHours) * time.Hour)
		expiresAt = sql.NullTime{Time: t, Valid: true}
	}

	// Generate unique code with retry
	var codeStr string
	var codeID string
	for attempt := 0; attempt < 10; attempt++ {
		codeStr = generateInviteCodeString(8)
		err := c.db.QueryRow(
			`INSERT INTO company_invite_codes (company_id, code, created_by, expires_at, max_uses)
			 VALUES ($1::uuid, $2, $3::uuid, $4, $5)
			 RETURNING id`,
			req.CompanyId, codeStr, userID, expiresAt, maxUses,
		).Scan(&codeID)
		if err == nil {
			break
		}
		if attempt == 9 {
			logger.Errorf("Company: GenerateInviteCode failed after 10 attempts: %v", err)
			return &gen.GenerateInviteCodeResponse{Success: false, Error: "failed to generate unique code"}, nil
		}
	}

	var createdAt time.Time
	_ = c.db.QueryRow(`SELECT created_at FROM company_invite_codes WHERE id=$1::uuid`, codeID).Scan(&createdAt)

	info := &gen.InviteCodeInfo{
		Id:        codeID,
		Code:      codeStr,
		CompanyId: req.CompanyId,
		CreatedBy: userID,
		CreatedAt: createdAt.Format(time.RFC3339),
		MaxUses:   maxUses,
		UseCount:  0,
		IsActive:  true,
	}
	if expiresAt.Valid {
		info.ExpiresAt = expiresAt.Time.Format(time.RFC3339)
	}

	logger.Infof("Company: Invite code %s generated for company %s by %s", codeStr, req.CompanyId, userID)

	return &gen.GenerateInviteCodeResponse{Success: true, Code: info}, nil
}

func (c *companyServer) JoinByInviteCode(ctx context.Context, req *gen.JoinByInviteCodeRequest) (*gen.JoinByInviteCodeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "unauthorized"}, nil
	}

	if req.Code == "" {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "code required"}, nil
	}

	// Get invite code with lock
	var companyID string
	var expiresAt sql.NullTime
	var maxUses int
	var useCount int
	var isActive bool

	err := c.db.QueryRow(`
		SELECT company_id, expires_at, max_uses, use_count, is_active
		FROM company_invite_codes WHERE code=$1
		FOR UPDATE`, req.Code,
	).Scan(&companyID, &expiresAt, &maxUses, &useCount, &isActive)
	if err != nil {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "invalid code"}, nil
	}

	if !isActive {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "code is revoked"}, nil
	}

	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "code has expired"}, nil
	}

	if useCount >= maxUses {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "code usage limit reached"}, nil
	}

	// Check if already a member
	var alreadyMember bool
	_ = c.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid)`,
		companyID, userID).Scan(&alreadyMember)
	if alreadyMember {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "already a member"}, nil
	}

	// Get default position (level 0 or first available)
	var positionID string
	// Try company_settings default_position_id first
	_ = c.db.QueryRow(`SELECT default_position_id::text FROM company_settings WHERE company_id=$1::uuid AND default_position_id IS NOT NULL`, companyID).Scan(&positionID)
	if positionID == "" {
		// Fallback: lowest level position
		_ = c.db.QueryRow(`SELECT id::text FROM company_positions WHERE company_id=$1::uuid ORDER BY level ASC LIMIT 1`, companyID).Scan(&positionID)
	}
	if positionID == "" {
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "no positions available in this company"}, nil
	}

	// Add member
	_, err = c.db.Exec(`INSERT INTO company_members (company_id, user_id, position_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		companyID, userID, positionID)
	if err != nil {
		logger.Errorf("Company: JoinByInviteCode insert error: %v", err)
		return &gen.JoinByInviteCodeResponse{Success: false, Error: "failed to join"}, nil
	}

	// Increment use_count
	_, _ = c.db.Exec(`UPDATE company_invite_codes SET use_count = use_count + 1 WHERE code=$1`, req.Code)

	logger.Infof("Company: User %s joined company %s via invite code %s", userID, companyID, req.Code)

	return &gen.JoinByInviteCodeResponse{Success: true, CompanyId: companyID}, nil
}

func (c *companyServer) RevokeInviteCode(ctx context.Context, req *gen.RevokeInviteCodeRequest) (*gen.RevokeInviteCodeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.RevokeInviteCodeResponse{Success: false}, nil
	}

	// Verify ownership
	var companyID string
	err := c.db.QueryRow(`SELECT company_id FROM company_invite_codes WHERE id=$1::uuid`, req.CodeId).Scan(&companyID)
	if err != nil {
		return &gen.RevokeInviteCodeResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(companyID, userID) {
		return &gen.RevokeInviteCodeResponse{Success: false}, nil
	}

	_, err = c.db.Exec(`UPDATE company_invite_codes SET is_active = FALSE WHERE id=$1::uuid`, req.CodeId)
	if err != nil {
		return &gen.RevokeInviteCodeResponse{Success: false}, nil
	}

	return &gen.RevokeInviteCodeResponse{Success: true}, nil
}

func (c *companyServer) ListInviteCodes(ctx context.Context, req *gen.ListInviteCodesRequest) (*gen.ListInviteCodesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.ListInviteCodesResponse{}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.ListInviteCodesResponse{}, nil
	}

	rows, err := c.db.Query(`
		SELECT id, code, company_id, created_by, created_at, expires_at, max_uses, use_count, is_active
		FROM company_invite_codes WHERE company_id=$1::uuid
		ORDER BY created_at DESC`, req.CompanyId)
	if err != nil {
		return &gen.ListInviteCodesResponse{}, nil
	}
	defer rows.Close()

	var codes []*gen.InviteCodeInfo
	for rows.Next() {
		info := &gen.InviteCodeInfo{}
		var createdAt time.Time
		var expiresAt sql.NullTime
		if err := rows.Scan(&info.Id, &info.Code, &info.CompanyId, &info.CreatedBy, &createdAt,
			&expiresAt, &info.MaxUses, &info.UseCount, &info.IsActive); err != nil {
			continue
		}
		info.CreatedAt = createdAt.Format(time.RFC3339)
		if expiresAt.Valid {
			info.ExpiresAt = expiresAt.Time.Format(time.RFC3339)
		}
		codes = append(codes, info)
	}

	return &gen.ListInviteCodesResponse{Codes: codes}, nil
}
