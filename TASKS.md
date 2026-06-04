# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.12
**Обновлено:** 2026-06-04
**Статус:** ✅ Unified Chat Widget + OWL удалён + Документация создана

---

## ✅ Сделано (v1.1.0.12)

### Документация
- `HERMES_ORCHESTRATOR_DOC.md` — полная документация по оркестратору (архитектура, компоненты, код, деплой)
- `HERMES_ORCHESTRATOR_PROMPT.md` — обновлённый промт для новой сессии

### Android — Unified Chat Widget
- widget_chat.xml, item_chat_message.xml, ChatMessageAdapter, ChatWidget.kt
- HermesChatActivity: агенты как участники (MaterialChip emoji + name)
- HermesChatViewModel: agents registry

### Android — OWL удалён (-2425 строк)
- OwlActivity, OwlGrpc, OwlMessageAdapter, все OWL layouts/drawables/menu/proto

### Сервер
- GRANT ALL PRIVILEGES для lavender user на hermes-таблицы
- RemoteAgentManager.SendTask() — полная реализация
- runRemoteAgent() — интеграция в оркестратор

---

## ⏳ Не начато (по приоритету)

1. **Auth токены для удалённых агентов** — генерация JWT при регистрации, валидация при каждом запросе
2. **Qdrant + CLIP** — production RAG (вместо in-memory TF-IDF)
3. **NewChatActivity** — адаптировать для использования ChatWidget (рефакторинг, низкий приоритет)
4. **Graceful reconnect** при keepalive failed

---

## Следующий шаг

Auth токены для удалённых агентов — реализовать JWT генерацию при регистрации агента, валидацию в каждом gRPC-вызове.
