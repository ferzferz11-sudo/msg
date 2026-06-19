package main

import (
	"context"
	"fmt"
	"time"

	"LavenderMessenger/gen"
	"github.com/google/uuid"
)

// ============================================================================
// server_remote.go — Remote Agent RPC handlers
//
// Все RPC методы связанные с Remote Agent:
//   - ListRemoteAgents      — список зарегистрированных агентов
//   - GetRemoteAgentStatus   — статус конкретного агента
//   - DeployAgentTask        — отправить задачу (unary, blocking)
//   - DeployAgentTaskStream  — стриминг результатов задачи
//   - GetAgentStream         — стриминг логов агента
//
// Принципы:
// 1. Каждый RPC метод — тонкая обёртка: валидация → делегирование менеджеру → ответ
// 2. Graceful degradation: ListRemoteAgents возвращает пустой список если менеджер недоступен
// 3. Логирование: каждая операция логируется с контекстом (taskID, agentID)
// ============================================================================

// remoteManager возвращает RemoteAgentManager из сервера.
func (s *server) remoteManager() *RemoteAgentManager {
	return s.remoteAgentManager
}

// ----------------------------------------------------------------------------
// ListRemoteAgents — список всех зарегистрированных удалённых агентов.
// Graceful: если менеджер недоступен — пустой список (не ошибка).
// ----------------------------------------------------------------------------
func (s *server) ListRemoteAgents(_ context.Context, _ *gen.ListRemoteAgentsRequest) (*gen.ListRemoteAgentsResponse, error) {
	mgr := s.remoteManager()
	if mgr == nil {
		logger.Info("[ListRemoteAgents] manager unavailable, returning empty list")
		return &gen.ListRemoteAgentsResponse{}, nil
	}

	agents := mgr.GetAllAgents()
	result := make([]*gen.RemoteAgentInfo, 0, len(agents))

	now := time.Now()
	for _, a := range agents {
		a.mu.RLock()
		info := &gen.RemoteAgentInfo{
			Id:            a.ID,
			Name:          a.Name,
			Host:          a.Host,
			IpAddress:     a.IPAddress,
			Os:            a.OS,
			Status:        a.Status,
			Capabilities:  a.Capabilities,
			ActiveTasks:   int32(a.ActiveTasks),
			LastHeartbeat: a.LastHeartbeat.Format(time.RFC3339),
		}
		a.mu.RUnlock()

		// Автоматически помечаем stale если heartbeat > 120с
		if a.Status == "connected" && now.Sub(a.LastHeartbeat) > 120*time.Second {
			info.Status = "stale"
		}

		result = append(result, info)
	}

	logger.Infof("[ListRemoteAgents] returned %d agents", len(result))
	return &gen.ListRemoteAgentsResponse{Agents: result}, nil
}

// ----------------------------------------------------------------------------
// GetRemoteAgentStatus — статус конкретного агента по ID.
// ----------------------------------------------------------------------------
func (s *server) GetRemoteAgentStatus(_ context.Context, req *gen.GetRemoteAgentStatusRequest) (*gen.GetRemoteAgentStatusResponse, error) {
	mgr := s.remoteManager()
	if mgr == nil {
		return &gen.GetRemoteAgentStatusResponse{Status: "unavailable"}, nil
	}

	if req.AgentId == "" {
		return &gen.GetRemoteAgentStatusResponse{Status: "invalid"}, nil
	}

	agent := mgr.GetAgent(req.AgentId)
	if agent == nil {
		return &gen.GetRemoteAgentStatusResponse{Status: "not_found"}, nil
	}

	agent.mu.RLock()
	defer agent.mu.RUnlock()

	return &gen.GetRemoteAgentStatusResponse{
		Status:        agent.Status,
		ActiveTasks:   int32(agent.ActiveTasks),
		LastHeartbeat: agent.LastHeartbeat.Format(time.RFC3339),
	}, nil
}

// ----------------------------------------------------------------------------
// DeployAgentTask — отправить задачу агенту (unary, blocking).
// ----------------------------------------------------------------------------
func (s *server) DeployAgentTask(_ context.Context, req *gen.DeployAgentTaskRequest) (*gen.DeployAgentTaskResponse, error) {
	mgr := s.remoteManager()
	if mgr == nil {
		return &gen.DeployAgentTaskResponse{
			Success: false,
			Error:   "remote agent manager not available",
		}, nil
	}

	if req.AgentId == "" {
		return &gen.DeployAgentTaskResponse{
			Success: false,
			Error:   "agent_id is required",
		}, nil
	}

	taskID := generateTaskID()
	logTask := fmt.Sprintf("[DeployAgentTask agent=%s task=%s type=%s]", req.AgentId, taskID, req.TaskType)

	// Логируем туннель если используется
	if req.TunnelMode != gen.TunnelMode_TUNNEL_NONE {
		logger.Infof("%s tunnel_mode=%v tunnel_host=%s local_port=%d",
			logTask, req.TunnelMode, req.TunnelHost, req.TunnelLocalPort)
	}

	// Проверяем что агент существует
	agent := mgr.GetAgent(req.AgentId)
	if agent == nil {
		return &gen.DeployAgentTaskResponse{
			Success: false,
			TaskId:  taskID,
			Error:   fmt.Sprintf("agent %s not found", req.AgentId),
		}, nil
	}

	task := &RemoteTask{
		ID:         taskID,
		AgentID:    req.AgentId,
		Type:       req.TaskType,
		Params:     req.Params,
		WorkingDir: req.WorkingDir,
		TimeoutSec: int(req.TimeoutSec),
	}

	if err := mgr.SendTask(task); err != nil {
		logger.Infof("%s send failed: %v", logTask, err)
		return &gen.DeployAgentTaskResponse{
			Success: false,
			TaskId:  taskID,
			Error:   err.Error(),
		}, nil
	}

	// Ждём результат
	timeout := time.Duration(task.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	result := mgr.WaitForResult(taskID, timeout)

	if result == nil {
		return &gen.DeployAgentTaskResponse{
			Success: true,
			TaskId:  taskID,
			Error:   "task sent but no result yet (timeout)",
		}, nil
	}

	return &gen.DeployAgentTaskResponse{
		Success:    result.Status == "success",
		TaskId:     taskID,
		Error:      result.Error,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExitCode:   int32(result.ExitCode),
		DurationMs: int64(result.Duration.Milliseconds()),
	}, nil
}

// ----------------------------------------------------------------------------
// DeployAgentTaskStream — отправить задачу агенту (server-side streaming).
// Стримит stdout/stderr/progress в реальном времени до завершения.
// ----------------------------------------------------------------------------
func (s *server) DeployAgentTaskStream(req *gen.DeployAgentTaskRequest, stream gen.ChatService_DeployAgentTaskStreamServer) error {
	mgr := s.remoteManager()
	if mgr == nil {
		return stream.Send(&gen.DeployAgentTaskStreamResponse{
			TaskId: "",
			Error:  "remote agent manager not available",
			Done:   true,
			Status: "failed",
		})
	}

	if req.AgentId == "" {
		return stream.Send(&gen.DeployAgentTaskStreamResponse{
			Error:  "agent_id is required",
			Done:   true,
			Status: "failed",
		})
	}

	taskID := generateTaskID()
	logTask := fmt.Sprintf("[DeployAgentTaskStream agent=%s task=%s]", req.AgentId, taskID)

	if req.TunnelMode != gen.TunnelMode_TUNNEL_NONE {
		logger.Infof("%s tunnel_mode=%v tunnel_host=%s local_port=%d",
			logTask, req.TunnelMode, req.TunnelHost, req.TunnelLocalPort)
	}

	// Канал для промежуточных обновлений
	streamCh := make(chan *RemoteTaskStreamUpdate, 64)
	var finalResult *RemoteTaskResult

	// Сохраняем старые callbacks для восстановления
	oldOnStream := mgr.onStream
	oldOnResult := mgr.onResult
	defer func() {
		mgr.onStream = oldOnStream
		mgr.onResult = oldOnResult
	}()

	// Callback для промежуточных обновлений
	mgr.onStream = func(agentID string, update *RemoteTaskStreamUpdate) {
		if update.TaskID == taskID {
			select {
			case streamCh <- update:
			default:
			}
		}
		// Пробрасываем в старый callback если был
		if oldOnStream != nil {
			oldOnStream(agentID, update)
		}
	}

	// Callback для финального результата
	mgr.onResult = func(agentID string, result *RemoteTaskResult) {
		if result.TaskID == taskID {
			finalResult = result
			close(streamCh)
		}
		// Пробрасываем в старый callback если был
		if oldOnResult != nil {
			oldOnResult(agentID, result)
		}
	}

	task := &RemoteTask{
		ID:           taskID,
		AgentID:      req.AgentId,
		Type:         req.GetTaskType(),
		Params:       req.GetParams(),
		WorkingDir:   req.GetWorkingDir(),
		TimeoutSec:   int(req.GetTimeoutSec()),
		StreamOutput: true,
	}

	if err := mgr.SendTask(task); err != nil {
		return stream.Send(&gen.DeployAgentTaskStreamResponse{
			TaskId: taskID,
			Error:  err.Error(),
			Done:   true,
			Status: "failed",
		})
	}

	// Подтверждаем приём задачи
	if err := stream.Send(&gen.DeployAgentTaskStreamResponse{
		TaskId:   taskID,
		Status:   "running",
		Progress: "Задача отправлена агенту...",
	}); err != nil {
		return err
	}

	// Стримим обновления от агента.
	// При done=True от агента — НЕ отправляем done=True клиенту сразу,
	// а ставим флаг streamDone и продолжаем слушать streamCh.
	// Канал закроется в onResult когда придёт TaskResult.
	// После этого отправляем один финальный done=True с полными данными.
	var streamDone bool

	for update := range streamCh {
		// Агент сигнализировал о завершении стрима (done=True).
		// TaskResult придёт отдельным сообщением через onResult.
		if update.Done || update.Status == "completed" || update.Status == "failed" ||
			update.Status == "timeout" || update.Status == "cancelled" {
			streamDone = true
			continue
		}

		if err := stream.Send(&gen.DeployAgentTaskStreamResponse{
			TaskId:      taskID,
			Status:      update.Status,
			Progress:    update.Progress,
			StdoutChunk: update.StdoutChunk,
			StderrChunk: update.StderrChunk,
		}); err != nil {
			logger.Errorf("%s send error: %v", logTask, err)
			return err
		}
	}

	// streamCh закрыт — onResult сработал, finalResult доступен.
	// Отправляем один финальный done=True с полными буферами из TaskResult.
	if streamDone && finalResult != nil {
		return stream.Send(&gen.DeployAgentTaskStreamResponse{
			TaskId:     taskID,
			Status:     finalResult.Status,
			Stdout:     finalResult.Stdout,
			Stderr:     finalResult.Stderr,
			ExitCode:   int32(finalResult.ExitCode),
			DurationMs: int64(finalResult.Duration.Milliseconds()),
			Error:      finalResult.Error,
			Done:       true,
		})
	}

	// Fallback: канал закрыт без done=true от агента, но результат есть
	if finalResult != nil {
		return stream.Send(&gen.DeployAgentTaskStreamResponse{
			TaskId:     taskID,
			Status:     finalResult.Status,
			Stdout:     finalResult.Stdout,
			Stderr:     finalResult.Stderr,
			ExitCode:   int32(finalResult.ExitCode),
			DurationMs: int64(finalResult.Duration.Milliseconds()),
			Error:      finalResult.Error,
			Done:       true,
		})
	}

	// Fallback: результат так и не пришёл (агент отключился, таймаут)
	return stream.Send(&gen.DeployAgentTaskStreamResponse{
		TaskId: taskID,
		Error:  "task cancelled or lost",
		Done:   true,
		Status: "cancelled",
	})
}

// ----------------------------------------------------------------------------
// Utilities
// ----------------------------------------------------------------------------

// generateTaskID — короткий уникальный ID задачи.
func generateTaskID() string {
	return uuid.New().String()[:12]
}
