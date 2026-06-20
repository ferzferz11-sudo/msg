package main

// hermes_remote_manager.go — менеджер удалённых агентов на стороне оркестратора
// Управляет подключениями, отправляет задачи, собирает результаты

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	hermesagent "LavenderMessenger/gen/hermes_agent"
)

// RemoteAgent — информация о подключённом удалённом агенте
type RemoteAgent struct {
	ID            string
	Name          string
	Version       string
	Host          string
	IPAddress     string
	OS            string
	Capabilities  []string // shell, git, build, deploy, file, docker
	Status        string   // "connected", "disconnected", "busy", "error"
	LastHeartbeat time.Time
	ActiveTasks   int
	MaxConcurrent int

	mu sync.RWMutex
}

// RemoteTask — задача для удалённого агента
type RemoteTask struct {
	ID           string
	AgentID      string
	Type         string // shell, file_read, file_write, git, build, deploy, docker, custom
	Params       map[string]string
	WorkingDir   string
	TimeoutSec   int
	StreamOutput bool

	// Резульtат
	Result      *RemoteTaskResult
	Done        chan struct{}
	CreatedAt   time.Time
	CompletedAt time.Time
}

// RemoteTaskResult — результат выполнения задачи на удалённом агенте
type RemoteTaskResult struct {
	TaskID   string
	Status   string // "success", "error", "timeout", "cancelled"
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Error    string
}

// RemoteTaskStreamUpdate — промежуточное обновление задачи (для streaming)
type RemoteTaskStreamUpdate struct {
	TaskID      string
	StdoutChunk string // промежуточный stdout
	StderrChunk string // промежуточный stderr
	Progress    string // progress message
	Status      string // "running", "completed", "failed", "timeout", "cancelled"
	Done        bool   // true = последнее сообщение
	Stdout      string // полный stdout (при Done)
	Stderr      string // полный stderr (при Done)
	ExitCode    int32  // финальный exit code (при Done)
	DurationMs  int64  // финальная длительность в мс (при Done)
	Error       string // финальная ошибка (при Done)
}

// RemoteAgentManager — менеджер всех удалённых агентов
type RemoteAgentManager struct {
	agents map[string]*RemoteAgent // key = agent_id
	mu     sync.RWMutex

	// gRPC streams от hermesAgentServer
	streams   map[string]*agentStream
	streamsMu sync.RWMutex

	// Очередь задач (для балансировки)
	taskQueue chan *RemoteTask

	// Callbacks
	onResult func(agentID string, result *RemoteTaskResult)
	onStream func(agentID string, stream *RemoteTaskStreamUpdate) // для real-time вывода

	// Карта task_id → *RemoteTask для результатов
	pendingTasks map[string]*RemoteTask
	pendingMu    sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRemoteAgentManager() *RemoteAgentManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &RemoteAgentManager{
		agents:       make(map[string]*RemoteAgent),
		streams:      make(map[string]*agentStream),
		taskQueue:    make(chan *RemoteTask, 256),
		pendingTasks: make(map[string]*RemoteTask),
		ctx:          ctx,
		cancel:       cancel,
	}
	go m.processTaskQueue()
	go m.healthCheckLoop()
	return m
}

// Shutdown stops background goroutines and closes the task queue
func (m *RemoteAgentManager) Shutdown() {
	m.cancel()
	close(m.taskQueue)
}

// RegisterAgent регистрирует нового удалённого агента
func (m *RemoteAgentManager) RegisterAgent(info *RemoteAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info.Status = "connected"
	info.LastHeartbeat = time.Now()
	info.ActiveTasks = 0
	info.MaxConcurrent = 3 // по умолчанию

	m.agents[info.ID] = info

	logger.Infof("[RemoteAgent] registered: id=%s name=%s host=%s caps=%v",
		info.ID, info.Name, info.Host, info.Capabilities)
}

// UnregisterAgent отключает агента
func (m *RemoteAgentManager) UnregisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, ok := m.agents[agentID]; ok {
		agent.Status = "disconnected"
		delete(m.agents, agentID)
		logger.Infof("[RemoteAgent] unregistered: id=%s", agentID)
	}
}

// GetAgent возвращает агента по ID
func (m *RemoteAgentManager) GetAgent(agentID string) *RemoteAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[agentID]
}

// GetAvailableAgents возвращает список доступных агентов с учётом capabilities
func (m *RemoteAgentManager) GetCapabilities(capability string) []*RemoteAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RemoteAgent
	for _, agent := range m.agents {
		if agent.Status != "connected" {
			continue
		}
		for _, cap := range agent.Capabilities {
			if cap == capability {
				result = append(result, agent)
				break
			}
		}
	}
	return result
}

// GetAllAgents возвращает всех зарегистрированных агентов
func (m *RemoteAgentManager) GetAllAgents() []*RemoteAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RemoteAgent, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

// SetStream сохраняет stream для агента (вызывается из hermesAgentServer)
func (m *RemoteAgentManager) SetStream(agentID string, stream *agentStream) {
	m.streamsMu.Lock()
	defer m.streamsMu.Unlock()
	m.streams[agentID] = stream
}

// GetStream возвращает stream агента
func (m *RemoteAgentManager) GetStream(agentID string) *agentStream {
	m.streamsMu.RLock()
	defer m.streamsMu.RUnlock()
	return m.streams[agentID]
}

// HandleTaskResult — вызывается из hermesAgentServer когда агент присылает результат
func (m *RemoteAgentManager) HandleTaskResult(agentID string, result *RemoteTaskResult) {
	m.pendingMu.Lock()
	task, ok := m.pendingTasks[result.TaskID]
	if ok {
		delete(m.pendingTasks, result.TaskID)
	}
	m.pendingMu.Unlock()

	if ok && task != nil {
		task.Result = result
		task.CompletedAt = time.Now()
		close(task.Done)
	}

	if m.onResult != nil {
		go m.onResult(agentID, result)
	}

	// Обновляем счётчик активных задач
	agent := m.GetAgent(agentID)
	if agent != nil {
		agent.mu.Lock()
		if agent.ActiveTasks > 0 {
			agent.ActiveTasks--
		}
		agent.mu.Unlock()
	}
}

// HandleTaskStream — промежуточное обновление от агента (stdout/stderr/progress)
func (m *RemoteAgentManager) HandleTaskStream(agentID string, update *RemoteTaskStreamUpdate) {
	if m.onStream != nil {
		m.onStream(agentID, update)
	}
}

// SendTask отправляет задачу на удалённый агент через gRPC stream
func (m *RemoteAgentManager) SendTask(task *RemoteTask) error {
	agent := m.GetAgent(task.AgentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found or disconnected", task.AgentID)
	}

	agent.mu.RLock()
	status := agent.Status
	activeTasks := agent.ActiveTasks
	maxConcurrent := agent.MaxConcurrent
	agent.mu.RUnlock()

	if status != "connected" {
		return fmt.Errorf("agent %s is %s", task.AgentID, status)
	}
	if activeTasks >= maxConcurrent {
		return fmt.Errorf("agent %s is busy (%d/%d tasks)", task.AgentID, activeTasks, maxConcurrent)
	}

	task.Done = make(chan struct{})
	task.CreatedAt = time.Now()

	// Регистрируем задачу в pending
	m.pendingMu.Lock()
	m.pendingTasks[task.ID] = task
	m.pendingMu.Unlock()

	// Отправляем через gRPC stream
	m.streamsMu.RLock()
	stream, ok := m.streams[task.AgentID]
	m.streamsMu.RUnlock()

	if !ok {
		m.pendingMu.Lock()
		delete(m.pendingTasks, task.ID)
		m.pendingMu.Unlock()
		return fmt.Errorf("agent %s has no active stream", task.AgentID)
	}

	pt := &hermesagent.Task{
		TaskId:       task.ID,
		TaskType:     taskTypeToProto(task.Type),
		Params:       task.Params,
		WorkingDir:   task.WorkingDir,
		TimeoutSec:   int32(task.TimeoutSec),
		StreamOutput: task.StreamOutput,
	}
	payload, err := proto.Marshal(pt)
	if err != nil {
		m.pendingMu.Lock()
		delete(m.pendingTasks, task.ID)
		m.pendingMu.Unlock()
		return fmt.Errorf("marshal task: %w", err)
	}

	if err := stream.send(&hermesagent.OrchestratorMessage{
		TargetAgentId: task.AgentID,
		Type:          hermesagent.OrchestratorMessageType_ORCHESTRATOR_TASK,
		Payload:       payload,
		TimestampMs:   time.Now().UnixMilli(),
	}); err != nil {
		m.pendingMu.Lock()
		delete(m.pendingTasks, task.ID)
		m.pendingMu.Unlock()
		return fmt.Errorf("send task: %w", err)
	}

	agent.mu.Lock()
	agent.ActiveTasks++
	agent.mu.Unlock()

	logger.Infof("[RemoteAgent] task sent: agent=%s task=%s type=%s", task.AgentID, task.ID, task.Type)
	return nil
}

// WaitForTaskResult ждёт результат задачи и возвращает его через callback
func (m *RemoteAgentManager) WaitForTaskResult(taskID string, timeout time.Duration, callback func(result *RemoteTaskResult)) {
	go func() {
		m.pendingMu.Lock()
		task, ok := m.pendingTasks[taskID]
		m.pendingMu.Unlock()

		if !ok {
			callback(&RemoteTaskResult{TaskID: taskID, Status: "error", Error: "task not found"})
			return
		}

		select {
		case <-task.Done:
			callback(task.Result)
		case <-time.After(timeout):
			m.pendingMu.Lock()
			delete(m.pendingTasks, taskID)
			m.pendingMu.Unlock()
			callback(&RemoteTaskResult{TaskID: taskID, Status: "timeout", Error: "wait timeout"})
		}
	}()
}

// SubscribeTaskResults — server-side streaming: подписывается на результаты задач для агента
// Используется клиентом для получения результатов в реальном времени

// WaitForResult ждёт результат выполнения задачи (blocking, для оркестратора)
func (m *RemoteAgentManager) WaitForResult(taskID string, timeout time.Duration) *RemoteTaskResult {
	m.pendingMu.Lock()
	task, ok := m.pendingTasks[taskID]
	m.pendingMu.Unlock()

	if !ok {
		return nil
	}

	select {
	case <-task.Done:
		return task.Result
	case <-time.After(timeout):
		m.pendingMu.Lock()
		delete(m.pendingTasks, taskID)
		m.pendingMu.Unlock()
		return &RemoteTaskResult{TaskID: taskID, Status: "timeout", Error: "wait timeout"}
	}
}

// processTaskQueue обрабатывает очередь задач (балансировка)
func (m *RemoteAgentManager) processTaskQueue() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case task, ok := <-m.taskQueue:
			if !ok {
				return
			}
			agent := m.GetAgent(task.AgentID)
			if agent == nil {
				capability := taskToCapability(task.Type)
				available := m.GetCapabilities(capability)
				if len(available) > 0 {
					task.AgentID = available[0].ID
					agent = available[0]
				}
			}
			if agent != nil {
				if err := m.SendTask(task); err != nil {
					logger.Errorf("[RemoteAgent] task queue error: %v", err)
				}
			}
		}
	}
}

// healthCheckLoop проверяет здоровье агентов каждые 30 секунд
func (m *RemoteAgentManager) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			agents := make([]*RemoteAgent, 0, len(m.agents))
			for _, a := range m.agents {
				agents = append(agents, a)
			}
			m.mu.RUnlock()

			for _, agent := range agents {
				if time.Since(agent.LastHeartbeat) > 90*time.Second {
					logger.Infof("[RemoteAgent] heartbeat timeout: id=%s", agent.ID)
					m.UnregisterAgent(agent.ID)
				}
			}
		}
	}
}

// taskToCapability определяет нужную capability по типу задачи
func taskToCapability(taskType string) string {
	switch taskType {
	case "shell":
		return "shell"
	case "file_read", "file_write":
		return "file"
	case "git":
		return "git"
	case "build":
		return "build"
	case "deploy":
		return "deploy"
	case "docker":
		return "docker"
	default:
		return "shell"
	}
}

// GetAgentStatusJSON — статус всех агентов для Android UI
func (m *RemoteAgentManager) GetAgentStatusJSON() string {
	type AgentStatus struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Host          string   `json:"host"`
		Status        string   `json:"status"`
		Capabilities  []string `json:"capabilities"`
		ActiveTasks   int      `json:"active_tasks"`
		LastHeartbeat string   `json:"last_heartbeat"`
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]AgentStatus, 0, len(m.agents))
	for _, a := range m.agents {
		a.mu.RLock()
		statuses = append(statuses, AgentStatus{
			ID:            a.ID,
			Name:          a.Name,
			Host:          a.Host,
			Status:        a.Status,
			Capabilities:  a.Capabilities,
			ActiveTasks:   a.ActiveTasks,
			LastHeartbeat: a.LastHeartbeat.Format(time.RFC3339),
		})
		a.mu.RUnlock()
	}

	data, _ := json.Marshal(statuses)
	return string(data)
}

// taskTypeToProto converts task type string to proto enum
func taskTypeToProto(t string) hermesagent.TaskType {
	switch t {
	case "shell":
		return hermesagent.TaskType_TASK_SHELL
	case "file_read":
		return hermesagent.TaskType_TASK_FILE_READ
	case "file_write":
		return hermesagent.TaskType_TASK_FILE_WRITE
	case "git":
		return hermesagent.TaskType_TASK_GIT
	case "build":
		return hermesagent.TaskType_TASK_BUILD
	case "deploy":
		return hermesagent.TaskType_TASK_DEPLOY
	case "docker":
		return hermesagent.TaskType_TASK_DOCKER
	default:
		return hermesagent.TaskType_TASK_SHELL
	}
}
