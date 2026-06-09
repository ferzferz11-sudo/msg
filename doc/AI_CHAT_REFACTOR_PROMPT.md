# Промпт для следующей сессии — AI Chat Refactor (v1.1.2.3)

## Задача
Реализовать проект AI Chat Refactor из /root/msg/doc/AI_CHAT_REFACTOR.md

Цель: единообразная архитектура для OWL и Hermes AI чатов — общие таблицы, общий Go-менеджер, упрощённый proto и клиент.

## Текущее состояние
- v1.1.2.2 задеплоена на prod, таг выпущен
- БД dev и prod полностью очищены от AI-чатов (0 записей)
- Старые таблицы БД ещё существуют (owl_messages, owl_chat_settings, hermes_sessions, hermes_messages, hermes_chat_settings) — их нужно дропнуть и создать новые
- Проект детально расписан в /root/msg/doc/AI_CHAT_REFACTOR.md

## План работ (порядок важен)

### 1. SQL — новые таблицы (dev + prod)
Дропнуть старые AI-таблицы, создать новые:
- ai_chat_sessions (id PK -> chats.id, user_id, agent_type, model, system_prompt, active_agent_id, agent_mode)
- ai_chat_messages (id BIGSERIAL, session_id FK -> ai_chat_sessions.id CASCADE, role, content, agent_id)
- ai_chat_settings (session_id PK -> ai_chat_sessions.id CASCADE, user_api_key, model_override)
Индексы: по user_id+created_at, по session_id+created_at

### 2. Proto — новые сообщения (/root/msg/messenger.proto)
Добавить:
- AIChatRequest (user_id, session_id, message, agent_id)
- AIChatResponse (token, finished, error)
- AIChatMessage (role, content, agent_id, created_at)
- AIChatSettings (session_id, user_api_key, model, is_using_custom_key, remaining, limit, window_seconds)
- GetAIChatHistoryRequest/Response, GetAIChatSettingsRequest/Response, UpdateAIChatSettingsRequest/Response

RPC:
- rpc ChatWithAI(AIChatRequest) returns (stream AIChatResponse)
- rpc GetAIChatHistory(GetAIChatHistoryRequest) returns (GetAIChatHistoryResponse)
- rpc GetAIChatSettings(GetAIChatSettingsRequest) returns (AIChatSettings)
- rpc UpdateAIChatSettings(UpdateAIChatSettingsRequest) returns (UpdateAIChatSettingsResponse)

Старые RPC НЕ удалять — пометить deprecated в комментариях. ChatWithOWL и ChatWithOrchestrator пока оставить.

### 3. Go — ai_chat_manager.go (/root/msg/)
Новый файл, единый менеджер:
- CreateSession(userID, agentType) -> sessionID
- GetSession(sessionID) -> *AIChatSession
- GetSessionsByUser(userID) -> []*AIChatSession
- DeleteSession(sessionID)
- AddMessage(sessionID, role, content, agentID)
- GetHistory(sessionID, limit) -> []AIMessage
- GetSettings(sessionID) -> *AIChatSettings
- SaveSettings(sessionID, apiKey, model)

### 4. Go — рефакторинг server.go
- Новый handler ChatWithAI() — определяет тип сессии (owl/hermes), для hermes вызывает orchestrator, для owl — прямой вызов OpenRouter
- Для hermes: убрать getOrCreateSession из Orchestrator, использовать ai_chat_manager
- UpdateHermesSettings / GetHermesSettings — использовать ai_chat_manager
- ChatWithOWL и ChatWithOrchestrator оставить как deprecated wrappers к ChatWithAI
- DeleteChat: каскадное удаление через FK CASCADE (ai_chat_sessions -> ai_chat_messages, ai_chat_settings)

### 5. OwlChat и HermesChat — минимальные изменения
- owlSessionManager и hermesSettingsManager остаются для обратной совместимости
- GetOwlHistory, GetOwlSettings, UpdateOwlSettings — используют owlSessionManager (не трогаем)
- Новые методы Go используют ai_chat_manager

### 6. Proto gen
```bash
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
```

### 7. Android — AiChatGrpc.kt
Новый файл: AiChatGrpc.kt с методами:
- chatWithAI(sessionId, message, agentId)
- getAIChatHistory(sessionId, limit)
- getAIChatSettings(sessionId)
- updateAIChatSettings(sessionId, apiKey, model)

Старые OwlGrpc.kt и HermesGrpc.kt пока не удалять.

### 8. Сборка и деплой
- Go build + deploy dev
- Проверка что сервер поднялся (OWL и Hermes работают)
- compileDebugKotlin
- Deploy prod

## Важные правила
- assembleRelease НЕ запускать на сервере (OOM)
- compileDebugKotlin OK
- userId (UUID) — всегда как ключ
- creator_id (UUID) — для проверки владельца
- JSON — всегда через json.Marshal
- Коммитить после каждого шага, пушить в feat/1.1.2.x
- Обновлять документацию (INTEGRATION_SESSION.md, TASKS.md, CHANGELOG.md, PITFALLS.md)

## Версия
Текущая: v1.1.2.2
Следующая: v1.1.2.3 (после завершения refactor)

## Документация
Читать в начале сессии:
- /root/msg/doc/INDEX.md
- /root/msg/doc/AI_SERVICES.md
- /root/msg/doc/AI_CHAT_REFACTOR.md (проект)
- /root/msg/doc/INTEGRATION_SESSION.md
- /root/msg/doc/TASKS.md
- /root/msg/doc/PITFALLS.md
