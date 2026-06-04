# Lavender Messenger — Задачи

**Последнее обновление:** 2026-06-04
**Версия:** 1.1.0.15
**Ветка:** main (server) / feat/1.1.0.x (android)

---

## 🔧 В процессе

### 1. Hermes Orchestrator — Tool Calling Loop
- **Статус:** требует доработки
- **Описание:** Pipeline имеет жёсткий лимит max 3 итерации tool calling. При активном function calling этого мало — нужна адаптивная логика.
- **Файлы:** `core/pipeline/pipeline.go`
- **Цель:** убрать жёсткий лимит, добавить проверку на завершение (LLM перестал вызывать tools → финализировать)

### 2. HermesAgentService — приём подключений от hermes-agent daemon
- **Статус:** не реализовано
- **Описание:** Оркестратор НЕ принимает подключения от hermes-agent daemon (bidirectional stream)
- **Файлы:** `hermes_agent_service.go`, `hermes_remote_manager.go`

### 3. RemoteAgentManager — SendTask()
- **Статус:** заглушка
- **Описание:** RemoteAgentManager.SendTask() не реализован
- **Файлы:** `hermes_remote_manager.go`

---

## ✅ Исправлено (v1.1.0.15)

### Hermes Orchestrator — полная архитектура
- LLM Router (OpenRouter + Hermes local provider)
- In-memory RAG (TF-IDF, 384 dim, cosine similarity, unit тесты)
- Tool Executor (search_messages, search_users, web_search, get_chat_info)
- Pipeline: RAG → LLM → Tool Calling loop
- gRPC: ChatWithPipeline(PipelineRequest) → stream PipelineResponse
- 8 агентов в реестре (7 пресетов + hermes-owl fallback)
- Dev сервер обновлён и работает

### Предыдущие версии (1.1.0.0 — 1.1.0.14)
- Все фиксы E2EE, favorites, OWL чатов, gRPC keepalive, TURN, звонков — в CHANGELOG.md

---

## 📋 Бэклог

### Высокий приоритет
- [ ] Tool calling loop — адаптивный лимит итераций
- [ ] HermesAgentService — bidirectional stream для hermes-agent daemon
- [ ] Auth токены для удалённых агентов

### Средний приоритет
- [ ] Qdrant + CLIP для production RAG (вместо in-memory TF-IDF)
- [ ] RemoteAgentManager.SendTask() — реальная реализация
- [ ] Кэширование OWL чатов в локальной БД (Android)

### Низкий приоритет
- [ ] Оптимизация списка моделей OWL (23 модели — кэшировать)
- [ ] Graceful reconnect при keepalive failed без потери сообщений
- [ ] Mac session logout issue

---

## 🔑 Ключевые решения

| Решение | Обоснование |
|---------|-------------|
| LLM Router с приоритетами | OpenRouter default (10), Hermes local (20) — fallback на локальный |
| In-memory RAG | Быстрый старт, замена на Qdrant+CLIP в production |
| Tool Executor через интерфейс | Легко добавлять новые инструменты |
| Pipeline max iterations | Временное решение — нужна адаптивная логика |
| OWL чаты в `chats` с `type='owl'` | Единая таблица |

---

## 📁 Ключевые файлы

| Файл | Назначение |
|------|------------|
| `/root/msg/hermes_orchestrator.go` | Оркестратор, LLM Router, RAG, Pipeline init |
| `/root/msg/core/pipeline/pipeline.go` | RAG → LLM → Tool Calling loop |
| `/root/msg/core/llm/provider.go` | LLMProvider, LLMRouter interfaces |
| `/root/msg/core/llm/openrouter/provider.go` | OpenRouter SSE provider |
| `/root/msg/core/llm/hermes/provider.go` | Hermes local provider (CLI wrapper) |
| `/root/msg/core/rag/memory/memory.go` | In-memory RAG (TF-IDF) |
| `/root/msg/core/tools/executor.go` | DefaultToolExecutor (4 инструмента) |
| `/root/msg/server.go` | gRPC хендлеры (~3550 строк) |
| `/root/msg/db.go` | PostgreSQL, миграции |
| `/root/msg/messenger.proto` | gRPC определения |
