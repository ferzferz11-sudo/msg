package main

// server_ai_v2.go — gRPC handlers for AI Services v2

import (
	"LavenderMessenger/gen"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ======= ChatWithAIV2 =======

func (s *server) ChatWithAIV2(req *gen.ChatWithAIV2Request, stream gen.ChatService_ChatWithAIV2Server) error {
	userID := getAIV2UserID(stream.Context())
	if userID == "" {
		return status.Error(codes.Unauthenticated, "unauthorized")
	}

	gateway := s.aiGateway
	if gateway == nil {
		return status.Error(codes.Internal, "AI gateway not initialized")
	}

	chatReq := &ChatRequest{
		UserID:  userID,
		ChatID:  req.SessionId,
		Message: req.Message,
		AgentID: req.AgentId,
	}

	logger.Infof("[AI] ChatWithAIV2: user=%s agent=%s session=%s msg=%dchars", userID, req.AgentId, req.SessionId, len(req.Message))

	err := gateway.Chat(stream.Context(), chatReq, func(token string, finished bool, imageURL string) error {
		return stream.Send(&gen.ChatWithAIV2Response{
			Token:    token,
			Finished: finished,
			ImageUrl: imageURL,
		})
	})

	if err != nil {
		logger.Infof("[AI] ChatWithAIV2 error: user=%s agent=%s err=%v", userID, req.AgentId, err)
		stream.Send(&gen.ChatWithAIV2Response{
			Error:    err.Error(),
			Finished: true,
		})
	}

	return nil
}

// ======= Agent CRUD =======

func (s *server) CreateAIAgent(ctx context.Context, req *gen.CreateAIAgentRequest) (*gen.CreateAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.CreateAIAgentResponse{Error: "unauthorized"}, nil
	}

	if req.Name == "" {
		return &gen.CreateAIAgentResponse{Error: "name is required"}, nil
	}
	if req.ProviderType == "" {
		return &gen.CreateAIAgentResponse{Error: "provider_type is required"}, nil
	}

	agentID := fmt.Sprintf("agent-%s", uuid.New().String()[:8])

	providerConfig := make(map[string]any)
	if req.ProviderConfig != "" {
		json.Unmarshal([]byte(req.ProviderConfig), &providerConfig)
	}
	ragConfig := make(map[string]any)
	if req.RagConfig != "" {
		json.Unmarshal([]byte(req.RagConfig), &ragConfig)
	}

	var rateLimit *int
	if req.RateLimit > 0 {
		rl := int(req.RateLimit)
		rateLimit = &rl
	}

	agent := &AgentV2{
		ID:             agentID,
		Name:           req.Name,
		Description:    req.Description,
		ProviderType:   req.ProviderType,
		ProviderConfig: providerConfig,
		SystemPrompt:   req.SystemPrompt,
		Model:          req.Model,
		MaxTokens:      int(req.MaxTokens),
		Temperature:    float64(req.Temperature),
		ToolsEnabled:   req.ToolsEnabled,
		ToolWhitelist:  req.ToolWhitelist,
		RAGEnabled:     req.RagEnabled,
		RAGConfig:      ragConfig,
		RateLimit:      rateLimit,
		IsPublic:       req.IsPublic,
		IsActive:       true,
		CreatedBy:      userID,
		Version:        1,
	}

	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}
	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}

	err := s.db.CreateAgentV2(agent)
	if err != nil {
		return &gen.CreateAIAgentResponse{Error: err.Error()}, nil
	}

	logger.Infof("[AI] CreateAgent: id=%s name=%s provider=%s user=%s", agentID, req.Name, req.ProviderType, userID)

	return &gen.CreateAIAgentResponse{
		Success: true,
		AgentId: agentID,
	}, nil
}

func (s *server) UpdateAIAgent(ctx context.Context, req *gen.UpdateAIAgentRequest) (*gen.UpdateAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.UpdateAIAgentResponse{Error: "unauthorized"}, nil
	}

	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return &gen.UpdateAIAgentResponse{Error: "agent not found"}, nil
	}

	if agent.CreatedBy != userID && !agent.IsPreset {
		return &gen.UpdateAIAgentResponse{Error: "permission denied"}, nil
	}

	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.ProviderConfig != "" {
		json.Unmarshal([]byte(req.ProviderConfig), &agent.ProviderConfig)
	}
	if req.SystemPrompt != "" {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.Model != "" {
		agent.Model = req.Model
	}
	if req.MaxTokens > 0 {
		agent.MaxTokens = int(req.MaxTokens)
	}
	if req.Temperature > 0 {
		agent.Temperature = float64(req.Temperature)
	}
	agent.ToolsEnabled = req.ToolsEnabled
	agent.ToolWhitelist = req.ToolWhitelist
	agent.RAGEnabled = req.RagEnabled
	if req.RagConfig != "" {
		json.Unmarshal([]byte(req.RagConfig), &agent.RAGConfig)
	}
	if req.RateLimit > 0 {
		rl := int(req.RateLimit)
		agent.RateLimit = &rl
	}
	agent.IsPublic = req.IsPublic

	if err := s.db.UpdateAgentV2(agent); err != nil {
		return &gen.UpdateAIAgentResponse{Error: err.Error()}, nil
	}

	logger.Infof("[AI] UpdateAgent: id=%s user=%s", req.AgentId, userID)

	return &gen.UpdateAIAgentResponse{Success: true}, nil
}

func (s *server) DeleteAIAgent(ctx context.Context, req *gen.DeleteAIAgentRequest) (*gen.DeleteAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.DeleteAIAgentResponse{Error: "unauthorized"}, nil
	}

	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return &gen.DeleteAIAgentResponse{Error: "agent not found"}, nil
	}

	if agent.CreatedBy != userID && !agent.IsPreset {
		return &gen.DeleteAIAgentResponse{Error: "permission denied"}, nil
	}

	if agent.IsPreset {
		return &gen.DeleteAIAgentResponse{Error: "cannot delete preset agent"}, nil
	}

	if err := s.db.DeleteAgentV2(req.AgentId); err != nil {
		return &gen.DeleteAIAgentResponse{Error: err.Error()}, nil
	}

	logger.Infof("[AI] DeleteAgent: id=%s user=%s", req.AgentId, userID)

	return &gen.DeleteAIAgentResponse{Success: true}, nil
}

func (s *server) GetAIAgent(ctx context.Context, req *gen.GetAIAgentRequest) (*gen.GetAIAgentResponse, error) {
	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "agent not found")
	}

	return &gen.GetAIAgentResponse{
		Agent: agentToProto(agent),
	}, nil
}

func (s *server) ListAIAgents(ctx context.Context, req *gen.ListAIAgentsRequest) (*gen.ListAIAgentsResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.ListAIAgentsResponse{}, nil
	}

	agents, err := s.db.ListAgentsV2(userID, req.IncludePublic)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var result []*gen.AgentInfoV2
	for _, a := range agents {
		result = append(result, agentToProto(a))
	}

	logger.Infof("[AI] ListAgents: user=%s includePublic=%v count=%d", userID, req.IncludePublic, len(result))

	return &gen.ListAIAgentsResponse{Agents: result}, nil
}

func (s *server) CloneAIAgent(ctx context.Context, req *gen.CloneAIAgentRequest) (*gen.CloneAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.CloneAIAgentResponse{Error: "unauthorized"}, nil
	}

	original, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return &gen.CloneAIAgentResponse{Error: "agent not found"}, nil
	}

	newID := fmt.Sprintf("agent-%s", uuid.New().String()[:8])
	clone := &AgentV2{
		ID:              newID,
		Name:            req.NewName,
		Description:     original.Description,
		ProviderType:    original.ProviderType,
		ProviderConfig:  original.ProviderConfig,
		SystemPrompt:    original.SystemPrompt,
		Model:           original.Model,
		MaxTokens:       original.MaxTokens,
		Temperature:     original.Temperature,
		ToolsEnabled:    original.ToolsEnabled,
		ToolWhitelist:   original.ToolWhitelist,
		RAGEnabled:      original.RAGEnabled,
		RAGConfig:       original.RAGConfig,
		RateLimit:       original.RateLimit,
		IsPreset:        false,
		IsPublic:        false,
		IsActive:        true,
		CreatedBy:       userID,
		OriginalAgentID: original.ID,
		Tags:            original.Tags,
		Version:         1,
	}

	if err := s.db.CreateAgentV2(clone); err != nil {
		return &gen.CloneAIAgentResponse{Error: err.Error()}, nil
	}

	logger.Infof("[AI] CloneAgent: from=%s new=%s user=%s", req.AgentId, newID, userID)

	return &gen.CloneAIAgentResponse{
		Success: true,
		AgentId: newID,
	}, nil
}

func (s *server) ListAITools(ctx context.Context, req *gen.ListAIToolsRequest) (*gen.ListAIToolsResponse, error) {
	gateway := s.aiGateway
	if gateway == nil {
		return &gen.ListAIToolsResponse{}, nil
	}

	infos := gateway.tools.ListInfo()
	var tools []*gen.ToolInfoV2
	for _, info := range infos {
		tools = append(tools, &gen.ToolInfoV2{
			Name:             info.Name,
			Description:      info.Description,
			ParametersSchema: info.ParametersSchema,
			RequiredRole:     info.RequiredRole,
		})
	}

	return &gen.ListAIToolsResponse{Tools: tools}, nil
}

// ======= Marketplace Handlers =======

func (s *server) RateAIAgent(ctx context.Context, req *gen.RateAIAgentRequest) (*gen.RateAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.RateAIAgentResponse{Error: "unauthorized"}, nil
	}

	if req.Rating < 1 || req.Rating > 5 {
		return &gen.RateAIAgentResponse{Error: "rating must be 1-5"}, nil
	}

	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return &gen.RateAIAgentResponse{Error: "agent not found"}, nil
	}
	if agent.IsPreset {
		return &gen.RateAIAgentResponse{Error: "cannot rate preset agents"}, nil
	}

	review := &AgentReview{
		AgentID: req.AgentId,
		UserID:  userID,
		Rating:  int(req.Rating),
		Review:  req.Review,
	}

	if err := s.db.AddAgentReview(review); err != nil {
		return &gen.RateAIAgentResponse{Error: err.Error()}, nil
	}

	updated, _ := s.db.GetAgentV2(req.AgentId)
	var avgRating float32
	var reviewCount int32
	if updated != nil {
		avgRating = float32(updated.AvgRating)
		reviewCount = int32(updated.ReviewCount)
	}

	logger.Infof("[AI] RateAgent: agent=%s user=%s rating=%d", req.AgentId, userID, req.Rating)

	return &gen.RateAIAgentResponse{
		Success:     true,
		AvgRating:   avgRating,
		ReviewCount: reviewCount,
	}, nil
}

func (s *server) GetAIAgentReviews(ctx context.Context, req *gen.GetAIAgentReviewsRequest) (*gen.GetAIAgentReviewsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	reviews, err := s.db.GetAgentReviews(req.AgentId, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var result []*gen.AgentReviewInfo
	for _, r := range reviews {
		result = append(result, &gen.AgentReviewInfo{
			UserId:    r.UserID,
			Rating:    int32(r.Rating),
			Review:    r.Review,
			CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	agent, _ := s.db.GetAgentV2(req.AgentId)
	var avgRating float32
	var reviewCount int32
	if agent != nil {
		avgRating = float32(agent.AvgRating)
		reviewCount = int32(agent.ReviewCount)
	}

	logger.Infof("[AI] GetReviews: agent=%s count=%d", req.AgentId, len(result))

	return &gen.GetAIAgentReviewsResponse{
		Reviews:     result,
		AvgRating:   avgRating,
		ReviewCount: reviewCount,
	}, nil
}

func (s *server) ListMarketplaceAgents(ctx context.Context, req *gen.ListMarketplaceAgentsRequest) (*gen.ListMarketplaceAgentsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int(req.Offset)

	agents, err := s.db.ListMarketplaceAgents(req.Query, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var result []*gen.AgentInfoV2
	for _, a := range agents {
		result = append(result, agentToProto(a))
	}

	logger.Infof("[AI] Marketplace: query=%q limit=%d offset=%d results=%d", req.Query, limit, offset, len(result))

	return &gen.ListMarketplaceAgentsResponse{
		Agents: result,
		Total:  int32(len(result)),
	}, nil
}

func (s *server) GetAIAgentStats(ctx context.Context, req *gen.GetAIAgentStatsRequest) (*gen.GetAIAgentStatsResponse, error) {
	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "agent not found")
	}

	return &gen.GetAIAgentStatsResponse{
		InstallCount: int32(agent.InstallCount),
		AvgRating:    float32(agent.AvgRating),
		ReviewCount:  int32(agent.ReviewCount),
	}, nil
}

func (s *server) ShareAIAgent(ctx context.Context, req *gen.ShareAIAgentRequest) (*gen.ShareAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.ShareAIAgentResponse{Error: "unauthorized"}, nil
	}

	agent, err := s.db.GetAgentV2(req.AgentId)
	if err != nil {
		return &gen.ShareAIAgentResponse{Error: "agent not found"}, nil
	}

	if agent.CreatedBy != userID {
		return &gen.ShareAIAgentResponse{Error: "permission denied"}, nil
	}

	shareCode := agent.ShareCode
	if shareCode == "" {
		shareCode = uuid.New().String()[:8]
		if err := s.db.SetAgentShareCode(agent.ID, shareCode); err != nil {
			return &gen.ShareAIAgentResponse{Error: err.Error()}, nil
		}
	}

	logger.Infof("[AI] ShareAgent: agent=%s code=%s user=%s", req.AgentId, shareCode, userID)

	return &gen.ShareAIAgentResponse{
		Success:   true,
		ShareCode: shareCode,
	}, nil
}

func (s *server) InstallAIAgent(ctx context.Context, req *gen.InstallAIAgentRequest) (*gen.InstallAIAgentResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.InstallAIAgentResponse{Error: "unauthorized"}, nil
	}

	original, err := s.db.GetAgentByShareCode(req.ShareCode)
	if err != nil {
		return &gen.InstallAIAgentResponse{Error: "agent not found for share code"}, nil
	}

	newID := fmt.Sprintf("agent-%s", uuid.New().String()[:8])
	newName := req.NewName
	if newName == "" {
		newName = original.Name
	}

	clone := &AgentV2{
		ID:              newID,
		Name:            newName,
		Description:     original.Description,
		ProviderType:    original.ProviderType,
		ProviderConfig:  original.ProviderConfig,
		SystemPrompt:    original.SystemPrompt,
		Model:           original.Model,
		MaxTokens:       original.MaxTokens,
		Temperature:     original.Temperature,
		ToolsEnabled:    original.ToolsEnabled,
		ToolWhitelist:   original.ToolWhitelist,
		RAGEnabled:      original.RAGEnabled,
		RAGConfig:       original.RAGConfig,
		RateLimit:       original.RateLimit,
		IsPreset:        false,
		IsPublic:        false,
		IsActive:        true,
		CreatedBy:       userID,
		OriginalAgentID: original.ID,
		Tags:            original.Tags,
		Version:         1,
	}

	if err := s.db.CreateAgentV2(clone); err != nil {
		return &gen.InstallAIAgentResponse{Error: err.Error()}, nil
	}

	s.db.IncrementInstallCount(original.ID)

	logger.Infof("[AI] InstallAgent: code=%s from=%s new=%s user=%s", req.ShareCode, original.ID, newID, userID)

	return &gen.InstallAIAgentResponse{
		Success: true,
		AgentId: newID,
	}, nil
}

func (s *server) GetAIUsageStats(ctx context.Context, req *gen.GetAIUsageStatsRequest) (*gen.GetAIUsageStatsResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.GetAIUsageStatsResponse{}, nil
	}

	gateway := s.aiGateway
	if gateway == nil {
		return &gen.GetAIUsageStatsResponse{}, nil
	}

	stats, err := gateway.GetAIUsageStats(userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	totalTokens, totalRequests, _ := gateway.GetAIUsageStatsSummary(userID)

	var result []*gen.UsageStatInfo
	for _, stat := range stats {
		agentName := stat.AgentID
		if a, err := s.db.GetAgentV2(stat.AgentID); err == nil {
			agentName = a.Name
		}
		result = append(result, &gen.UsageStatInfo{
			AgentId:      stat.AgentID,
			TotalTokens:  int32(stat.TotalTokens),
			RequestCount: int32(stat.RequestCount),
			PeriodStart:  stat.PeriodStart.Format("2006-01-02T15:04:05Z"),
			AgentName:    agentName,
		})
	}

	logger.Infof("[AI] UsageStats: user=%s tokens=%d requests=%d", userID, totalTokens, totalRequests)

	return &gen.GetAIUsageStatsResponse{
		Stats:         result,
		TotalTokens:   int32(totalTokens),
		TotalRequests: int32(totalRequests),
	}, nil
}

// ======= helpers =======

func agentToProto(a *AgentV2) *gen.AgentInfoV2 {
	caps := &gen.AgentCapabilitiesV2{
		SupportsImages:    a.ProviderType == "openrouter" || a.ProviderType == "mimo",
		SupportsTools:     a.ToolsEnabled,
		SupportsStreaming: true,
		MaxTokens:         int32(a.MaxTokens),
	}
	return &gen.AgentInfoV2{
		Id:              a.ID,
		Name:            a.Name,
		Description:     a.Description,
		ProviderType:    a.ProviderType,
		Model:           a.Model,
		SystemPrompt:    a.SystemPrompt,
		ToolsEnabled:    a.ToolsEnabled,
		RagEnabled:      a.RAGEnabled,
		IsPreset:        a.IsPreset,
		IsPublic:        a.IsPublic,
		MaxTokens:       int32(a.MaxTokens),
		Temperature:     float32(a.Temperature),
		CreatedBy:       a.CreatedBy,
		Capabilities:    caps,
		InstallCount:    int32(a.InstallCount),
		AvgRating:       float32(a.AvgRating),
		ReviewCount:     int32(a.ReviewCount),
		Tags:            a.Tags,
		OriginalAgentId: a.OriginalAgentID,
		Version:         int32(a.Version),
		ShareCode:       a.ShareCode,
	}
}

func getAIV2UserID(ctx context.Context) string {
	return GetUserID(ctx)
}

// ======= AI Chat Settings (per-session user API key & model override) =======

func (s *server) GetAIChatSettings(ctx context.Context, req *gen.GetAIChatSettingsRequest) (*gen.AIChatSettings, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.AIChatSettings{}, nil
	}

	if req.SessionId == "" {
		return &gen.AIChatSettings{}, nil
	}

	chat, err := s.db.GetAIChatV2(req.SessionId)
	if err != nil {
		return &gen.AIChatSettings{}, nil
	}
	if chat.UserID != userID {
		return &gen.AIChatSettings{}, nil
	}

	apiKey, _ := chat.Settings["user_api_key"].(string)
	model, _ := chat.Settings["model_override"].(string)
	isCustom := apiKey != ""

	var remaining, limit, windowSeconds int32

	return &gen.AIChatSettings{
		SessionId:        req.SessionId,
		UserApiKey:       apiKey,
		Model:            model,
		IsUsingCustomKey: isCustom,
		Remaining:        remaining,
		Limit:            limit,
		WindowSeconds:    windowSeconds,
	}, nil
}

func (s *server) UpdateAIChatSettings(ctx context.Context, req *gen.UpdateAIChatSettingsRequest) (*gen.UpdateAIChatSettingsResponse, error) {
	userID := getAIV2UserID(ctx)
	if userID == "" {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "unauthorized"}, nil
	}

	if req.SessionId == "" {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "session_id required"}, nil
	}

	chat, err := s.db.GetAIChatV2(req.SessionId)
	if err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "chat not found"}, nil
	}
	if chat.UserID != userID {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: "permission denied"}, nil
	}

	if chat.Settings == nil {
		chat.Settings = make(map[string]any)
	}

	if req.ApiKey != "" {
		chat.Settings["user_api_key"] = req.ApiKey
	} else {
		delete(chat.Settings, "user_api_key")
	}

	if req.Model != "" {
		chat.Settings["model_override"] = req.Model
	} else {
		delete(chat.Settings, "model_override")
	}

	if err := s.db.UpdateAIChatV2(chat); err != nil {
		return &gen.UpdateAIChatSettingsResponse{Success: false, Message: err.Error()}, nil
	}

	logger.Infof("[AI] UpdateSettings: session=%s user=%s hasKey=%v model=%s", req.SessionId, userID, req.ApiKey != "", req.Model)

	return &gen.UpdateAIChatSettingsResponse{
		Success: true,
		Message: "settings updated",
	}, nil
}
