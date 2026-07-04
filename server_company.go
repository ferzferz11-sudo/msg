package main

import (
	"LavenderMessenger/gen"
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// companyServer implements CompanyService.
type companyServer struct {
	gen.UnimplementedCompanyServiceServer
	db *DB
}

func newCompanyServer(db *DB) *companyServer {
	return &companyServer{db: db}
}

func (c *companyServer) CreateCompany(ctx context.Context, req *gen.CreateCompanyRequest) (*gen.CreateCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.CreateCompanyResponse{Success: false}, nil
	}

	var id string
	err := c.db.QueryRow(
		`INSERT INTO companies (name, owner_id) VALUES ($1, $2::uuid) RETURNING id`,
		req.Name, userID,
	).Scan(&id)
	if err != nil {
		logger.Errorf("Company: CreateCompany error: %v", err)
		return &gen.CreateCompanyResponse{Success: false}, nil
	}

	// Auto-create default positions
	defaultPositions := []struct {
		title    string
		level    int
		chatAccess string
	}{
		{"Owner", 3, "owner_only"},
		{"Top Manager", 2, "management"},
		{"Manager", 1, "management"},
		{"Employee", 0, "member"},
	}
	for _, p := range defaultPositions {
		_, _ = c.db.Exec(
			`INSERT INTO company_positions (company_id, title, level, chat_access) VALUES ($1::uuid, $2, $3, $4)`,
			id, p.title, p.level, p.chatAccess,
		)
	}

	// Add creator as Owner
	var posID string
	_ = c.db.QueryRow(`SELECT id FROM company_positions WHERE company_id=$1::uuid AND level=3`, id).Scan(&posID)
	if posID != "" {
		_, _ = c.db.Exec(
			`INSERT INTO company_members (company_id, user_id, position_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
			id, userID, posID,
		)
	}

	logger.Infof("Company: Created company %q (id=%s) by user %s", req.Name, id, userID)

	return &gen.CreateCompanyResponse{
		Success: true,
		Company: &gen.Company{
			Id:        id,
			Name:      req.Name,
			OwnerId:   userID,
			CreatedAt: time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (c *companyServer) GetCompany(ctx context.Context, req *gen.GetCompanyRequest) (*gen.GetCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetCompanyResponse{}, nil
	}

	company, err := c.getCompanyByID(req.CompanyId)
	if err != nil {
		return &gen.GetCompanyResponse{}, nil
	}

	positions, _ := c.getPositions(req.CompanyId)
	var memberCount int
	_ = c.db.QueryRow(`SELECT COUNT(*) FROM company_members WHERE company_id=$1::uuid`, req.CompanyId).Scan(&memberCount)

	return &gen.GetCompanyResponse{
		Company:     company,
		Positions:   positions,
		MemberCount: int32(memberCount),
	}, nil
}

func (c *companyServer) UpdateCompany(ctx context.Context, req *gen.UpdateCompanyRequest) (*gen.UpdateCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateCompanyResponse{Success: false}, nil
	}

	if !c.isOwner(req.CompanyId, userID) {
		return &gen.UpdateCompanyResponse{Success: false}, nil
	}

	_, err := c.db.Exec(
		`UPDATE companies SET name=COALESCE(NULLIF($1,''), name), avatar_url=COALESCE(NULLIF($2,''), avatar_url) WHERE id=$3::uuid`,
		req.Name, req.AvatarUrl, req.CompanyId,
	)
	if err != nil {
		logger.Errorf("Company: UpdateCompany error: %v", err)
		return &gen.UpdateCompanyResponse{Success: false}, nil
	}

	company, _ := c.getCompanyByID(req.CompanyId)
	return &gen.UpdateCompanyResponse{Success: true, Company: company}, nil
}

func (c *companyServer) DeleteCompany(ctx context.Context, req *gen.DeleteCompanyRequest) (*gen.DeleteCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.DeleteCompanyResponse{Success: false}, nil
	}

	if !c.isOwner(req.CompanyId, userID) {
		return &gen.DeleteCompanyResponse{Success: false, Message: "only owner can delete company"}, nil
	}

	_, err := c.db.Exec(`DELETE FROM companies WHERE id=$1::uuid`, req.CompanyId)
	if err != nil {
		logger.Errorf("Company: DeleteCompany error: %v", err)
		return &gen.DeleteCompanyResponse{Success: false}, nil
	}

	return &gen.DeleteCompanyResponse{Success: true, Message: "Company deleted"}, nil
}

func (c *companyServer) ListCompanies(ctx context.Context, _ *gen.ListCompaniesRequest) (*gen.ListCompaniesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.ListCompaniesResponse{}, nil
	}

	rows, err := c.db.Query(`
		SELECT c.id, c.name, c.owner_id, COALESCE(c.avatar_url,''), c.created_at,
		       (SELECT COUNT(*) FROM company_members cm WHERE cm.company_id=c.id)
		FROM companies c
		JOIN company_members cm ON cm.company_id=c.id
		WHERE cm.user_id=$1::uuid
		ORDER BY c.name`, userID)
	if err != nil {
		return &gen.ListCompaniesResponse{}, nil
	}
	defer rows.Close()

	var companies []*gen.Company
	for rows.Next() {
		var comp gen.Company
		var createdAt time.Time
		if err := rows.Scan(&comp.Id, &comp.Name, &comp.OwnerId, &comp.AvatarUrl, &createdAt, &comp.MemberCount); err == nil {
			comp.CreatedAt = createdAt.Format(time.RFC3339)
			companies = append(companies, &comp)
		}
	}

	return &gen.ListCompaniesResponse{Companies: companies}, nil
}

// --- Positions ---

func (c *companyServer) CreatePosition(ctx context.Context, req *gen.CreatePositionRequest) (*gen.CreatePositionResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.CreatePositionResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.CreatePositionResponse{Success: false}, nil
	}

	chatAccess := req.ChatAccess
	if chatAccess == "" {
		chatAccess = "member"
	}

	var id string
	err := c.db.QueryRow(
		`INSERT INTO company_positions (company_id, title, level, chat_access) VALUES ($1::uuid, $2, $3, $4) RETURNING id`,
		req.CompanyId, req.Title, req.Level, chatAccess,
	).Scan(&id)
	if err != nil {
		logger.Errorf("Company: CreatePosition error: %v", err)
		return &gen.CreatePositionResponse{Success: false}, nil
	}

	return &gen.CreatePositionResponse{
		Success: true,
		Position: &gen.CompanyPosition{
			Id:          id,
			CompanyId:   req.CompanyId,
			Title:       req.Title,
			Level:       req.Level,
			ChatAccess:  chatAccess,
		},
	}, nil
}

func (c *companyServer) UpdatePosition(ctx context.Context, req *gen.UpdatePositionRequest) (*gen.UpdatePositionResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdatePositionResponse{Success: false}, nil
	}

	var companyID string
	_ = c.db.QueryRow(`SELECT company_id FROM company_positions WHERE id=$1::uuid`, req.PositionId).Scan(&companyID)
	if companyID == "" || !c.isOwnerOrManager(companyID, userID) {
		return &gen.UpdatePositionResponse{Success: false}, nil
	}

	_, err := c.db.Exec(
		`UPDATE company_positions SET title=COALESCE(NULLIF($1,''), title), level=COALESCE(NULLIF($2,0), level), chat_access=COALESCE(NULLIF($3,''), chat_access) WHERE id=$4::uuid`,
		req.Title, req.Level, req.ChatAccess, req.PositionId,
	)
	if err != nil {
		return &gen.UpdatePositionResponse{Success: false}, nil
	}

	var pos gen.CompanyPosition
	_ = c.db.QueryRow(`SELECT id, company_id, title, level, chat_access FROM company_positions WHERE id=$1::uuid`, req.PositionId).
		Scan(&pos.Id, &pos.CompanyId, &pos.Title, &pos.Level, &pos.ChatAccess)

	return &gen.UpdatePositionResponse{Success: true, Position: &pos}, nil
}

func (c *companyServer) DeletePosition(ctx context.Context, req *gen.DeletePositionRequest) (*gen.DeletePositionResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.DeletePositionResponse{Success: false}, nil
	}

	var companyID, title string
	var level int
	_ = c.db.QueryRow(`SELECT company_id, title, level FROM company_positions WHERE id=$1::uuid`, req.PositionId).Scan(&companyID, &title, &level)
	if companyID == "" || !c.isOwnerOrManager(companyID, userID) {
		return &gen.DeletePositionResponse{Success: false}, nil
	}

	// Don't delete built-in positions (level 0-3)
	if level >= 0 && level <= 3 && (title == "Owner" || title == "Top Manager" || title == "Manager" || title == "Employee") {
		return &gen.DeletePositionResponse{Success: false, Message: "cannot delete built-in position"}, nil
	}

	// Check if anyone has this position
	var count int
	_ = c.db.QueryRow(`SELECT COUNT(*) FROM company_members WHERE position_id=$1::uuid`, req.PositionId).Scan(&count)
	if count > 0 {
		return &gen.DeletePositionResponse{Success: false, Message: "position has members, reassign them first"}, nil
	}

	_, err := c.db.Exec(`DELETE FROM company_positions WHERE id=$1::uuid`, req.PositionId)
	if err != nil {
		return &gen.DeletePositionResponse{Success: false}, nil
	}

	return &gen.DeletePositionResponse{Success: true, Message: "Position deleted"}, nil
}

func (c *companyServer) ListPositions(ctx context.Context, req *gen.ListPositionsRequest) (*gen.ListPositionsResponse, error) {
	positions, err := c.getPositions(req.CompanyId)
	if err != nil {
		return &gen.ListPositionsResponse{}, nil
	}
	return &gen.ListPositionsResponse{Positions: positions}, nil
}

// --- Members ---

func (c *companyServer) AddMember(ctx context.Context, req *gen.AddMemberRequest) (*gen.AddMemberResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.AddMemberResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.AddMemberResponse{Success: false}, nil
	}

	// Check if user already a member
	var exists bool
	_ = c.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid)`, req.CompanyId, req.UserId).Scan(&exists)
	if exists {
		return &gen.AddMemberResponse{Success: false}, nil
	}

	var id string
	err := c.db.QueryRow(
		`INSERT INTO company_members (company_id, user_id, position_id) VALUES ($1::uuid, $2::uuid, $3::uuid) RETURNING id`,
		req.CompanyId, req.UserId, req.PositionId,
	).Scan(&id)
	if err != nil {
		logger.Errorf("Company: AddMember error: %v", err)
		return &gen.AddMemberResponse{Success: false}, nil
	}

	member, _ := c.getMemberByID(id)
	return &gen.AddMemberResponse{Success: true, Member: member}, nil
}

func (c *companyServer) RemoveMember(ctx context.Context, req *gen.RemoveMemberRequest) (*gen.RemoveMemberResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.RemoveMemberResponse{Success: false}, nil
	}

	// Can remove self (leave) or others if manager+
	if req.UserId != userID && !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.RemoveMemberResponse{Success: false, Message: "not authorized"}, nil
	}

	// Owner cannot be removed
	if c.isOwner(req.CompanyId, req.UserId) {
		return &gen.RemoveMemberResponse{Success: false, Message: "cannot remove owner"}, nil
	}

	_, err := c.db.Exec(`DELETE FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid`, req.CompanyId, req.UserId)
	if err != nil {
		return &gen.RemoveMemberResponse{Success: false}, nil
	}

	return &gen.RemoveMemberResponse{Success: true, Message: "Member removed"}, nil
}

func (c *companyServer) UpdateMemberPosition(ctx context.Context, req *gen.UpdateMemberPositionRequest) (*gen.UpdateMemberPositionResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.UpdateMemberPositionResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.UpdateMemberPositionResponse{Success: false}, nil
	}

	_, err := c.db.Exec(
		`UPDATE company_members SET position_id=$3::uuid WHERE company_id=$1::uuid AND user_id=$2::uuid`,
		req.CompanyId, req.UserId, req.PositionId,
	)
	if err != nil {
		return &gen.UpdateMemberPositionResponse{Success: false}, nil
	}

	// Return updated member
	var memberID string
	_ = c.db.QueryRow(`SELECT id FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid`, req.CompanyId, req.UserId).Scan(&memberID)
	member, _ := c.getMemberByID(memberID)

	return &gen.UpdateMemberPositionResponse{Success: true, Member: member}, nil
}

func (c *companyServer) ListMembers(ctx context.Context, req *gen.ListMembersRequest) (*gen.ListMembersResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.ListMembersResponse{}, nil
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT cm.id, cm.company_id, cm.user_id, u.username, COALESCE(u.avatar_url,''),
		       cp.id, cp.company_id, cp.title, cp.level, cp.chat_access,
		       cm.joined_at
		FROM company_members cm
		JOIN users u ON u.id = cm.user_id
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.company_id=$1::uuid`

	args := []interface{}{req.CompanyId}

	if req.Cursor != "" {
		query += ` AND cm.joined_at < (SELECT joined_at FROM company_members WHERE id=$2::uuid)`
		args = append(args, req.Cursor)
	}

	query += ` ORDER BY cm.joined_at DESC LIMIT ` + strconv.Itoa(limit+1)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return &gen.ListMembersResponse{}, nil
	}
	defer rows.Close()

	var members []*gen.CompanyMember
	var lastCursor string
	for rows.Next() {
		var m gen.CompanyMember
		var p gen.CompanyPosition
		var joinedAt time.Time
		if err := rows.Scan(&m.Id, &m.CompanyId, &m.UserId, &m.Username, &m.AvatarUrl,
			&p.Id, &p.CompanyId, &p.Title, &p.Level, &p.ChatAccess,
			&joinedAt); err == nil {
			m.Position = &p
			m.JoinedAt = joinedAt.Format(time.RFC3339)
			members = append(members, &m)
			lastCursor = m.Id
		}
	}

	hasMore := len(members) > limit
	if hasMore {
		members = members[:limit]
	}

	return &gen.ListMembersResponse{
		Members:    members,
		NextCursor: lastCursor,
		HasMore:    hasMore,
	}, nil
}

// --- Company Chats ---

func (c *companyServer) CreateCompanyChat(ctx context.Context, req *gen.CreateCompanyChatRequest) (*gen.CreateCompanyChatResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.CreateCompanyChatResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(req.CompanyId, userID) {
		return &gen.CreateCompanyChatResponse{Success: false}, nil
	}

	accessLevel := req.AccessLevel
	if accessLevel == "" {
		accessLevel = "member"
	}

	// Create chat
	chatID := uuid.New().String()
	participants := "[]"

	_, err := c.db.Exec(
		`INSERT INTO chats (id, name, type, participants, creator_id) VALUES ($1, $2, 'company', $3, $4::uuid)`,
		chatID, req.Name, participants, userID,
	)
	if err != nil {
		logger.Errorf("Company: CreateCompanyChat error: %v", err)
		return &gen.CreateCompanyChatResponse{Success: false}, nil
	}

	// Link to company
	_, err = c.db.Exec(
		`INSERT INTO company_chats (chat_id, company_id, access_level, min_position_level) VALUES ($1, $2::uuid, $3, $4)`,
		chatID, req.CompanyId, accessLevel, req.MinPositionLevel,
	)
	if err != nil {
		logger.Errorf("Company: CreateCompanyChat link error: %v", err)
		return &gen.CreateCompanyChatResponse{Success: false}, nil
	}

	// Add eligible members
	if len(req.ParticipantIds) > 0 {
		for _, pid := range req.ParticipantIds {
			c.addUserToCompanyChat(chatID, pid)
		}
	} else {
		c.addAllEligibleMembers(chatID, req.CompanyId, accessLevel, int(req.MinPositionLevel))
	}

	logger.Infof("Company: Created company chat %q (id=%s) for company %s", req.Name, chatID, req.CompanyId)

	return &gen.CreateCompanyChatResponse{Success: true, ChatId: chatID}, nil
}

func (c *companyServer) SetCompanyChatAccess(ctx context.Context, req *gen.SetCompanyChatAccessRequest) (*gen.SetCompanyChatAccessResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.SetCompanyChatAccessResponse{Success: false}, nil
	}

	// Find company_id for this chat
	var companyID string
	_ = c.db.QueryRow(`SELECT company_id FROM company_chats WHERE chat_id=$1`, req.ChatId).Scan(&companyID)
	if companyID == "" {
		return &gen.SetCompanyChatAccessResponse{Success: false}, nil
	}

	if !c.isOwnerOrManager(companyID, userID) {
		return &gen.SetCompanyChatAccessResponse{Success: false}, nil
	}

	_, err := c.db.Exec(
		`UPDATE company_chats SET access_level=$1, min_position_level=$2 WHERE chat_id=$3`,
		req.AccessLevel, req.MinPositionLevel, req.ChatId,
	)
	if err != nil {
		return &gen.SetCompanyChatAccessResponse{Success: false}, nil
	}

	return &gen.SetCompanyChatAccessResponse{Success: true}, nil
}

func (c *companyServer) GetCompanyChats(ctx context.Context, req *gen.GetCompanyChatsRequest) (*gen.GetCompanyChatsResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetCompanyChatsResponse{}, nil
	}

	rows, err := c.db.Query(`
		SELECT cc.chat_id, cc.company_id, cc.access_level, cc.min_position_level
		FROM company_chats cc
		WHERE cc.company_id=$1::uuid`, req.CompanyId)
	if err != nil {
		return &gen.GetCompanyChatsResponse{}, nil
	}
	defer rows.Close()

	var chats []*gen.CompanyChatInfo
	for rows.Next() {
		var ch gen.CompanyChatInfo
		if err := rows.Scan(&ch.ChatId, &ch.CompanyId, &ch.AccessLevel, &ch.MinPositionLevel); err == nil {
			chats = append(chats, &ch)
		}
	}

	return &gen.GetCompanyChatsResponse{Chats: chats}, nil
}

func (c *companyServer) JoinCompany(ctx context.Context, req *gen.JoinCompanyRequest) (*gen.JoinCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.JoinCompanyResponse{Success: false}, nil
	}

	// Check invite code (company_id used as invite for simplicity)
	companyID := req.CompanyId

	// Check if already a member
	var exists bool
	_ = c.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid)`, companyID, userID).Scan(&exists)
	if exists {
		return &gen.JoinCompanyResponse{Success: false}, nil
	}

	// Find default Employee position
	var posID string
	_ = c.db.QueryRow(`SELECT id FROM company_positions WHERE company_id=$1::uuid AND level=0 LIMIT 1`, companyID).Scan(&posID)
	if posID == "" {
		// Find any lowest level position
		_ = c.db.QueryRow(`SELECT id FROM company_positions WHERE company_id=$1::uuid ORDER BY level ASC LIMIT 1`, companyID).Scan(&posID)
	}
	if posID == "" {
		return &gen.JoinCompanyResponse{Success: false}, nil
	}

	var id string
	err := c.db.QueryRow(
		`INSERT INTO company_members (company_id, user_id, position_id) VALUES ($1::uuid, $2::uuid, $3::uuid) RETURNING id`,
		companyID, userID, posID,
	).Scan(&id)
	if err != nil {
		return &gen.JoinCompanyResponse{Success: false}, nil
	}

	member, _ := c.getMemberByID(id)

	// Auto-add to company chats with eligible access level
	c.autoJoinCompanyChats(userID, companyID)

	return &gen.JoinCompanyResponse{Success: true, Member: member}, nil
}

func (c *companyServer) LeaveCompany(ctx context.Context, req *gen.LeaveCompanyRequest) (*gen.LeaveCompanyResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.LeaveCompanyResponse{Success: false}, nil
	}

	if c.isOwner(req.CompanyId, userID) {
		return &gen.LeaveCompanyResponse{Success: false, Message: "owner cannot leave, transfer ownership first"}, nil
	}

	_, err := c.db.Exec(`DELETE FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid`, req.CompanyId, userID)
	if err != nil {
		return &gen.LeaveCompanyResponse{Success: false}, nil
	}

	// Remove from company chats
	c.autoLeaveCompanyChats(userID, req.CompanyId)

	return &gen.LeaveCompanyResponse{Success: true, Message: "Left company"}, nil
}

// --- Helpers ---

func (c *companyServer) getCompanyByID(id string) (*gen.Company, error) {
	var comp gen.Company
	var createdAt time.Time
	err := c.db.QueryRow(
		`SELECT id, name, owner_id, COALESCE(avatar_url,''), created_at FROM companies WHERE id=$1::uuid`, id,
	).Scan(&comp.Id, &comp.Name, &comp.OwnerId, &comp.AvatarUrl, &createdAt)
	if err != nil {
		return nil, err
	}
	comp.CreatedAt = createdAt.Format(time.RFC3339)
	_ = c.db.QueryRow(`SELECT COUNT(*) FROM company_members WHERE company_id=$1::uuid`, id).Scan(&comp.MemberCount)
	return &comp, nil
}

func (c *companyServer) getPositions(companyID string) ([]*gen.CompanyPosition, error) {
	rows, err := c.db.Query(`SELECT id, company_id, title, level, chat_access FROM company_positions WHERE company_id=$1::uuid ORDER BY level DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*gen.CompanyPosition
	for rows.Next() {
		var p gen.CompanyPosition
		if err := rows.Scan(&p.Id, &p.CompanyId, &p.Title, &p.Level, &p.ChatAccess); err == nil {
			positions = append(positions, &p)
		}
	}
	return positions, nil
}

func (c *companyServer) getMemberByID(id string) (*gen.CompanyMember, error) {
	var m gen.CompanyMember
	var p gen.CompanyPosition
	var joinedAt time.Time
	err := c.db.QueryRow(`
		SELECT cm.id, cm.company_id, cm.user_id, u.username, COALESCE(u.avatar_url,''),
		       cp.id, cp.company_id, cp.title, cp.level, cp.chat_access,
		       cm.joined_at
		FROM company_members cm
		JOIN users u ON u.id = cm.user_id
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.id=$1::uuid`, id,
	).Scan(&m.Id, &m.CompanyId, &m.UserId, &m.Username, &m.AvatarUrl,
		&p.Id, &p.CompanyId, &p.Title, &p.Level, &p.ChatAccess,
		&joinedAt)
	if err != nil {
		return nil, err
	}
	m.Position = &p
	m.JoinedAt = joinedAt.Format(time.RFC3339)
	return &m, nil
}

func (c *companyServer) isOwner(companyID, userID string) bool {
	var ownerID string
	_ = c.db.QueryRow(`SELECT owner_id FROM companies WHERE id=$1::uuid`, companyID).Scan(&ownerID)
	return ownerID == userID
}

func (c *companyServer) isOwnerOrManager(companyID, userID string) bool {
	if c.isOwner(companyID, userID) {
		return true
	}
	var level int
	err := c.db.QueryRow(`
		SELECT cp.level FROM company_members cm
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.company_id=$1::uuid AND cm.user_id=$2::uuid`, companyID, userID).Scan(&level)
	if err != nil {
		return false
	}
	return level >= 1
}

func (c *companyServer) getUserPositionLevel(companyID, userID string) int {
	var level int
	_ = c.db.QueryRow(`
		SELECT cp.level FROM company_members cm
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.company_id=$1::uuid AND cm.user_id=$2::uuid`, companyID, userID).Scan(&level)
	return level
}

func (c *companyServer) addUserToCompanyChat(chatID, userID string) {
	_, _ = c.db.Exec(
		`UPDATE chats SET participants = CASE
			WHEN participants = '[]' THEN '[''"'"'' || $1 || ''"'"']'
			WHEN participants LIKE '%'"'"'' || $1 || ''"'"''%' THEN participants
			ELSE rtrim(participants, ']') || ',"'"'"'' || $1 || ''"'"']'
		END WHERE id=$2`,
		userID, chatID,
	)
}

func (c *companyServer) addAllEligibleMembers(chatID, companyID, accessLevel string, minLevel int) {
	var levelThreshold int
	switch accessLevel {
	case "owner_only":
		levelThreshold = 3
	case "management":
		levelThreshold = 1
	default:
		levelThreshold = 0
	}
	if minLevel > levelThreshold {
		levelThreshold = minLevel
	}

	rows, err := c.db.Query(`
		SELECT cm.user_id FROM company_members cm
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.company_id=$1::uuid AND cp.level >= $2`, companyID, levelThreshold)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err == nil {
			c.addUserToCompanyChat(chatID, uid)
		}
	}
}

func (c *companyServer) autoJoinCompanyChats(userID, companyID string) {
	rows, err := c.db.Query(`
		SELECT cc.chat_id, cc.access_level, cc.min_position_level
		FROM company_chats cc
		WHERE cc.company_id=$1::uuid`, companyID)
	if err != nil {
		return
	}
	defer rows.Close()

	userLevel := c.getUserPositionLevel(companyID, userID)

	for rows.Next() {
		var chatID, accessLevel string
		var minLevel int
		if err := rows.Scan(&chatID, &accessLevel, &minLevel); err != nil {
			continue
		}

		threshold := 0
		switch accessLevel {
		case "owner_only":
			threshold = 3
		case "management":
			threshold = 1
		}
		if minLevel > threshold {
			threshold = minLevel
		}

		if userLevel >= threshold {
			c.addUserToCompanyChat(chatID, userID)
		}
	}
}

func (c *companyServer) autoLeaveCompanyChats(userID, companyID string) {
	rows, err := c.db.Query(`
		SELECT chat_id FROM company_chats WHERE company_id=$1::uuid`, companyID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chatID string
		if err := rows.Scan(&chatID); err != nil {
			continue
		}
		// Remove user from chat participants
		var participants string
		_ = c.db.QueryRow(`SELECT participants FROM chats WHERE id=$1`, chatID).Scan(&participants)
		newParticipants := removeParticipant(participants, userID)
		_, _ = c.db.Exec(`UPDATE chats SET participants=$1 WHERE id=$2`, newParticipants, chatID)
	}
}

func removeParticipant(participants, userID string) string {
	parts := strings.Split(strings.Trim(participants, "[]"), ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "'\"")
		if p != userID && p != "" {
			result = append(result, "'"+p+"'")
		}
	}
	if len(result) == 0 {
		return "[]"
	}
	return "[" + strings.Join(result, ",") + "]"
}

// GetUserInfo returns public user info including company membership.
func (c *companyServer) GetUserInfo(ctx context.Context, req *gen.GetUserInfoRequest) (*gen.GetUserInfoResponse, error) {
	var username, avatarURL, fullAvatarURL, bio, status string
	var lastSeenAt sql.NullTime
	err := c.db.QueryRow(
		`SELECT username, COALESCE(avatar_url,''), COALESCE(full_avatar_url,''), COALESCE(bio,''), COALESCE(status,''), last_seen_at
		 FROM users WHERE id=$1::uuid`, req.UserId,
	).Scan(&username, &avatarURL, &fullAvatarURL, &bio, &status, &lastSeenAt)
	if err != nil {
		return &gen.GetUserInfoResponse{}, nil
	}

	info := &gen.UserPublicInfo{
		UserId:        req.UserId,
		Username:      username,
		AvatarUrl:     avatarURL,
		FullAvatarUrl: fullAvatarURL,
		Bio:           bio,
		Status:        status,
	}
	if lastSeenAt.Valid {
		info.LastSeenAt = lastSeenAt.Time.Format(time.RFC3339)
	}

	// Company info (highest position)
	var companyID, companyName, positionTitle sql.NullString
	var positionLevel sql.NullInt32
	_ = c.db.QueryRow(`
		SELECT c.id, c.name, cp.title, cp.level
		FROM company_members cm
		JOIN companies c ON c.id = cm.company_id
		JOIN company_positions cp ON cp.id = cm.position_id
		WHERE cm.user_id=$1::uuid
		ORDER BY cp.level DESC LIMIT 1`, req.UserId).Scan(&companyID, &companyName, &positionTitle, &positionLevel)

	if companyID.Valid {
		info.CompanyId = companyID.String
		info.CompanyName = companyName.String
	}
	if positionTitle.Valid {
		info.PositionTitle = positionTitle.String
	}
	if positionLevel.Valid {
		info.PositionLevel = int32(positionLevel.Int32)
	}

	return &gen.GetUserInfoResponse{Info: info}, nil
}

// GetCompanyByUser returns the company and membership for a user.
func (c *companyServer) GetCompanyByUser(ctx context.Context, req *gen.GetCompanyByUserRequest) (*gen.GetCompanyByUserResponse, error) {
	var companyID string
	_ = c.db.QueryRow(`
		SELECT cm.company_id FROM company_members cm WHERE cm.user_id=$1::uuid LIMIT 1`, req.UserId).Scan(&companyID)
	if companyID == "" {
		return &gen.GetCompanyByUserResponse{}, nil
	}

	company, err := c.getCompanyByID(companyID)
	if err != nil {
		return &gen.GetCompanyByUserResponse{}, nil
	}

	// Get member record
	var memberID string
	_ = c.db.QueryRow(`SELECT id FROM company_members WHERE company_id=$1::uuid AND user_id=$2::uuid`, companyID, req.UserId).Scan(&memberID)
	member, _ := c.getMemberByID(memberID)

	return &gen.GetCompanyByUserResponse{Company: company, Member: member}, nil
}


