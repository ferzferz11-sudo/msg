# Lavender Messenger — Server

**Author:** Pavel Davydov (ferz)  
**Version:** 1.4.0.5  
**Language:** Go 1.26

gRPC-сервер мессенджера «Лава» с end-to-end шифрованием, PostgreSQL, Firebase push-уведомлениями, системой компаний, стикеров и AI-ассистентами.

## Репозитории

| Репозиторий | Описание |
|-------------|----------|
| `ferzferz11-sudo/msg` | Сервер (этот репо) |
| `ferzferz11-sudo/msg.client.android` | Android-клиент |

---

## Быстрый старт

```bash
# 1. Склонировать
git clone https://github.com/ferzferz11-sudo/msg.git
cd msg

# 2. Настроить окружение
cp .env.example .env
# Заполнить DATABASE_URL, CHAT_SECRET_KEY, JWT_SECRET

# 3. Запустить
go run .

# Или собрать бинарник
go build -o lavender-server .
./lavender-server
```

**Если порт занят:**
```bash
lsof -ti:50051 | xargs kill -9 2>/dev/null; go run .
```

---

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                     gRPC Server (:50051)                     │
├─────────────┬──────────────┬──────────────┬─────────────────┤
│   ChatV2    │  CallSession │  Typing      │  150+ unary RPC │
│  (stream)   │   (stream)   │  (stream)    │                 │
└──────┬──────┴──────┬───────┴──────┬───────┴────────┬────────┘
       │             │              │                │
       ▼             ▼              ▼                ▼
┌──────────────────────────────────────────────────────────────┐
│                         Hub (connections)                     │
├──────────────────────────────────────────────────────────────┤
│                    PostgreSQL (messages, chats, users)        │
├──────────────────────────────────────────────────────────────┤
│  Firebase  │  AI Providers  │  Redis  │  HTTP (:8081/:8082)  │
└────────────┴────────────────┴─────────┴──────────────────────┘
```

**Протокол:** gRPC bidirectional streaming (ChatV2, CallSession, Typing) + unary RPCs  
**БД:** PostgreSQL с connection pooling, автомиграции при старте  
**Шифрование:** AES-256-GCM для сообщений, bcrypt для паролей, ECDH для E2EE  
**Push:** Firebase Cloud Messaging (FCM) с batch-отправкой и дебаунсингом  
**Keepalive:** ping 20s, timeout 20s, MaxConnectionAge 2h

---

## Ключевые возможности

### Мессенджинг
- Чаты: direct, group, secret (E2EE через ECDH + AES-256-GCM)
- Сообщения v2: текст, изображения, голосовые, файлы, реакции, ответы, редактирование, пересылка
- Избранное: сохранение сообщений, отправка в saved_messages
- Черновики: автосохранение_drafts
- Закрепление сообщений и чатов
- Поиск по сообщениям
- Read receipts (отметки прочтения)
- Typing indicators (печатает...)
- Self-destruct timer (автоудаление: 30s, 1m, 5m, 1h, 24h)
- Удаление сообщений с persistence (deleted_messages table)

### Звонки
- Аудио и видеозвонки через CallSession stream
- WebRTC signaling (SDP, ICE candidates)
- Конференции (multi-participant)
- Системные сообщения о звонках (📞/📹)

### Компании
- CRUD компаний с владельцем
- Должности (positions) с уровнями доступа
- Участники с ролями
- Company chats с access control
- Приглашения по кодам
- Многокомпанийная поддержка

### AI
- **OWL AI** — встроенный ассистент с streaming-ответами
- **Hermes Orchestrator** — multi-agent система с RAG pipeline
- **AI Agents v2** — создание, установка, маркетплейс агентов
- **Провайдеры:** OpenRouter, Hermes local, Mimo, Reve (изображения), MCP, WebSocket, Webhook
- **Инструменты:** поиск сообщений, поиск пользователей, web search, web fetch, query DB, get chat info

### Пользователи
- Регистрация/авторизация (SignUp/SignIn v2 с device management)
- Профили (аватары, био, статус)
- Контакты
- Настройки (темы, локаль, push-уведомления)
- Управление сессиями (мульт устройство)
- OIDC SSO провайдер

### Безопасность
- JWT аутентификация с refresh tokens
- Rate limiting (per-user, per-IP)
- CORS защита
- Admin panel (is_super_admin)
- Парольный сброс через email

---

## Стек технологий

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.26 |
| RPC | gRPC + Protocol Buffers |
| БД | PostgreSQL |
| Кэш | Redis |
| Push | Firebase Cloud Messaging |
| AI | OpenRouter, Hermes, Mimo |
| RAG | Qdrant (optional) |
| Шифрование | AES-256-GCM, bcrypt, ECDH |

---

## Структура проекта

```
msg/
├── main.go                          # Entry point: gRPC + HTTP серверы
├── server.go                        # Core server struct, version
├── server_chat.go                   # ChatV2 bidirectional stream
├── server_call.go                   # CallSession stream (WebRTC signaling)
├── server_messages_v2.go            # SendMessageV2, GetHistoryV2, DeleteMessageV2
├── server_favorites.go              # AddFavorite, GetFavorites, SaveFavoriteMessage
├── server_chats.go                  # CreateGroupChat, DeleteChat, UpdateChatName
├── server_contacts.go               # AddContact, RemoveContact, GetContacts
├── server_profile_v2.go             # UpdateProfile, DeleteProfile, GetUserSettings
├── server_users.go                  # GetAdminUserList, AdminUpdatePassword
├── server_company.go                # Company CRUD, members, positions
├── server_stickers.go               # Sticker packs, CRUD, marketplace
├── server_self_destruct.go          # Self-destruct timer logic
├── server_push.go                   # FCM push notifications, call messages
├── server_ai_v2.go                  # AI chat v2, agent management
├── auth_service.go                  # SignIn, SignUp, SignOut
├── auth_jwt.go                      # JWT generation, validation
├── auth_interceptor.go              # gRPC auth interceptors
├── hub.go                           # Connection hub (broadcast, register)
├── db.go                            # PostgreSQL connection, migrations
├── db_migrations.go                 # Schema migrations
├── db_messages_v2.go                # Messages v2 queries
├── db_chats.go                      # Chat queries
├── db_users.go                      # User queries, DeleteProfile
├── db_chatlist_v2.go                # Chat list v2 queries
├── crypto.go                        # AES-256-GCM, bcrypt
├── secret_chat.go                   # E2EE (ECDH + AES)
├── http_server.go                   # HTTP: /health, /uploads, /apk
├── rate_limiter.go                  # In-memory rate limiter
├── redis_rate_limiter.go            # Redis-based rate limiter
├── push_debounce.go                 # Push notification debouncer
├── email.go                         # Email (password reset)
├── bot_commands.go                  # Bot commands (/status, /deploy, /logs)
├── logger.go                        # Structured logging
│
├── ai_v2.go                         # AI chat v2 core
├── ai_router.go                     # Hybrid router (keywords + binding)
├── ai_agent_executor.go             # Agent execution loop
├── ai_provider.go                   # Provider interface
├── ai_provider_openrouter.go        # OpenRouter provider
├── ai_provider_hermes_acp.go        # Hermes ACP provider
├── ai_provider_mimo.go              # Mimo provider
├── ai_provider_reve.go              # Reve image generation
├── ai_tool.go                       # Tool interface
├── ai_tool_search_messages.go       # Search messages tool
├── ai_tool_web_search.go            # Web search tool
├── ai_tool_web_fetch.go             # Web fetch tool
│
├── messenger.proto                  # Основной gRPC proto
├── gen/                             # Сгенерированный Go код
│   ├── messenger.pb.go
│   └── messenger_grpc.pb.go
│
├── doc/                             # Документация
│   ├── ARCHITECTURE.md
│   ├── CLIENT_INTEGRATION.md
│   ├── TESTING.md
│   └── ...
│
├── scripts/                         # Скрипты деплоя и обслуживания
│   ├── deploy-dev-local.sh
│   └── deploy-prod-local.sh
│
├── CHANGELOG.md                     # История версий
├── .env.example                     # Шаблон конфигурации
├── go.mod / go.sum                  # Go модули
└── uploads/                         # Загруженные файлы
```

---

## gRPC API (выборочно)

### Аутентификация
| RPC | Описание |
|-----|----------|
| `SignUpV2` | Регистрация с device info |
| `SignInV2` | Авторизация с device info |
| `SignOut` | Выход, отзыв токена |
| `RefreshToken` | Обновление JWT |

### Чаты и сообщения
| RPC | Описание |
|-----|----------|
| `ChatV2` | Bidirectional stream (сообщения, typing, broadcast) |
| `SendMessageV2` | Отправка сообщения (текст, медиа, E2EE, saved_messages) |
| `GetHistoryV2` | История сообщений с cursor-based pagination |
| `DeleteMessageV2` | Удаление сообщений |
| `EditMessageV2` | Редактирование сообщений |
| `SetReactionV2` | Реакции на сообщения |
| `ClearRoomHistory` | Очистка всех сообщений в чате |
| `CreateGroupChat` | Создание группового чата |
| `CreateDirectChat` | Создание direct чата |
| `CreateSecretChat` | Создание E2EE чата |

### Звонки
| RPC | Описание |
|-----|----------|
| `CallSession` | Bidirectional stream для звонков |

### Избранное
| RPC | Описание |
|-----|----------|
| `AddFavorite` | Добавить в избранное |
| `RemoveFavorite` | Удалить из избранного |
| `GetFavorites` | Список избранного |
| `SaveFavoriteMessage` | Сохранить сообщение в избранное |

### Компании
| RPC | Описание |
|-----|----------|
| `CreateCompany` | Создать компанию |
| `CreatePosition` | Создать должность |
| `AddMember` | Добавить участника |
| `CreateCompanyChat` | Создать чат компании |
| `GenerateInviteCode` | Сгенерировать код приглашения |

### AI
| RPC | Описание |
|-----|----------|
| `ChatWithAIV2` | Чат с AI-агентом (streaming) |
| `CreateAIAgent` | Создать AI-агента |
| `ListAIAgents` | Список агентов (маркетплейс) |
| `InstallAIAgent` | Установить агента |

### Админ
| RPC | Описание |
|-----|----------|
| `GetAdminUserList` | Список пользователей (cursor pagination, поиск) |
| `GetAdminUserSessions` | Сессии пользователя |
| `AdminUpdatePassword` | Смена пароля админом |

---

## Конфигурация (.env)

```bash
# Сервер
SERVER_ADDRESS=0.0.0.0:50051          # gRPC listen address

# Безопасность
CHAT_SECRET_KEY=CHANGE_ME_32_CHARS!!  # AES-256 ключ (ровно 32 байта)
JWT_SECRET=CHANGE_ME_32_CHARS_OR_MORE # JWT signing key

# База данных
DATABASE_URL=postgres://USER:PASS@HOST:5432/DBNAME?sslmode=require

# HTTP серверы
HTTP_PORT=8082                        # Файлы, health check, uploads
APK_PORT=8081                         # APK distribution

# Redis (rate limiting)
REDIS_ADDR=localhost:6379

# Firebase (push)
FIREBASE_CREDENTIALS_PATH=/path/to/firebase-adminsdk.json

# AI (опционально)
OPENROUTER_API_KEY=sk-or-v1-...
OPENROUTER_MODEL=openrouter/owl-alpha

# Qdrant RAG (опционально)
QDRANT_URL=http://localhost:6333
OPENAI_API_KEY=sk-...
```

---

## Деплой

### Dev / Prod (из локальной машины)

```bash
# Dev
./scripts/deploy-dev-local.sh

# Prod (с автоматическим rollback при_failure)
./scripts/deploy-prod-local.sh
```

Скрипты делают:
1. Cross-compile для Linux (amd64)
2. SCP на сервер
3. Остановка systemd сервиса
4. Замена бинарника
5. Запуск + проверка health endpoint

### Серверы

| Сервер | IP | gRPC | HTTP | Роль |
|--------|-----|------|------|------|
| Dev | 13.140.25.249 | :50052 | :8083 | Разработка |
| Prod | 13.140.25.249 | :50051 | :8082 | Продакшен |

### Health check

```bash
curl http://HOST:8082/health
# {"status":"ok","version":"1.4.0.5","db_connected":true,"active_streams":3,"uptime_seconds":12345}
```

---

## Разработка

### Тесты

```bash
go test ./...
```

Покрытие:
- `crypto_test.go` — AES, bcrypt, reset tokens (22 теста)
- `ai_v2_test.go` — AI providers, tools, router, executor (76 тестов)
- `company_test.go` — company logic (9 тестов)
- `self_destruct_test.go` — self-destruct timer (6 тестов)
- `auth_jwt_test.go` — JWT validation
- `messages_v2_test.go` — message operations
- `reactions_test.go` — reactions

### Proto regeneration

После изменения `.proto` файлов:

```bash
export PATH=$PATH:/root/go/bin
protoc --go_out=gen --go_opt=paths=source_relative \
       --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
       messenger.proto
```

### Ветки

| Ветка | Назначение |
|-------|------------|
| `main` | Production |
| `feat/1.4.0.x` | Текущая разработка |

---

## Документация

| Файл | Описание |
|------|----------|
| [CHANGELOG.md](CHANGELOG.md) | История версий |
| [doc/ARCHITECTURE.md](doc/ARCHITECTURE.md) | Архитектура |
| [doc/CLIENT_INTEGRATION.md](doc/CLIENT_INTEGRATION.md) | Интеграция клиента |
| [doc/TESTING.md](doc/TESTING.md) | Тестирование |
| [doc/AI_SERVICES.md](doc/AI_SERVICES.md) | AI сервисы |
| [doc/TASKS.md](doc/TASKS.md) | Задачи и релизы |

---

## Лицензия

Proprietary. Все права защищены.
