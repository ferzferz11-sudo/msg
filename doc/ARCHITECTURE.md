# Lavender Messenger — Архитектура

**Дата:** 2026-06-14
**Версия сервера:** 1.1.3.10
**Версия клиента:** 1.1.3.10

---

## 1. Общая архитектура

```
┌─────────────────┐     gRPC (bidirectional)    ┌──────────────────┐
│                 │◄────────────────────────────►│                  │
│  Android Client │     gRPC (unary)             │    Go Server     │
│    (Kotlin)     │◄────────────────────────────►│    (main.go)     │
│                 │                              │                  │
└─────────────────┘                              └────────┬─────────┘
                                                          │
┌─────────────────┐                                       │
│  Remote Agent   │     gRPC (bidirectional)              │
│    (Python)     │◄────────────────────────────────────►│
└─────────────────┘                                       │
                                                          │
                                                 ┌────────┴────────┐
                                                 │    PostgreSQL   │
                                                 │      (DB)       │
                                                 └─────────────────┘
```

## 2. Порты

| Сервис | Порт | Протокол | Описание |
|--------|------|----------|----------|
| gRPC (prod) | 50051 | gRPC | Основной сервер |
| gRPC (dev) | 50052 | gRPC | Dev сервер |
| HTTP (prod) | 8081-8082 | HTTP | Файлы, аватары, /health |
| HTTP (dev) | 8083 | HTTP | Dev HTTP |
| Log Monitor | 8090 | HTTP | Логи (опционально) |

## 3. Структура сервера (Go)

### Пакеты

| Файл | Назначение |
|------|------------|
| `main.go` | Точка входа, gRPC + HTTP серверы |
| `logger.go` | Structured logging (logrus) |
| `server.go` | Основные gRPC хендлеры |
| `server_chat.go` | Авторизация, стриминг сообщений |
| `server_chats.go` | Чаты: создание, удаление, список |
| `server_users.go` | Пользователи: профиль, аватар |
| `server_messages.go` | Сообщения: история, реакции, редактирование |
| `server_profile.go` | Профиль: username, password, удаление |
| `server_push.php` | FCM push-уведомления |
| `server_contacts.go` | Контакты |
| `server_themes.go` | Темы оформления |
| `server_favorites.go` | Избранное |
| `server_drafts.go` | Черновики сообщений |
| `server_muted.go` | Отключённые чаты |
| `server_management.go` | Управление сервером |
| `server_ai.go` | AI: OWL, Hermes, оркестратор |
| `server_remote.go` | Remote Agent RPC |
| `hermes_orchestrator.go` | Оркестратор агентов |
| `hermes_agents.go` | Реестр агентов |
| `hermes_agent_service.go` | HermesAgentService gRPC |
| `hermes_remote_manager.go` | Менеджер Remote Agent |
| `owl.go` | OWL AI сессии |
| `db.go` | PostgreSQL, миграции |
| `db_hermes.go` | Миграции Hermes таблиц |
| `http_server.go` | HTTP сервер (файлы, загрузки) |

### Core пакеты

| Файл | Назначение |
|------|------------|
| `core/llm/provider.go` | LLM Router |
| `core/llm/openrouter/` | OpenRouter провайдер |
| `core/llm/hermes/` | Hermes провайдер |
| `core/pipeline/pipeline.go` | RAG → LLM → Tools pipeline |
| `core/rag/` | RAG интерфейсы + in-memory |
| `core/tools/` | Tool Executor |

## 4. Remote Agent

```
┌─────────────┐     gRPC bidirectional     ┌──────────────────┐
│   Python    │◄──────────────────────────►│   Go Server      │
│   Agent     │                            │   (server_remote)│
└─────────────┘                            └──────────────────┘

Протокол:
  Agent → Connect → AGENT_REGISTER
  Agent → AGENT_HEARTBEAT (каждые 30с)
  Agent → AGENT_TASK_STREAM_UPDATE (stdout/stderr chunks)
  Agent → AGENT_TASK_STREAM_UPDATE (done=True)
  Agent → AGENT_TASK_RESULT (финальный результат)
```

### Типы задач

| Тип | Описание |
|-----|----------|
| shell | Выполнение shell команд |
| git | Git операции |
| build | Сборка проекта |
| deploy | Деплой |
| file | Операции с файлами |
| docker | Docker операции |
| ai | AI задачи |

## 5. Технический стек

### Сервер
- Go 1.26
- gRPC + Protocol Buffers
- PostgreSQL (database/sql + pq)
- Firebase Cloud Messaging (push)
- logrus (structured logging)
- systemd, .env конфигурация
- JWT аутентификация для Remote Agent

### Клиент (Android)
- Kotlin, gRPC (protobuf-lite manual)
- Room Database, Firebase, WebRTC
- MVVM + StateFlow + ViewBinding
- Material Design 3

## 6. Логирование

Используется `logrus` (structured logging):

```bash
# Формат (по умолчанию text)
LOG_FORMAT=json   # JSON для production
LOG_FORMAT=text   # Читаемый для dev

# Уровень (по умолчанию info)
LOG_LEVEL=debug|info|warn|error
```

Пример JSON output:
```json
{"time":"2026-06-14T04:33:55","level":"info msg":"Listening clients at [::]:50052"}
```

## 7. Тесты

| Команда | Описание |
|---------|----------|
| `./scripts/run-tests.sh` | Все тесты |
| `./scripts/run-unit-tests.sh` | Unit-тесты |
| `./scripts/run-streaming-tests.sh` | Тесты стриминга |

## 8. Деплой

| Команда | Описание |
|---------|----------|
| `./scripts/deploy-dev.sh` | Деплой на dev |
| `./scripts/release.sh <version>` | Релиз |

## 9. Безопасность

- JWT токены для Remote Agent (SHA-256 хеш в БД)
- Grace period 30с для переподключения агентов
- E2EE для секретных чатов (AES-256-GCM)
- Rate limiting: OWL 10 req/min, Hermes 5 req/min
