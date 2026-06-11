# Лава — AI Chat Refactor Project

Документ проекта по рефакторингу структуры таблиц AI-чатов (OWL + Hermes).
Цель: единообразная, расширяемая архитектура для обоих типов чатов.

---

## Анализ текущих проблем

### 1. Разнородность таблиц
- `owl_messages` отдельно от `hermes_messages` — одинаковые данные, разные таблицы
- `owl_chat_settings` и `hermes_chat_settings` — дублирование схемы
- `hermes_sessions` — отдельная таблица, привязанная к сессии, не к чату
- `chats.agent_id` — добавлено специально для Hermes, не используется для OWL

### 2. Несогласованность ключей
- OWL: чат ID в `chats.id` = `owl-<uuid>`, сообщения ссылаются на `chat_id`
- Hermes: сессия в `hermes_sessions.id` = `hermes-<uuid>`, но чат в `chats.id` = `hermes-<uuid>` (тот же ID), но `active_agent_id` в `hermes_sessions`, а `agent_id` в `chats`
- `getOrCreateSession` создаёт сессию с `id = "hermes-" + userID`, а `CreateHermesSession` — с `id = "hermes-" + UUID` — два разных паттерна

### 3. DeleteChat не каскадирует
- Удаление из `chats` не удаляет из `hermes_sessions`/`hermes_messages`
- Удаление из `chats` не удаляет `owl_chat_settings` (FK есть, но owl_messages без FK CASCADE к settings)

### 4. Proto — дублирование типов
- `OwlHistoryMessage` и `HermesChatMessage` — идентичные структуры
- `GetOwlSettingsResponse` и `GetHermesSettingsResponse` — идентичные поля
- `CreateOwlChatRequest/Response` vs `CreateHermesSessionRequest/Response` — разные паттерны

### 5. Нет единого абстрактного "AI Chat"
- Нет общего интерфейса/сущности для AI-чата
- Каждый тип обрабатывается отдельным кодом во всех слоях

---

## Целевая архитектура

### Принципы
1. **Единая таблица `ai_chat_sessions`** для всех AI-чатов (OWL + Hermes)
2. **Единая таблица `ai_chat_messages`** для всех сообщений
3. **Единая таблица `ai_chat_settings`** для настроек (per-chat API key + model)
4. `chats` остаётся для UI-списка (метаданные), но AI-данные живут в своих таблицах
5. Чёткие FK с CASCADE — удаление чата удаляет всё

### ERD

```
chats (id, name, type='owl'|'hermes', creator_id, participants, last_message_text, created_at, ...)
  |
  |--- 1:1 ---> ai_chat_sessions (id (=chats.id), user_id, agent_type,
  |                                  model, system_prompt, active_agent_id,
  |                                  agent_mode, created_at, updated_at)
  |
  |--- 1:N ---> ai_chat_messages (id, session_id (=chats.id), role,
  |                                content, agent_id, created_at)
  |
  |--- 1:1 ---> ai_chat_settings (session_id (=chats.id), user_api_key,
                                   model_override, updated_at)
```

### SQL — новые таблицы

```sql
-- =============================================================================
-- AI Chat Sessions — единая таблица для всех AI-чатов
-- =============================================================================
CREATE TABLE IF NOT EXISTS ai_chat_sessions (
    id              VARCHAR(255) PRIMARY KEY,          -- совпадает с chats.id
    user_id         TEXT        NOT NULL,               -- UUID владельца
    agent_type      TEXT        NOT NULL DEFAULT 'owl', -- 'owl' | 'hermes'
    model           TEXT        DEFAULT '',             -- модель по умолчанию
    system_prompt   TEXT        DEFAULT '',             -- system prompt
    active_agent_id TEXT        DEFAULT '',             -- активный агент (для hermes)
    agent_mode      TEXT        DEFAULT 'single',       -- 'single'|'parallel'|'pipeline'
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT fk_ai_sessions_chat
        FOREIGN KEY (id) REFERENCES chats(id) ON DELETE CASCADE
);

CREATE INDEX idx_ai_chat_sessions_user ON ai_chat_sessions(user_id, created_at DESC);
CREATE INDEX idx_ai_chat_sessions_type ON ai_chat_sessions(agent_type);

-- =============================================================================
-- AI Chat Messages — единая таблица для всех сообщений
-- =============================================================================
CREATE TABLE IF NOT EXISTS ai_chat_messages (
    id              BIGSERIAL   PRIMARY KEY,
    session_id      VARCHAR(255) NOT NULL,              -- FK -> ai_chat_sessions.id
    role            TEXT        NOT NULL,               -- 'user'|'assistant'|'system'|'agent'
    content         TEXT        NOT NULL DEFAULT '',
    agent_id        TEXT        DEFAULT '',             -- ID агента (для hermes multi-agent)
    created_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT fk_ai_messages_session
        FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_ai_messages_session ON ai_chat_messages(session_id, created_at ASC);

-- =============================================================================
-- AI Chat Settings — per-chat настройки (API key, model override)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ai_chat_settings (
    session_id      VARCHAR(255) PRIMARY KEY,           -- FK -> ai_chat_sessions.id
    user_api_key    TEXT        DEFAULT '',
    model_override  TEXT        DEFAULT '',
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT fk_ai_settings_session
        FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE
);
```

### Миграция данных (если нужно)

Поскольку БД сейчас пустая (все AI-чаты очищены), миграция не нужна — просто дропаем старые таблицы и создаём новые.

---

## Изменения в proto

### Удаляемые (устаревшие, заменяются):
- `CreateHermesSessionRequest/Response` → заменяется `CreateAIChatRequest(type=hermes)`
- `DeleteHermesSessionRequest/Response` → через `DeleteChat` (уже есть)
- `HermesChatMessage` → заменяется `AIChatMessage`
- `OrchestratorRequest/Response` → заменяется `AIChatRequest/Response`
- `OWLRequest/Response` → тоже `AIChatRequest/Response` (с type=owl)

### Новые сообщения:
```protobuf
// AI Chat — unified message for both OWL and Hermes streaming
message AIChatRequest {
    string user_id = 1;
    string session_id = 2;  // empty = new chat
    string message = 3;
    string agent_id = 4;    // optional: force specific agent (hermes)
}

message AIChatResponse {
    string token = 1;
    bool finished = 2;
    string error = 3;
}

message AIChatMessage {
    string role = 1;
    string content = 2;
    string agent_id = 3;
    string created_at = 4;
}

message AIChatSettings {
    string session_id = 1;
    string user_api_key = 2;
    string model = 3;
    bool is_using_custom_key = 4;
    int32 remaining = 5;
    int32 limit = 6;
    int32 window_seconds = 7;
}

message GetAIChatHistoryRequest {
    string session_id = 1;
    string user_id = 2;
    int32 limit = 3;
}

message GetAIChatHistoryResponse {
    repeated AIChatMessage messages = 1;
}
```

### Упрощение RPC:
```
// Новые
rpc ChatWithAI(AIChatRequest) returns (stream AIChatResponse);
rpc GetAIChatHistory(GetAIChatHistoryRequest) returns (GetAIChatHistoryResponse);
rpc GetAIChatSettings(GetAIChatSettingsRequest) returns (AIChatSettings);
rpc UpdateAIChatSettings(UpdateAIChatSettingsRequest) returns (UpdateAIChatSettingsResponse);

// Остаются без изменений:
// ChatWithPipeline (RAG), ListAgents, CreateAgent, etc.
```

---

## Go — рефакторинг

### Новая структура:
```
ai_chat.go          — общий интерфейс + unified streaming
ai_chat_manager.go  — DB-backed sessions, messages, settings
owl.go              → удаляется, функционал в ai_chat + system_prompt по умолчанию
hermes_orchestrator.go → упрощается (убираем getOrCreateSession, используем ai_chat_manager)
server.go           → ChatWithAI вместо ChatWithOWL + ChatWithOrchestrator
```

### ai_chat_manager:
```go
type AIChatManager struct {
    db *sql.DB
}

func (m *AIChatManager) CreateSession(userID, agentType string) (sessionID string, err error)
func (m *AIChatManager) GetSession(sessionID string) (*AIChatSession, error)
func (m *AIChatManager) GetSessionsByUser(userID string) ([]*AIChatSession, error)
func (m *AIChatManager) DeleteSession(sessionID string) error
func (m *AIChatManager) AddMessage(sessionID, role, content, agentID string) error
func (m *AIChatManager) GetHistory(sessionID string, limit int) ([]AIMessage, error)
func (m *AIChatManager) GetSettings(sessionID string) (*AIChatSettings, error)
func (m *AIChatManager) SaveSettings(sessionID, apiKey, model string) error
```

---

## Android — рефакторинг

### Новые классы:
```
AiChatGrpc.kt       — единый файл (вместо OwlGrpc + HermesGrpc)
AiChatActivity.kt   — единая Activity ( routing на основе type)
AiChatViewModel.kt  — единая ViewModel
```

### Старые:
```
OwlGrpc.kt          → удаляется (или deprecated wrapper)
HermesGrpc.kt       → удаляется
OwlChatActivity.kt  → deprecated wrapper → AiChatActivity
HermesChatActivity.kt → deprecated wrapper → AiChatActivity
OwlChatViewModel.kt → удаляется (или наследник AiChatViewModel)
HermesChatViewModel → удаляется
```

---

## План работ

1. SQL: дропать старые таблицы, создавать новые (БД пустая)
2. Proto: добавить новые сообщения, старые пометить deprecated
3. Go: ai_chat_manager.go, упрощение server.go
4. Proto gen
5. Android: AiChatGrpc.kt, AiChatActivity.kt, AiChatViewModel.kt
6. Сборка, деплой dev, тестирование
7. Деплой prod

---

## Версия

Это часть v1.1.2.x — начинаем с v1.1.2.3
