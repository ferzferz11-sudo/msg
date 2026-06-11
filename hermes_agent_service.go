package main

// hermes_agent_service.go — реализация HermesAgentServiceServer
// Принимает подключения от hermes-agent daemon через bidirectional gRPC stream

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"LavenderMessenger/auth"
	hermesagent "LavenderMessenger/gen/hermes_agent"
)

// agentStream — информация о подключённом агенте через stream
type agentStream struct {
	agentID   string
	agentName string
	host      string
	ipAddress string
	os        string
	caps      []string
	stream    hermesagent.HermesAgentService_ConnectServer
	mu        sync.Mutex
}

// streamSend — потокобезопасная отправка сообщения агенту
func (as *agentStream) send(msg *hermesagent.OrchestratorMessage) error {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.stream.Send(msg)
}

// hermesAgentServer реализует HermesAgentServiceServer
type hermesAgentServer struct {
	*server
	hermesagent.UnimplementedHermesAgentServiceServer
	hermesOrchestrator *Orchestrator
	streams            map[string]*agentStream // agentID → stream
	mu                 sync.RWMutex
}

// newHermesAgentServer создаёт сервер для HermesAgentService
func newHermesAgentServer(s *server, o *Orchestrator) *hermesAgentServer {
	return &hermesAgentServer{
		server:             s,
		hermesOrchestrator: o,
		streams:            make(map[string]*agentStream),
	}
}

// Connect — bidirectional stream между оркестратором и агентом
func (h *hermesAgentServer) Connect(stream hermesagent.HermesAgentService_ConnectServer) error {
	var as *agentStream

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[HermesAgentService] stream closed")
			h.unregisterStream(as)
			return nil
		}
		if err != nil {
			log.Printf("[HermesAgentService] recv error: %v", err)
			h.unregisterStream(as)
			return err
		}

		switch msg.Type {
		case hermesagent.AgentMessageType_AGENT_REGISTER:
			as, err = h.handleRegister(stream, msg)
			if err != nil {
				return err
			}
		case hermesagent.AgentMessageType_AGENT_HEARTBEAT:
			h.handleHeartbeat(msg)
		case hermesagent.AgentMessageType_AGENT_TASK_RESULT:
			if as != nil {
				h.handleTaskResult(as, msg)
			}
		case hermesagent.AgentMessageType_AGENT_LOG:
			h.handleAgentLog(msg)
		case hermesagent.AgentMessageType_AGENT_DISCONNECT:
			log.Printf("[HermesAgentService] agent %s disconnecting", msg.AgentId)
			h.unregisterStream(as)
			return nil
		case hermesagent.AgentMessageType_AGENT_ERROR:
			log.Printf("[HermesAgentService] agent %s error: %s", msg.AgentId, string(msg.Payload))
			h.handleAgentError(msg)
		default:
			log.Printf("[HermesAgentService] unknown type: %v", msg.Type)
		}
	}
}

// HealthCheck — одноразовая проверка связи
func (h *hermesAgentServer) HealthCheck(_ context.Context, req *hermesagent.HealthCheckRequest) (*hermesagent.HealthCheckResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id required")
	}
	h.mu.RLock()
	_, ok := h.streams[req.AgentId]
	h.mu.RUnlock()
	if !ok {
		return &hermesagent.HealthCheckResponse{Status: "not_connected"}, nil
	}
	return &hermesagent.HealthCheckResponse{Status: "ok", Version: "1.0.0"}, nil
}

func (h *hermesAgentServer) handleRegister(stream hermesagent.HermesAgentService_ConnectServer, msg *hermesagent.AgentMessage) (*agentStream, error) {
	var info hermesagent.RegistrationInfo
	if err := proto.Unmarshal(msg.Payload, &info); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid registration payload")
	}
	agentID := info.AgentId
	if agentID == "" {
		agentID = msg.AgentId
	}
	log.Printf("[HermesAgentService] register: id=%s name=%s host=%s caps=%v",
		agentID, info.AgentName, info.Host, info.Capabilities)

	if info.AuthToken != "" && !h.validateToken(agentID, info.AuthToken) {
		return nil, status.Error(codes.Unauthenticated, "invalid auth token")
	}

	as := &agentStream{
		agentID: agentID, agentName: info.AgentName,
		host: info.Host, ipAddress: info.IpAddress, os: info.Os,
		caps: info.Capabilities, stream: stream,
	}

	h.mu.Lock()
	h.streams[agentID] = as
	h.mu.Unlock()

	h.hermesOrchestrator.remoteManager.RegisterAgent(&RemoteAgent{
		ID: agentID, Name: info.AgentName, IPAddress: info.IpAddress,
		Host: info.Host, OS: info.Os, Capabilities: info.Capabilities,
		Status: "connected", LastHeartbeat: time.Now(), MaxConcurrent: 3,
	})
	h.hermesOrchestrator.remoteManager.SetStream(agentID, as)
	return as, nil
}

func (h *hermesAgentServer) handleHeartbeat(msg *hermesagent.AgentMessage) {
	agent := h.hermesOrchestrator.remoteManager.GetAgent(msg.AgentId)
	if agent != nil {
		agent.mu.Lock()
		agent.LastHeartbeat = time.Now()
		agent.Status = "connected"
		agent.mu.Unlock()
	}
}

func (h *hermesAgentServer) handleTaskResult(as *agentStream, msg *hermesagent.AgentMessage) {
	var result hermesagent.TaskResult
	if err := proto.Unmarshal(msg.Payload, &result); err != nil {
		log.Printf("[HermesAgentService] result unmarshal error: %v", err)
		return
	}
	log.Printf("[HermesAgentService] task=%s status=%v duration=%dms",
		result.TaskId, result.Status, result.DurationMs)

	h.hermesOrchestrator.remoteManager.HandleTaskResult(as.agentID, &RemoteTaskResult{
		TaskID: result.TaskId, Status: taskStatusFromProto(result.Status),
		Stdout: result.Stdout, Stderr: result.Stderr,
		ExitCode: int(result.ExitCode),
		Duration: time.Duration(result.DurationMs) * time.Millisecond,
		Error:    result.Error,
	})
}

func (h *hermesAgentServer) handleAgentLog(msg *hermesagent.AgentMessage) {
	var entry hermesagent.LogEntry
	if err := proto.Unmarshal(msg.Payload, &entry); err == nil {
		log.Printf("[AgentLog %s@%s] %s", entry.Level, msg.AgentId, entry.Message)
	}
}

func (h *hermesAgentServer) handleAgentError(msg *hermesagent.AgentMessage) {
	if agent := h.hermesOrchestrator.remoteManager.GetAgent(msg.AgentId); agent != nil {
		agent.mu.Lock()
		agent.Status = "error"
		agent.mu.Unlock()
	}
}

func (h *hermesAgentServer) unregisterStream(as *agentStream) {
	if as == nil {
		return
	}
	h.mu.Lock()
	delete(h.streams, as.agentID)
	h.mu.Unlock()
	h.hermesOrchestrator.remoteManager.UnregisterAgent(as.agentID)
}

func (h *hermesAgentServer) validateToken(agentID, token string) bool {
	if token == "" {
		log.Printf("[HermesAgentService] reject %s: empty token", agentID)
		return false
	}

	// Парсим и валидируем JWT
	claims, err := auth.ValidateAgentToken(token)
	if err != nil {
		log.Printf("[HermesAgentService] reject %s: invalid token: %v", agentID, err)
		return false
	}

	// Проверяем что agent_id в токене совпадает с заявленным
	if claims.AgentID != agentID {
		log.Printf("[HermesAgentService] reject %s: token agent_id mismatch (token=%s)",
			agentID, claims.AgentID)
		return false
	}

	// Проверяем что токен не отозван в БД
	if h.server.hermesDB != nil {
		tokenHash := hashToken(token)
		storedToken, err := h.server.hermesDB.GetAgentTokenByHash(tokenHash)
		if err != nil {
			log.Printf("[HermesAgentService] reject %s: token not found in DB: %v", agentID, err)
			return false
		}
		if storedToken.Revoked {
			log.Printf("[HermesAgentService] reject %s: token revoked", agentID)
			return false
		}
	}

	log.Printf("[HermesAgentService] token valid: %s (%s)", agentID, claims.AgentName)
	return true
}

// hashToken вычисляет SHA-256 хеш токена для хранения в БД
func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// isAdmin проверяет является ли пользователь супер-админом
func (h *hermesAgentServer) isAdmin(userID string) bool {
	if h.server == nil || h.server.db == nil || userID == "" {
		return false
	}
	return h.server.db.IsSuperAdmin(userID)
}

// GenerateAgentToken — генерация JWT токена для нового агента
func (h *hermesAgentServer) GenerateAgentToken(_ context.Context, req *hermesagent.GenerateAgentTokenRequest) (*hermesagent.GenerateAgentTokenResponse, error) {
	if req.AgentId == "" || req.AgentName == "" {
		return &hermesagent.GenerateAgentTokenResponse{
			Success: false, Error: "agent_id and agent_name are required",
		}, nil
	}

	ttl := time.Duration(req.TtlHours) * time.Hour

	token, err := auth.GenerateAgentToken(req.AgentId, req.AgentName, req.Capabilities, ttl)
	if err != nil {
		return &hermesagent.GenerateAgentTokenResponse{
			Success: false, Error: fmt.Sprintf("generate token: %v", err),
		}, nil
	}

	// Сохраняем хеш токена в БД
	tokenHash := hashToken(token)
	var expiresAt time.Time
	if req.TtlHours > 0 {
		expiresAt = time.Now().Add(ttl)
	} else {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	if h.server.hermesDB != nil {
		if err := h.server.hermesDB.SaveAgentToken(
			req.AgentId, req.AgentName, tokenHash,
			req.Capabilities, expiresAt, req.AdminUserId,
		); err != nil {
			log.Printf("[HermesAgentService] failed to save token: %v", err)
		}
	}

	claims, _ := auth.ValidateAgentToken(token)

	return &hermesagent.GenerateAgentTokenResponse{
		Success:   true,
		Token:     token,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

// RevokeAgentToken — отзыв токена агента
func (h *hermesAgentServer) RevokeAgentToken(_ context.Context, req *hermesagent.RevokeAgentTokenRequest) (*hermesagent.RevokeAgentTokenResponse, error) {
	if req.AgentId == "" {
		return &hermesagent.RevokeAgentTokenResponse{
			Success: false, Error: "agent_id is required",
		}, nil
	}

	if h.server.hermesDB != nil {
		if err := h.server.hermesDB.RevokeAgentToken(req.AgentId); err != nil {
			return &hermesagent.RevokeAgentTokenResponse{
				Success: false, Error: fmt.Sprintf("revoke: %v", err),
			}, nil
		}
	}

	log.Printf("[HermesAgentService] token revoked: agent=%s by=%s", req.AgentId, req.AdminUserId)
	return &hermesagent.RevokeAgentTokenResponse{Success: true}, nil
}

// ListAgentTokens — список всех токенов агентов
func (h *hermesAgentServer) ListAgentTokens(_ context.Context, req *hermesagent.ListAgentTokensRequest) (*hermesagent.ListAgentTokensResponse, error) {
	if h.server.hermesDB == nil {
		return &hermesagent.ListAgentTokensResponse{
			Success: false, Error: "database not available",
		}, nil
	}

	tokens, err := h.server.hermesDB.ListAgentTokens()
	if err != nil {
		return &hermesagent.ListAgentTokensResponse{
			Success: false, Error: fmt.Sprintf("list: %v", err),
		}, nil
	}

	var infos []*hermesagent.AgentTokenInfo
	for _, t := range tokens {
		infos = append(infos, &hermesagent.AgentTokenInfo{
			Id:           t.ID,
			AgentId:      t.AgentID,
			AgentName:    t.AgentName,
			TokenHash:    t.Token,
			Capabilities: t.Capabilities,
			CreatedAt:    t.CreatedAt.Format(time.RFC3339),
			ExpiresAt:    t.ExpiresAt.Format(time.RFC3339),
			Revoked:      t.Revoked,
			CreatedBy:    t.CreatedBy,
		})
	}

	return &hermesagent.ListAgentTokensResponse{Success: true, Tokens: infos}, nil
}

// SendTaskToAgent отправляет задачу агенту через stream и ждёт результат
func (h *hermesAgentServer) SendTaskToAgent(agentID string, task *RemoteTask) (*RemoteTaskResult, error) {
	h.mu.RLock()
	as, ok := h.streams[agentID]
	h.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	pt := &hermesagent.Task{
		TaskId: task.ID, TaskType: taskTypeToProto(task.Type),
		Params: task.Params, WorkingDir: task.WorkingDir,
		TimeoutSec: int32(task.TimeoutSec), StreamOutput: task.StreamOutput,
	}
	payload, err := proto.Marshal(pt)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}
	if err := as.send(&hermesagent.OrchestratorMessage{
		TargetAgentId: agentID, Type: hermesagent.OrchestratorMessageType_ORCHESTRATOR_TASK,
		Payload: payload, TimestampMs: time.Now().UnixMilli(),
	}); err != nil {
		return nil, fmt.Errorf("send task: %w", err)
	}

	timeout := time.Duration(task.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	select {
	case <-task.Done:
		return task.Result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("task %s timeout", task.ID)
	}
}

// GetConnectedAgentIDs — список ID подключённых агентов
func (h *hermesAgentServer) GetConnectedAgentIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.streams))
	for id := range h.streams {
		ids = append(ids, id)
	}
	return ids
}

// ===== proto helpers =====

func taskStatusFromProto(s hermesagent.TaskStatus) string {
	switch s {
	case hermesagent.TaskStatus_TASK_SUCCESS:
		return "success"
	case hermesagent.TaskStatus_TASK_ERROR:
		return "error"
	case hermesagent.TaskStatus_TASK_TIMEOUT:
		return "timeout"
	case hermesagent.TaskStatus_TASK_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
	}
}
