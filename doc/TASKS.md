# Лава — Задачи

**Версия:** v1.2.1.0
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-15 (сессия 10)

---

## ✅ v1.2.1.0 — ProfileService v2 + Typing/CallSession compat

### Сервер
- ✅ **ServerVersion обновлён до v1.2.1.0** (server.go:33)
- ✅ **ProfileService v2** — отдельный gRPC сервис для профиля (JWT, dev only)
  - Методы: GetProfile, UpdateProfile, UpdateAvatar, DeleteProfile, GetUserSettings, UpdateUserSettings
  - Регистрируется ТОЛЬКО на dev (APP_ENV=dev)
  - ProfileServiceVersion = "2.0" в /info
- ✅ **user_settings таблица** — locale, theme_id, push_enabled, custom JSONB
- ✅ **Typing/CallSession whitelist** — добавлены в AuthStreamInterceptor (v1 compat)
- ✅ **ServerService** — зарегистрирован только на dev

### Android
- ✅ **ProfileClient** — ProfileService v2 client с JWT Bearer auth
  - Автоопределение версии сервера через /info (profile >= "2.0")
  - Fallback на legacy ChatService для prod
  - fetchServerInfo() вызывается автоматически при connect()
- ✅ **Proto data classes** для ProfileService v2 в MessengerProto.kt
- ✅ **GrpcClient facade** — isProfileV2Supported, profileServiceVersion
- ✅ **Typing/CallSession compat** — v1 клиенты работают без JWT

### Коммиты (сервер)
- `a989511` — feat: ProfileService v2 + Typing/CallSession interceptor whitelist

### Коммиты (Android)
- `dbbf266` — feat: ProfileService v2 client + Typing/CallSession compat
- `5990917` — docs: update session notes for v1.1.3.13
- `7782993` — fix: ProfileClient — use unaryCall consistently
- `73da2e1` — fix: use inline Marshaller objects in ProfileClient.unaryCall
- `d707fa8` — fix: add missing imports for ProfileV2 proto classes
- `1a73dee` — fix: suppress newInstance deprecation warning in ProfileClient

---

## ✅ v1.2.0.1 — AuthService v2 (JWT) + Server info + UI fixes

### Сервер
- ✅ **ServerVersion обновлён до v1.2.0.1** (server.go:33)
- ✅ **Service version constants** — AuthServiceVersion="2.0", ChatServiceVersion="1.0"
- ✅ **Endpoint `/info`** — версии сервисов для client capability negotiation
- ✅ **APP_ENV support** — загрузка `.env.<APP_ENV>`
- ✅ **AuthInterceptor** — gRPC Bearer token interceptor
- ✅ **AuthService v2** — SignInV2, SignUpV2, RefreshToken, SignOut, RevokeDevice, GetDevices
- ✅ **Token rotation** — обнаружение refresh token reuse
- ✅ **Device management** — user_devices, device_auth_log

### Android
- ✅ **BearerTokenInterceptor** — ClientInterceptor для JWT Bearer token
- ✅ **Proactive token refresh** — каждые 60с
- ✅ **Per-server token validation** — jwt_server_address в CredentialStore
- ✅ **AuthV2 integration** — SessionManager.loginV2() с fallback на v1
- ✅ **Auth bottom sheets cosmetics** — drag handle, status indicator
- ✅ **getChats() error callback** + **loadChats() timeout**

---

## 📋 Бэклог

### Высокий приоритет
- [ ] **ChatList v2** — новая версия списка чатов с улучшенным UI/UX

### Средний приоритет
- [ ] **Тесты для ProfileService v2** — unit-тесты (сервер + Android)
- [ ] **Тесты для /info endpoint** — unit-тесты для http_server.go
- [ ] **Bearer token в Chat stream** — вместо password в первом сообщении (v1.2.2.x, отложено)

### Отложено
- [ ] Редеплой prod сервера — только после выхода Android клиента
- [ ] Qdrant + CLIP (production RAG) — см. AI_SERVICES.md
- [ ] Prometheus метрики

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.1.0", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor (unary + streaming)
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц (включая user_settings)
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info)
messenger.proto            — ChatService, AuthService, ProfileService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService
```

### Android (Kotlin)
```
ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt   — Шторка выбора входа
│   ├── LoginBottomSheet.kt        — Шторка входа (prefillUsername)
│   └── RegisterBottomSheet.kt     — Шторка регистрации
├── ServersActivity.kt             — Управление списком серверов
├── remote/                        — Remote Agent UI
├── chat/widget/ChatWidget.kt      — Общий виджет чата
└── adapter/ChatAdapter.kt         — Адаптер чатов

data/
├── grpc/BearerTokenInterceptor.kt — ClientInterceptor для JWT
├── grpc/GrpcClient.kt             — Facade
├── grpc/RealGrpcClient.kt         — Реализация gRPC
├── grpc/ProfileClient.kt          — ProfileService v2 client (JWT, dev only)
├── auth/AuthManager.kt            — JWT token storage
├── session/CredentialStore.kt     — Credentials + jwt_server_address
├── session/SessionManager.kt      — loginV2 + loginV1 fallback
├── session/UserSession.kt         — accessToken, refreshToken, authMethod
└── models/ErrorHandler.kt         — Единый обработчик ошибок
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.2.1.0 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.13 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |
