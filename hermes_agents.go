package main

// hermes_agents.go — конфигурация AI агентов для Hermes Orchestrator
// Содержит: пресеты должностей, реестр агентов, CRUD для кастомных агентов

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ===== Типы =====

// AgentDefinition — описание одного AI агента
type AgentDefinition struct {
	ID           string // Уникальный идентификатор (напр. "hermes-dev")
	Name         string // Человекочитаемое имя
	Role         string // Должность/роль (Developer, DevOps, Architect, ...)
	Description  string // Описание для оркестратора — когда вызывать
	SystemPrompt string // Системный промпт агента
	Model        string // Модель OpenRouter (пустая = использовать дефолт)
	MaxTokens    int    // Максимум токенов ответа
	IsPreset     bool   // true = системный пресет, false = кастомный
	CreatedBy    string // user_id создателя (пусто = системный)
}

// AgentPreset — шаблон для быстрого создания агента
type AgentPreset struct {
	ID           string // ID пресета (напр. "preset-developer")
	Name         string // Имя по умолчанию
	Role         string // Должность
	Description  string // Описание для оркестратора
	SystemPrompt string // Базовый промпт (можно кастомизировать)
	Model        string // Рекомендуемая модель
	MaxTokens    int    // Лимит токенов
	Icon         string // Иконка для UI (emoji)
}

// HermesAgentRegistry — реестр всех доступных агентов
type HermesAgentRegistry struct {
	agents map[string]*AgentDefinition
	db     *sql.DB // для загрузки кастомных агентов из БД
}

// RoutingDecision — решение оркестратора: какой агент(ы) выбрать
type RoutingDecision struct {
	Mode     string   // "single", "parallel", "pipeline"
	AgentIDs []string // ID выбранных агентов
	Reason   string   // Объяснение выбора
}

// ===== ПРЕСЕТЫ ДОЛЖНОСТЕЙ =====

// GetAgentPresets возвращает все доступные пресеты
func GetAgentPresets() []*AgentPreset {
	return []*AgentPreset{
		{
			ID:          "preset-developer",
			Name:        "Developer",
			Role:        "Разработчик",
			Description: "Пишет, рефакторит, отлаживает код. Знает стек проекта. Используй для: баги, фичи, ревью кода, SQL, proto.",
			SystemPrompt: `Ты — разработчик проекта.
Пиши идиоматичный код. Используй стандартную библиотеку.
Для gRPC — google.golang.org/grpc + status codes.
Для DB — sql.DB через обёртку.
Краткие комментарии на русском.
Всегда проверяй код перед коммитом.`,
			Model:     "",
			MaxTokens: 4096,
			Icon:      "💻",
		},
		{
			ID:          "preset-devops",
			Name:        "DevOps",
			Role:        "DevOps инженер",
			Description: "Управляет сервером: деплой, мониторинг, логи, бэкапы. Используй для: перезапуск, логи, деплой, диагностика.",
			SystemPrompt: `Ты — DevOps инженер сервера.
Знаешь systemd, nginx, PostgreSQL, Docker.
Безопасность: не удалять файлы без необходимости.
Думай дважды перед destructive действиями.
Всегда делай backup перед изменениями.`,
			Model:     "",
			MaxTokens: 3072,
			Icon:      "🔧",
		},
		{
			ID:          "preset-architect",
			Name:        "Architect",
			Role:        "Архитектор",
			Description: "Проектирует системы, выбирает технологии, оценивает trade-offs. Используй для: архитектура, выбор подхода, планирование.",
			SystemPrompt: `Ты — ведущий архитектор.
Используй простые решения. Не добавляй сложность без необходимости.
Принципы: чистый код, разделение слоёв, минимум зависимостей.
Оценивай trade-offs: latency vs consistency, simplicity vs features.`,
			Model:     "",
			MaxTokens: 4096,
			Icon:      "🏗️",
		},
		{
			ID:          "preset-support",
			Name:        "Support",
			Role:        "Специалист поддержки",
			Description: "Помогает пользователям с вопросами о приложении. Дружелюбный и терпеливый. Используй для: вопросы пользователей, баги, настройки.",
			SystemPrompt: `Ты — специалист поддержки пользователей.
Отвечай дружелюбно, терпеливо, на русском языке.
Помогаешь с вопросами о приложении, настройками, функциями.
Если не знаешь ответ — честно скажи и предложи обратиться к разработчику.`,
			Model:     "",
			MaxTokens: 2048,
			Icon:      "🎧",
		},
		{
			ID:          "preset-qa",
			Name:        "QA Engineer",
			Role:        "Тестировщик",
			Description: "Тестирует функциональность, находит баги, пишет тест-кейсы. Используй для: тестирование, поиск багов, валидация.",
			SystemPrompt: `Ты — QA инженер.
Находи баги, пиши тест-кейсы, проверяй функциональность.
Внимателен к деталям. Проверяй edge cases.
Документируй найденные проблемы чётко и воспроизводимо.`,
			Model:     "",
			MaxTokens: 3072,
			Icon:      "🧪",
		},
		{
			ID:          "preset-analyst",
			Name:        "Analyst",
			Role:        "Аналитик",
			Description: "Анализирует данные, метрики, логи. Составляет отчёты. Используй для: анализ данных, метрики, отчёты, статистика.",
			SystemPrompt: `Ты — аналитик данных.
Анализируешь метрики, логи, статистику.
Составляешь понятные отчёты с выводами.
Используй данные для обоснования рекомендаций.`,
			Model:     "",
			MaxTokens: 3072,
			Icon:      "📊",
		},
		{
			ID:          "preset-security",
			Name:        "Security",
			Role:        "Специалист безопасности",
			Description: "Проверяет код и конфигурации на уязвимости. Используй для: security review, уязвимости, аудит.",
			SystemPrompt: `Ты — специалист по информационной безопасности.
Проверяешь код на уязвимости: SQL injection, XSS, CSRF, утечки данных.
Проверяешь конфигурации: права доступа, секреты, TLS.
Даёшь конкретные рекомендации по исправлению.`,
			Model:     "",
			MaxTokens: 3072,
			Icon:      "🔒",
		},
		{
			ID:          "preset-custom",
			Name:        "Custom Agent",
			Role:        "Кастомный агент",
			Description: "Агент с пользовательскими настройками. Создаётся через UI с выбором параметров.",
			SystemPrompt: `Ты — кастомный AI-ассистент.
Следуй инструкциям пользователя.
Будь полезен, дружелюбен и лаконичен.`,
			Model:     "",
			MaxTokens: 2048,
			Icon:      "🤖",
		},
	}
}

// GetPresetByID возвращает пресет по ID
func GetPresetByID(id string) *AgentPreset {
	for _, p := range GetAgentPresets() {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ===== РЕЕСТР АГЕНТОВ =====

func NewAgentRegistry(db *sql.DB) *HermesAgentRegistry {
	r := &HermesAgentRegistry{
		agents: make(map[string]*AgentDefinition),
		db:     db,
	}

	// Регистрируем системных агентов (пресеты)
	r.registerPresetAgents()

	// Регистрируем hermes-OWL (встроенный AI агент)
	r.Register(&AgentDefinition{
		ID:           "hermes-owl",
		Name:         "OWL AI",
		Role:         "AI Assistant",
		Description:  "Универсальный AI ассистент на базе OpenRouter. Используется как fallback при ошибках оркестратора.",
		SystemPrompt: "Ты — полезный AI ассистент. Отвечай на языке пользователя.",
		Model:        "",
		MaxTokens:    4096,
		IsPreset:     true,
		CreatedBy:    "",
	})

	// Загружаем кастомных агентов из БД
	r.loadCustomAgents()

	return r
}

// registerPresetAgents регистрирует агентов из пресетов
func (r *HermesAgentRegistry) registerPresetAgents() {
	presets := GetAgentPresets()
	for _, p := range presets {
		// Пропускаем "custom" — это шаблон, не агент
		if p.ID == "preset-custom" {
			continue
		}
		agentID := strings.TrimPrefix(p.ID, "preset-")
		r.Register(&AgentDefinition{
			ID:           "hermes-" + agentID,
			Name:         p.Name,
			Role:         p.Role,
			Description:  p.Description,
			SystemPrompt: p.SystemPrompt,
			Model:        p.Model,
			MaxTokens:    p.MaxTokens,
			IsPreset:     true,
			CreatedBy:    "",
		})
	}
}

// loadCustomAgents загружает кастомных агентов из БД
func (r *HermesAgentRegistry) loadCustomAgents() {
	if r.db == nil {
		return
	}

	rows, err := r.db.Query(
		`SELECT id, name, role, description, system_prompt, model, max_tokens, created_by
		 FROM hermes_custom_agents WHERE is_active = TRUE`,
	)
	if err != nil {
		log.Printf("[HermesRegistry] load custom agents error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var a AgentDefinition
		var model sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.Description, &a.SystemPrompt, &model, &a.MaxTokens, &a.CreatedBy); err == nil {
			if model.Valid {
				a.Model = model.String
			}
			a.IsPreset = false
			r.Register(&a)
			count++
		}
	}
	if count > 0 {
		log.Printf("[HermesRegistry] loaded %d custom agents from DB", count)
	}
}

// Register добавляет агента в реестр
func (r *HermesAgentRegistry) Register(agent *AgentDefinition) {
	r.agents[agent.ID] = agent
}

// Get возвращает агента по ID
func (r *HermesAgentRegistry) Get(id string) *AgentDefinition {
	return r.agents[id]
}

// GetAll возвращает всех агентов (для оркестратора)
func (r *HermesAgentRegistry) GetAll() []*AgentDefinition {
	result := make([]*AgentDefinition, 0, len(r.agents))
	for _, a := range r.agents {
		result = append(result, a)
	}
	return result
}

// GetByUserID возвращает агентов пользователя (кастомные + пресеты)
func (r *HermesAgentRegistry) GetByUserID(userID string) []*AgentDefinition {
	result := make([]*AgentDefinition, 0)
	for _, a := range r.agents {
		if a.IsPreset || a.CreatedBy == userID {
			result = append(result, a)
		}
	}
	return result
}

// ===== CRUD ДЛЯ КАСТОМНЫХ АГЕНТОВ =====

// CreateCustomAgent создаёт нового кастомного агента из пресета
func (r *HermesAgentRegistry) CreateCustomAgent(userID, presetID, customName, customPrompt, model string, maxTokens int) (*AgentDefinition, error) {
	preset := GetPresetByID(presetID)
	if preset == nil {
		return nil, fmt.Errorf("preset %s not found", presetID)
	}

	// Генерируем ID
	agentID := "custom-" + userID + "-" + presetID + "-" + fmt.Sprintf("%d", timeNow())

	name := customName
	if name == "" {
		name = preset.Name
	}

	prompt := customPrompt
	if prompt == "" {
		prompt = preset.SystemPrompt
	}

	if maxTokens <= 0 {
		maxTokens = preset.MaxTokens
	}

	agent := &AgentDefinition{
		ID:           agentID,
		Name:         name,
		Role:         preset.Role,
		Description:  preset.Description,
		SystemPrompt: prompt,
		Model:        model,
		MaxTokens:    maxTokens,
		IsPreset:     false,
		CreatedBy:    userID,
	}

	// Сохраняем в БД
	if r.db != nil {
		_, err := r.db.Exec(
			`INSERT INTO hermes_custom_agents (id, name, role, description, system_prompt, model, max_tokens, created_by, is_active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE)`,
			agent.ID, agent.Name, agent.Role, agent.Description, agent.SystemPrompt, agent.Model, agent.MaxTokens, agent.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("DB insert error: %w", err)
		}
	}

	r.Register(agent)
	return agent, nil
}

// UpdateCustomAgent обновляет кастомного агента
func (r *HermesAgentRegistry) UpdateCustomAgent(agentID, userID, name, prompt, model string, maxTokens int) error {
	agent := r.Get(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}
	if agent.CreatedBy != userID {
		return fmt.Errorf("not your agent")
	}

	if name != "" {
		agent.Name = name
	}
	if prompt != "" {
		agent.SystemPrompt = prompt
	}
	if model != "" {
		agent.Model = model
	}
	if maxTokens > 0 {
		agent.MaxTokens = maxTokens
	}

	_, err := r.db.Exec(
		`UPDATE hermes_custom_agents SET name=$1, system_prompt=$2, model=$3, max_tokens=$4 WHERE id=$5 AND created_by=$6`,
		agent.Name, agent.SystemPrompt, agent.Model, agent.MaxTokens, agentID, userID,
	)
	return err
}

// DeleteCustomAgent удаляет кастомного агента
func (r *HermesAgentRegistry) DeleteCustomAgent(agentID, userID string) error {
	agent := r.Get(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}
	if agent.CreatedBy != userID {
		return fmt.Errorf("not your agent")
	}

	_, err := r.db.Exec("UPDATE hermes_custom_agents SET is_active = FALSE WHERE id = $1 AND created_by = $2", agentID, userID)
	if err != nil {
		return err
	}

	delete(r.agents, agentID)
	return nil
}

// ===== ВСПОМОГАТЕЛЬНЫЕ =====

// AgentConfigJSON — сериализация агента для Android UI
func (a *AgentDefinition) ToJSON() string {
	data, _ := json.Marshal(map[string]interface{}{
		"id":          a.ID,
		"name":        a.Name,
		"role":        a.Role,
		"description": a.Description,
		"isPreset":    a.IsPreset,
		"createdBy":   a.CreatedBy,
	})
	return string(data)
}

// PresetToJSON — сериализация пресета для Android UI
func (p *AgentPreset) ToJSON() string {
	data, _ := json.Marshal(map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"role":        p.Role,
		"description": p.Description,
		"icon":        p.Icon,
		"maxTokens":   p.MaxTokens,
	})
	return string(data)
}

// timeNow — для генерации ID (unix timestamp)
func timeNow() int64 {
	return time.Now().Unix()
}
