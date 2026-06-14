# Лава — Задачи

**Версия:** v1.2.0.1
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-14 (сессия 6)

---

## ✅ v1.2.0.1 — AuthService v2 (JWT) + Server info + UI fixes

### Сервер
- ✅ **ServerVersion обновлён до v1.2.0.1** (server.go:33)
- ✅ **Service version constants** — AuthServiceVersion="2.0", ChatServiceVersion="1.0", etc.
- ✅ **Endpoint `/info`** — возвращает версии сервисов для client capability negotiation
- ✅ **APP_ENV support** — загрузка `.env.<APP_ENV>` (.env.dev при APP_ENV=dev)
- ✅ **Systemd: Environment=APP_ENV=dev** вместо EnvironmentFile
- ✅ **Dev server endpoint** — `GET http://host:8083/info` работает на dev

### Android
- ✅ **AuthV2 integration** — SessionManager.loginV2() с fallback на v1
- ✅ **JWT token storage** — AuthManager.storeTokens(), getAccessToken(), getRefreshToken(), getBearerToken()
- ✅ **UserSession** — accessToken, refreshToken, authMethod, isJwtAuth
- ✅ **Toolbar flickering fix** — isConnecting flag, единый поток загрузки
- ✅ **Logout сохраняет username** — last_username в legacy prefs
- ✅ **Предзаполнение username** — LoginBottomSheet.prefillUsername()
- ✅ **Убран диалог "Предложить регистрацию"** — Toast с реальной ошибкой
- ✅ **Cancel в login/register sheets** — закрывает шторку и возвращает к auth choice
- ✅ **Подавлены DEPRECATION warnings** — @Suppress("DEPRECATION") на loginV1 fallback

### Документация
- ✅ **Dev Server Management в PITFALLS.md** — systemd, .env, common issues
- ✅ **Обновлён INTEGRATION_SESSION.md** — v1.2.0.1
- ✅ **Обновлён INDEX.md**

---

## 📋 Бэклог

### Высокий приоритет
- [ ] **Тестирование JWT auth на dev** — регистрация, вход, refresh token, logout
- [ ] **Token refresh interceptor** — автоматический refresh при 401 от сервера
- [ ] **Подставить Bearer token во все gRPC вызовы** — getChats, getHistory, sendMessage, etc.
- [ ] **Протестировать server switch** — prod ↔ dev, проверить что токены не конфликтуют

### Средний приоритет
- [ ] **Тесты для /info endpoint** — unit-тесты для http_server.go
- [ ] **Обновить CHANGELOG.md** — сервер и Android
- [ ] **Проверить шторку профиля** — нет горизонтальной черты (divider)

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG)
- [ ] Prometheus метрики

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion, service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info)
messenger.proto            — ChatService, AuthService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService
```

### Android (Kotlin)
```
data/
├── proto/MessengerProto.kt       — Все proto data classes (AuthResponseV2Proto, etc.)
├── grpc/GrpcClient.kt             — Facade (signInV2, signUpV2, refreshToken)
├── grpc/RealGrpcClient.kt         — Реализация gRPC клиента
├── session/CredentialStore.kt     — Credentials + server list + last_username
├── session/SessionManager.kt      — loginV2 (JWT) + loginV1 (legacy fallback)
├── session/UserSession.kt         — accessToken, refreshToken, authMethod, isJwtAuth
├── auth/AuthManager.kt            — JWT token storage, getBearerToken, getAccessToken
└── models/ErrorHandler.kt         — Единый обработчик ошибок

ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt   — Шторка выбора входа
│   ├── LoginBottomSheet.kt        — Шторка входа (prefillUsername)
│   └── RegisterBottomSheet.kt     — Шторка регистрации
├── remote/                        — Remote Agent UI
├── chat/widget/ChatWidget.kt      — Общий виджет чата
└── adapter/ChatAdapter.kt         — Адаптер чатов
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.2.0.1 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.11+ |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |
