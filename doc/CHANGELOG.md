# Lavender Messenger — Changelog (сервер)

## v1.1.2.3 — AI Chat Refactor
**Дата:** 2026-06-09

### Новое
- ai_chat_manager.go — единый менеджер AI чатов (sessions, messages, settings)
- Единые таблицы: ai_chat_sessions, ai_chat_messages, ai_chat_settings
- Proto: AIChatRequest, AIChatResponse, AIChatMessage, AIChatSettings
- RPC: ChatWithAI (streaming), GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings
- Миграция БД: FK CASCADE на все AI-таблицы

### Удалено
- Таблицы: owl_messages, owl_chat_settings, hermes_messages, hermes_sessions, hermes_chat_settings

### Deprecated
- ChatWithOWL, ChatWithOrchestrator (пометлены в proto, пока работают)

### Известные проблемы
- Hermes история не загружается (HermesChatActivity не мигрирован на ChatWithAI, старый ChatWithOrchestrator пишет в удалённую таблицу)
- Счётчик запросов показывает max 19 вместо 20 (off-by-one в rate limiter.remaining())

---

## v1.1.2.2 — DeleteChat cascade fix
**Дата:** 2026-06-09

- DeleteChat: каскадное удаление hermes_sessions + hermes_messages
- DeleteChat: каскадное удаление owl_messages + owl_chat_settings
- Полная очистка orphan-записей на dev и prod

## v1.1.2.1 — История из БД + счётчик запросов
**Дата:** 2026-06-09

- GetOrchestratorHistory загружает из hermes_messages БД
- GetOwlSettings/GetHermesSettings возвращают remaining/limit/window_seconds
- Rate limiter: метод remaining(userID)

## v1.1.2.0 — Prod Релиз
**Дата:** 2026-06-09

- Все баги AI чатов исправлены
- Log-monitor исправлен (JS split escape)
- ServerVersion 1.1.2.0
