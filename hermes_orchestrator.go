package main

// hermes_orchestrator.go — оркестратор AI агентов
// Анализирует запрос, маршрутизирует к агентам, агрегирует результаты

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/core/llm"
	"LavenderMessenger/core/llm/hermes"
	"LavenderMessenger/core/llm/openrouter"
	"LavenderMessenger/core/pipeline"
	"LavenderMessenger/core/rag"
	"LavenderMessenger/core/rag/memory"
	"LavenderMessenger/core/tools"
)

// OrchestratorSession — контекст диалога с оркестратором
type OrchestratorSession struct {
	mu            sync.Mutex
	UserID        string
	AgentID       string // ID текущего чата с оркестратором
	Messages      []OrchestratorMessage
	LastActivity  time.Time
	ActiveAgentID string // Какой агент сейчас активен (после маршрутизации)
}

type OrchestratorMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// Orchestrator — центральный оркестратор
type Orchestrator struct {
	registry  *HermesAgentRegistry
	sessions  map[string]*OrchestratorSession // key = userID
	mu        sync.RWMutex
	db        *sql.DB
	apiKey    string
	model     string

	// Remote Agent Manager
	remoteManager *RemoteAgentManager

	// ===== NEW: LLM Router + RAG Pipeline (Ports & Adapters) =====
	llmRouter   llm.LLMRouter       // маршрутизатор LLM-провайдеров
	ragPipeline rag.RAGPipeline     // RAG-пайплайн (in-memory с TF-IDF embeddings)
	aiPipeline  *pipeline.Pipeline  // полный пайплайн: RAG → LLM → Tools
}

func NewOrchestrator(registry *HermesAgentRegistry, db *sql.DB, apiKey, model string) *Orchestrator {
	o := &Orchestrator{
		registry:      registry,
		sessions:      make(map[string]*OrchestratorSession),
		db:            db,
		apiKey:        apiKey,
		model:         model,
		remoteManager: NewRemoteAgentManager(),
	}

	// ===== NEW: Initialize LLM Router with OpenRouter provider =====
	openRouterProvider := openrouter.NewProvider(apiKey, model)
	llmRouter := llm.NewSimpleRouter(openRouterProvider)
	llmRouter.Register(llm.RouteRule{
		ModelPrefix: "openrouter/",
		Provider:    openRouterProvider,
		Priority:    10,
	})
	// Register Hermes local provider
	hermesProvider, err := hermes.NewProvider("")
	if err == nil {
		llmRouter.Register(llm.RouteRule{
			ModelPrefix: "local/",
			Provider:    hermesProvider,
			Priority:    20,
		})
		log.Printf("[Orchestrator] Hermes local provider registered (prefix=local/)")
	} else {
		log.Printf("[Orchestrator] Hermes local provider not available: %v", err)
	}
	o.llmRouter = llmRouter

	// ===== NEW: Initialize RAG pipeline (in-memory with real TF-IDF embeddings) =====
	ragEmbedder := memory.NewInMemoryEmbeddingService(384)
	ragVectorDB := memory.NewInMemoryVectorDB(384)
	o.ragPipeline = memory.NewInMemoryRAGPipeline(ragEmbedder, ragVectorDB)

	// ===== NEW: Initialize AI Pipeline with Tool Executor =====
	toolExecutor := tools.NewDefaultToolExecutor(db)
	o.aiPipeline = pipeline.NewPipeline(llmRouter, o.ragPipeline, toolExecutor)

	log.Printf("[Orchestrator] LLM Router initialized with OpenRouter provider (model=%s)", model)
	log.Printf("[Orchestrator] RAG Pipeline initialized (in-memory, TF-IDF embeddings, dim=384)")
	log.Printf("[Orchestrator] Tool Executor initialized (search_messages, search_users, web_search, get_chat_info)")

	return o
}

// getOrCreateSession возвращает существующую сессию или создаёт новую
func (o *Orchestrator) getOrCreateSession(userID string) *OrchestratorSession {
	o.mu.Lock()
	defer o.mu.Unlock()

	if s, ok := o.sessions[userID]; ok {
		s.LastActivity = time.Now()
		return s
	}

	s := &OrchestratorSession{
		UserID:       userID,
		Messages:     make([]OrchestratorMessage, 0),
		LastActivity: time.Now(),
	}
	o.sessions[userID] = s

	// Persist session to DB
	if o.db != nil {
		_, _ = o.db.Exec(
			"INSERT INTO hermes_sessions (id, user_id, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET updated_at = NOW()",
			"hermes-"+userID, userID, "Лава ИИ",
		)
	}

	return s
}

// Orchestrate — главный метод: принимает запрос пользователя, возвращает ответ
func (o *Orchestrator) Orchestrate(ctx context.Context, userID, chatID, userMessage string, streamFn func(token string, finished bool) error) error {
	session := o.getOrCreateSession(userID)

	// Сохраняем сообщение пользователя
	session.mu.Lock()
	session.Messages = append(session.Messages, OrchestratorMessage{Role: "user", Content: userMessage})
	session.mu.Unlock()

	// Шаг 1: Анализ запроса — выбираем агента(ов)
	decision, err := o.analyzeRequest(ctx, session, userMessage)
	if err != nil {
		return fmt.Errorf("orchestrator analysis failed: %w", err)
	}

	log.Printf("[ORCHESTRATOR] user=%s decision: mode=%s agents=%v reason=%s",
		userID, decision.Mode, decision.AgentIDs, decision.Reason)

	// Шаг 1.5: Проверяем есть ли remote agents в решении
	// Если агент не найден в локальном реестре — ищем в remote agents
	resolvedAgentIDs := make([]string, 0, len(decision.AgentIDs))
	for _, agentID := range decision.AgentIDs {
		if o.registry.Get(agentID) != nil {
			resolvedAgentIDs = append(resolvedAgentIDs, agentID)
		} else if remoteAgent := o.remoteManager.GetAgent(agentID); remoteAgent != nil {
			// Remote agent найден — выполняем через него
			log.Printf("[ORCHESTRATOR] routing to remote agent: %s (%s)", agentID, remoteAgent.Name)
			return o.runRemoteAgent(ctx, session, remoteAgent, userMessage, streamFn)
		}
	}

	if len(resolvedAgentIDs) == 0 {
		// Ни локальных, ни remote агентов не найдено — fallback на hermes-owl
		log.Printf("[ORCHESTRATOR] no agents found, fallback to hermes-owl")
		resolvedAgentIDs = []string{"hermes-owl"}
	}
	decision.AgentIDs = resolvedAgentIDs

	// Шаг 2: Выполнение в зависимости от режима
	switch decision.Mode {
	case "single":
		return o.runSingleAgent(ctx, session, decision.AgentIDs[0], userMessage, streamFn)
	case "parallel":
		return o.runParallelAgents(ctx, session, decision.AgentIDs, userMessage, streamFn)
	case "pipeline":
		return o.runPipelineAgents(ctx, session, decision.AgentIDs, userMessage, streamFn)
	default:
		return o.runSingleAgent(ctx, session, "hermes-owl", userMessage, streamFn)
	}
}

// analyzeRequest — LLM анализирует запрос и выбирает агента(ов)
func (o *Orchestrator) analyzeRequest(ctx context.Context, session *OrchestratorSession, userMessage string) (*RoutingDecision, error) {
	// Формируем промпт для оркестратора
	agentList := o.buildAgentListPrompt()

	orchestratorPrompt := fmt.Sprintf(`Ты — оркестратор AI агентов в мессенджере Lavender.
Твоя задача: проанализировать запрос пользователя и выбрать оптимального агента(ов).

Доступные агенты:
%s

Режимы выполнения:
- single: один агент справится
- parallel: несколько агентов работают параллельно (для комплексных вопросов)
- pipeline: цепочка агентов (output одного → input следующего)

Ответь СТРОГО в JSON формате:
{"mode": "single|parallel|pipeline", "agents": ["agent-id"], "reason": "краткое объяснение"}

Запрос пользователя: "%s"`, agentList, userMessage)

	// Вызываем OpenRouter для анализа
	messages := []map[string]string{
		{"role": "system", "content": orchestratorPrompt},
		{"role": "user", "content": userMessage},
	}

	response, err := callOpenRouterContext(ctx, o.apiKey, o.model, orchestratorPrompt, messages)
	if err != nil {
		// Fallback: используем OWL для всех запросов
		log.Printf("[ORCHESTRATOR] analysis error, fallback to OWL: %v", err)
		return &RoutingDecision{
			Mode:     "single",
			AgentIDs: []string{"hermes-owl"},
			Reason:   "fallback: analysis error",
		}, nil
	}

	// Парсим JSON ответ
	decision := o.parseRoutingDecision(response)
	return decision, nil
}

// buildAgentListPrompt формирует список агентов для промпта оркестратора
func (o *Orchestrator) buildAgentListPrompt() string {
	var sb strings.Builder
	for _, agent := range o.registry.GetAll() {
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", agent.ID, agent.Name, agent.Description))
	}
	return sb.String()
}

// parseRoutingDecision парсит JSON ответ оркестратора
func (o *Orchestrator) parseRoutingDecision(response string) *RoutingDecision {
	// Ищем JSON в ответе
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return &RoutingDecision{Mode: "single", AgentIDs: []string{"hermes-owl"}, Reason: "parse error"}
	}

	jsonStr := response[start:end+1]

	var parsed struct {
		Mode    string   `json:"mode"`
		Agents  []string `json:"agents"`
		Reason  string   `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return &RoutingDecision{Mode: "single", AgentIDs: []string{"hermes-owl"}, Reason: "json parse error"}
	}

	// Валидация: проверяем что агенты существуют
	validAgents := make([]string, 0)
	for _, id := range parsed.Agents {
		if o.registry.Get(id) != nil {
			validAgents = append(validAgents, id)
		}
	}

	if len(validAgents) == 0 {
		validAgents = []string{"hermes-owl"}
	}

	// Ограничения
	if len(validAgents) > 3 {
		validAgents = validAgents[:3]
	}

	mode := parsed.Mode
	if mode != "single" && mode != "parallel" && mode != "pipeline" {
		mode = "single"
	}

	return &RoutingDecision{
		Mode:     mode,
		AgentIDs: validAgents,
		Reason:   parsed.Reason,
	}
}

// runSingleAgent — запускает одного агента и стримит ответ
func (o *Orchestrator) runSingleAgent(ctx context.Context, session *OrchestratorSession, agentID, userMessage string, streamFn func(token string, finished bool) error) error {
	agent := o.registry.Get(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	session.mu.Lock()
	session.ActiveAgentID = agentID
	session.mu.Unlock()

	log.Printf("[ORCHESTRATOR] running single agent: %s (%s)", agentID, agent.Name)

	// Стримим информацию о выбранном агенте
	intro := fmt.Sprintf("[%s] ", agent.Name)
	if err := streamFn(intro, false); err != nil {
		return err
	}

	// Вызываем агента через streamOpenRouter с callback
	model := agent.Model
	if model == "" {
		model = o.model
	}
	agentHistory := o.buildAgentHistory(session)

	var fullResponse strings.Builder
	err := streamOpenRouter(ctx, o.apiKey, model, agent.SystemPrompt, agentHistory, func(token string, finished bool) error {
		if finished {
			if fullResponse.Len() > 0 {
				o.saveAgentMessage(session, agentID, fullResponse.String())
			}
			return nil
		}
		fullResponse.WriteString(token)
		return streamFn(token, false)
	})
	if err != nil {
		return fmt.Errorf("agent %s error: %w", agentID, err)
	}
	if fullResponse.Len() > 0 {
		o.saveAgentMessage(session, agentID, fullResponse.String())
		_ = streamFn("", true)
	}
	return nil
}

// runParallelAgents — запускает нескольких агентов параллельно
func (o *Orchestrator) runParallelAgents(ctx context.Context, session *OrchestratorSession, agentIDs []string, userMessage string, streamFn func(token string, finished bool) error) error {
	log.Printf("[ORCHESTRATOR] running parallel agents: %v", agentIDs)

	type agentResult struct {
		AgentID  string
		Response string
		Error    error
	}

	results := make([]agentResult, len(agentIDs))
	var wg sync.WaitGroup

	for i, agentID := range agentIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			agent := o.registry.Get(id)
			if agent == nil {
				results[idx] = agentResult{AgentID: id, Error: fmt.Errorf("not found")}
				return
			}

			model := agent.Model
			if model == "" {
				model = o.model
			}

			agentHistory := o.buildAgentHistory(session)
			var full strings.Builder
			if err := streamOpenRouter(ctx, o.apiKey, model, agent.SystemPrompt, agentHistory, func(token string, finished bool) error {
				if finished {
					return nil
				}
				full.WriteString(token)
				return nil
			}); err != nil {
				results[idx] = agentResult{AgentID: id, Error: err, Response: full.String()}
				return
			}
			results[idx] = agentResult{AgentID: id, Response: full.String()}
		}(i, agentID)
	}

	wg.Wait()

	// Агрегируем результаты
	var aggregated strings.Builder
	for _, r := range results {
		agent := o.registry.Get(r.AgentID)
		agentName := r.AgentID
		if agent != nil {
			agentName = agent.Name
		}

		if r.Error != nil {
			aggregated.WriteString(fmt.Sprintf("\n⚠️ %s: ошибка — %v\n", agentName, r.Error))
			continue
		}

		o.saveAgentMessage(session, r.AgentID, r.Response)
		aggregated.WriteString(fmt.Sprintf("\n## %s\n%s\n", agentName, r.Response))
	}

	// Стримим агрегированный ответ по чанкам
	words := strings.Fields(aggregated.String())
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}
		isLast := i == len(words)-1
		if err := streamFn(chunk, isLast); err != nil {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}

// runPipelineAgents — цепочка агентов: output N → input N+1
func (o *Orchestrator) runPipelineAgents(ctx context.Context, session *OrchestratorSession, agentIDs []string, userMessage string, streamFn func(token string, finished bool) error) error {
	log.Printf("[ORCHESTRATOR] running pipeline agents: %v", agentIDs)

	currentInput := userMessage

	for _, agentID := range agentIDs {
		agent := o.registry.Get(agentID)
		if agent == nil {
			continue
		}

		model := agent.Model
		if model == "" {
			model = o.model
		}

		// Стримим заголовок агента
		header := fmt.Sprintf("\n→ %s: ", agent.Name)
		if err := streamFn(header, false); err != nil {
			return nil
		}

		agentHistory := []map[string]string{
			{"role": "user", "content": currentInput},
		}

		var full strings.Builder
		err := streamOpenRouter(ctx, o.apiKey, model, agent.SystemPrompt, agentHistory, func(token string, finished bool) error {
			if finished {
				return nil
			}
			full.WriteString(token)
			return streamFn(token, false)
		})
		if err != nil {
			return fmt.Errorf("pipeline agent %s error: %w", agentID, err)
		}
		if full.Len() > 0 {
			o.saveAgentMessage(session, agentID, full.String())
			currentInput = full.String()
		}
	}

	_ = streamFn("", true)
	return nil
}

// buildAgentHistory формирует историю сообщений для агента
func (o *Orchestrator) buildAgentHistory(session *OrchestratorSession) []map[string]string {
	session.mu.Lock()
	defer session.mu.Unlock()

	history := make([]map[string]string, 0, len(session.Messages))
	for _, msg := range session.Messages {
		history = append(history, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return history
}

// saveAgentMessage сохраняет сообщение агента в сессию
func (o *Orchestrator) saveAgentMessage(session *OrchestratorSession, agentID, content string) {
	session.mu.Lock()
	defer session.mu.Unlock()

	session.Messages = append(session.Messages, OrchestratorMessage{
		Role:    "assistant",
		Content: fmt.Sprintf("[%s] %s", agentID, content),
	})
}

// ===== NEW: ProcessWithPipeline — обработка через RAG + LLM Pipeline =====

// ProcessWithPipeline обрабатывает запрос через новый пайплайн:
// RAG (эмбеддинг → векторный поиск → контекст) → LLM (стриминг) → Tool Calls
func (o *Orchestrator) ProcessWithPipeline(
	ctx context.Context,
	userID string,
	userMessage string,
	images [][]byte,
	onChunk func(token string, finished bool) error,
) error {
	if o.aiPipeline == nil {
		return fmt.Errorf("AI pipeline not initialized")
	}

	// Получаем историю из сессии
	session := o.getOrCreateSession(userID)
	session.mu.Lock()
	history := make([]llm.Message, 0, len(session.Messages))
	for _, m := range session.Messages {
		history = append(history, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	session.mu.Unlock()

	// Сохраняем сообщение пользователя
	session.mu.Lock()
	session.Messages = append(session.Messages, OrchestratorMessage{Role: "user", Content: userMessage})
	session.mu.Unlock()

	// Запускаем пайплайн
	var fullResponse strings.Builder
	err := o.aiPipeline.ProcessRequest(ctx, "", userMessage, images, history, func(chunk llm.StreamChunk) error {
		if chunk.Content != "" {
			fullResponse.WriteString(chunk.Content)
		}
		return onChunk(chunk.Content, chunk.Done)
	})

	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	// Сохраняем ответ ассистента
	if fullResponse.Len() > 0 {
		session.mu.Lock()
		session.Messages = append(session.Messages, OrchestratorMessage{
			Role:    "assistant",
			Content: fullResponse.String(),
		})
		session.mu.Unlock()
	}

	return nil
}

// runRemoteAgent — выполняет запрос через удалённый агент
func (o *Orchestrator) runRemoteAgent(
	ctx context.Context,
	session *OrchestratorSession,
	agent *RemoteAgent,
	userMessage string,
	streamFn func(token string, finished bool) error,
) error {
	log.Printf("[ORCHESTRATOR] runRemoteAgent: agent=%s (%s) host=%s", agent.ID, agent.Name, agent.Host)

	// Информируем пользователя о маршрутизации
	intro := fmt.Sprintf("→ [%s@%s] ", agent.Name, agent.Host)
	if err := streamFn(intro, false); err != nil {
		return err
	}

	// Создаём задачу для remote agent
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	task := &RemoteTask{
		ID:         taskID,
		AgentID:    agent.ID,
		Type:       "shell",
		Params:     map[string]string{"command": userMessage},
		TimeoutSec: 120,
	}

	// Отправляем задачу через RemoteAgentManager
	if err := o.remoteManager.SendTask(task); err != nil {
		errMsg := fmt.Sprintf("\n⚠️ Remote agent error: %v\n", err)
		streamFn(errMsg, false)
		return err
	}

	// Ждём результат
	result := o.remoteManager.WaitForResult(taskID, 2*time.Minute)
	if result == nil {
		streamFn("\n⚠️ Remote agent: no result\n", false)
		return fmt.Errorf("remote agent %s: no result", agent.ID)
	}

	// Стримим результат
	if result.Status == "success" {
		// Стримим stdout по чанкам
		lines := strings.Split(result.Stdout, "\n")
		for _, line := range lines {
			if line != "" {
				streamFn(line+"\n", false)
			}
		}
	} else {
		errMsg := fmt.Sprintf("\n⚠️ Remote agent error: %s\n", result.Error)
		streamFn(errMsg, false)
	}

	streamFn("", true)

	// Сохраняем ответ в сессию
	output := result.Stdout
	if result.Status != "success" {
		output = fmt.Sprintf("[remote:%s] error: %s", agent.ID, result.Error)
	}
	session.mu.Lock()
	session.Messages = append(session.Messages, OrchestratorMessage{
		Role:    "assistant",
		Content: output,
	})
	session.mu.Unlock()

	return nil
}

// GetLLMRouter возвращает LLM Router для регистрации дополнительных провайдеров
func (o *Orchestrator) GetLLMRouter() llm.LLMRouter {
	return o.llmRouter
}

// GetRAGPipeline возвращает RAG pipeline для загрузки данных
func (o *Orchestrator) GetRAGPipeline() rag.RAGPipeline {
	return o.ragPipeline
}
