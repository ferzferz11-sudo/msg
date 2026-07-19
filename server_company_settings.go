package main

import (
	"LavenderMessenger/gen"
	"context"
)

func (c *companyServer) GetCompanySettings(ctx context.Context, req *gen.GetCompanySettingsRequest) (*gen.GetCompanySettingsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetCompanySettingsResponse{}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.GetCompanySettingsResponse{}, nil
	}

	s := &gen.CompanySettings{
		CompanyId:     req.CompanyId,
		ChatAccess:    "member",
	}

	err := c.db.QueryRow(
		`SELECT COALESCE(invite_only, FALSE), COALESCE(default_position_id::text, ''),
		        COALESCE(allow_member_invite, FALSE), COALESCE(chat_access, 'member'),
		        COALESCE(require_approval, FALSE)
		 FROM company_settings WHERE company_id=$1::uuid`, req.CompanyId,
	).Scan(&s.InviteOnly, &s.DefaultPositionId, &s.AllowMemberInvite, &s.ChatAccess, &s.RequireApproval)
	if err != nil {
		// Return defaults if no settings row exists
		return &gen.GetCompanySettingsResponse{Settings: s}, nil
	}

	return &gen.GetCompanySettingsResponse{Settings: s}, nil
}

func (c *companyServer) UpdateCompanySettings(ctx context.Context, req *gen.UpdateCompanySettingsRequest) (*gen.UpdateCompanySettingsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateCompanySettingsResponse{Success: false, Error: "unauthorized"}, nil
	}

	if !c.isOwner(req.CompanyId, userID) {
		return &gen.UpdateCompanySettingsResponse{Success: false, Error: "only owner can update settings"}, nil
	}

	s := req.Settings
	if s == nil {
		return &gen.UpdateCompanySettingsResponse{Success: false, Error: "settings required"}, nil
	}

	// Validate default_position_id belongs to this company
	if s.DefaultPositionId != "" {
		var exists bool
		_ = c.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_positions WHERE id=$1::uuid AND company_id=$2::uuid)`,
			s.DefaultPositionId, req.CompanyId).Scan(&exists)
		if !exists {
			return &gen.UpdateCompanySettingsResponse{Success: false, Error: "default_position_id does not belong to this company"}, nil
		}
	}

	// Validate chat_access
	switch s.ChatAccess {
	case "member", "management", "owner_only":
	default:
		s.ChatAccess = "member"
	}

	_, err := c.db.Exec(`
		INSERT INTO company_settings (company_id, invite_only, default_position_id, allow_member_invite, chat_access, require_approval, updated_at)
		VALUES ($1::uuid, $2, NULLIF($3,'')::uuid, $4, $5, $6, NOW())
		ON CONFLICT (company_id) DO UPDATE SET
			invite_only = EXCLUDED.invite_only,
			default_position_id = EXCLUDED.default_position_id,
			allow_member_invite = EXCLUDED.allow_member_invite,
			chat_access = EXCLUDED.chat_access,
			require_approval = EXCLUDED.require_approval,
			updated_at = NOW()`,
		req.CompanyId, s.InviteOnly, s.DefaultPositionId, s.AllowMemberInvite, s.ChatAccess, s.RequireApproval,
	)
	if err != nil {
		logger.Errorf("Company: UpdateCompanySettings error: %v", err)
		return &gen.UpdateCompanySettingsResponse{Success: false, Error: "database error"}, nil
	}

	logger.Infof("Company: Settings updated for %s by user %s", req.CompanyId, userID)

	return &gen.UpdateCompanySettingsResponse{Success: true}, nil
}
