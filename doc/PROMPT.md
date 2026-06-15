# Промпт для новой сессии — Server v1.2.1.0

**Дата:** 2026-06-14
**Ветка сервера:** feat/1.2.0.x
**Ветка Android:** feat/1.1.3.x

---

## СТАТУС: v1.2.1.0 — DEV / v1.1.3.13 — Android

Сервер: v1.2.1.0 на dev (порт 50052, HTTP 8083). Prod: v1.1.3.10.
Android: v1.1.3.13 — ProfileService v2 client + Typing/CallSession compat.

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
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
http_server.go             — HTTP (/health, /info на 8082/8083)
messenger.proto            — ChatService, AuthService, ProfileService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService
```

### Android (/root/msg.client.android)
```
ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt    — шторка выбора входа (лого + сервер + статус)
│   ├── LoginBottomSheet.kt         — шторка входа (prefillUsername)
│   └── RegisterBottomSheet.kt      — шторка регистрации
├── ServersActivity.kt              — управление списком серверов
├── remote/                         — Remote Agent UI
├── chat/widget/ChatWidget.kt       — общий виджет чата
└── adapter/ChatAdapter.kt          — адаптер чатов (clearAll)

data/
├── grpc/BearerTokenInterceptor.kt  — ClientInterceptor для JWT Bearer token
├── grpc/GrpcClient.kt              — фасад
├── grpc/RealGrpcClient.kt          — реализация gRPC
├── grpc/ProfileClient.kt           — ProfileService v2 client (JWT, dev only)
├── auth/AuthManager.kt             — JWT token storage, getBearerToken, needsRefresh
├── session/CredentialStore.kt      — credentials, jwt_server_address, last_username
├── session/SessionManager.kt       — loginV2 + loginV1 fallback, startTokenRefresh
├── session/UserSession.kt          — accessToken, refreshToken, authMethod
└── models/ErrorHandler.kt          — единый обработчик ошибок
```

---

## КЛЮЧЕВЫЕ РЕШЕНИЯ

### Сервер
- AuthService v2 (JWT) — основной метод аутентификации
- AuthService v1 — deprecated, но работает для совместимости
- gRPC Bearer token interceptor — валидация JWT на каждом вызове
- Device management (user_devices, device_auth_log)
- `/info` endpoint — версии сервисов для client capability negotiation
- APP_ENV — загрузка `.env.<APP_ENV>` для dev сервера
- Token rotation с обнаружением reuse
- **ProfileService v2** — отдельный сервис для профиля (JWT, dev only)
- **user_settings** — таблица для locale, theme_id, push_enabled
- **Typing/CallSession** — whitelist в AuthStreamInterceptor (v1 compat)

### Android
- BearerTokenInterceptor — автоматическая подстановка JWT Bearer token
- Proactive token refresh — каждые 60с, за 5 минут до истечения
- Per-server token validation — токены привязаны к серверу
- 3 auth виджета: ServerAuthBottomSheet, LoginBottomSheet, RegisterBottomSheet
- isLoadingChats предотвращает двойную загрузку
- startSync() останавливается при смене сервера
- LoginV2 с fallback на V1 при недоступности JWT
- Logout сохраняет username для предзаполнения
- Drag handle во всех шторках входа
- Status indicator — только кружок слева от названия сервера
- **ProfileClient** — ProfileService v2 client, fallback на v1
- **fetchServerInfo()** — определение версии сервера через /info при connect()

### i18n
- Все строки в values/strings.xml (en) + values-ru/strings.xml
- app_version_format: "Lava: app Android %s" / "Lava: приложение Android %s"

---

## ПРАВИЛА

1. НЕ компилировать на сервере (OOM kill) — это касается и Go и Android (./gradlew убивает всё по памяти, а на сервере крутится prod)
2. НЕ деплоить новую версию на prod без прямого указания ферзя
3. Коммитить и пушить после каждого значимого изменения
4. Версия сервера в server.go:33, версия Android в version.txt
5. userId (UUID) — всегда как ключ, НЕ username
6. changelog.txt БОЛЬШЕ НЕ ИСПОЛЬЗУЕТСЯ
7. JWT секрет: минимум 32 байта, НЕ коммитить
8. Темы: цвета программно через ThemeUtils.parseSafeColor()
9. i18n: все новые строки ОДНОВРЕМЕННО в values/strings.xml + values-ru/strings.xml
10. НЕ инициализировать getString() в полях класса Activity
11. Форматирование строк: позиционные форматтеры (%1$s, %2$d)
12. НЕ деплоить на prod без тестирования на dev
13. **ProfileService v2** — регистрировать только на dev (APP_ENV=dev). Prod использует legacy ChatService.
14. **Серверная ветка версий: 1.2.0.x**, Android: 1.1.3.x до релиза, потом 1.2.0.x
15. Вся разработка на dev сервере, проверка обратной совместимости на prod

---

## КОМАНДЫ

```bash
# === СЕРВЕР ===
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Сборка и деплой на dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod (НЕ делать без тестирования на dev!)
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...

# Логи
journalctl -u lavender-server-dev -f
journalctl -u lavender-server -f

# === ANDROID ===
cd /root/msg.client.android
# assembleRelease ТОЛЬКО локально!
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Имя | Lava Germany dev | Lava Germany |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Systemd | `Environment=APP_ENV=dev` | `Environment=APP_ENV=` (пусто) |
| ProfileService | v2 (JWT) | v1 (legacy ChatService) |
| Версия | v1.2.1.0 | v1.1.3.10 |

---

## ДОКУМЕНТАЦИЯ

- Индекс: `/root/msg/doc/INDEX.md`
- Сервер: `/root/msg/doc/INTEGRATION_SESSION.md`, `/root/msg/doc/TASKS.md`
- Android: `/root/msg.client.android/doc/TASKS.md`, `/root/msg.client.android/doc/PROMPT_ANDROID.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Remote Agent: `/root/msg.client.android/doc/REMOTE_AGENT.md`
- Паттерны: `/root/msg.client.android/doc/PATTERNS.md`
- Log Monitor: `/root/msg/doc/LOG_MONITOR.md`
- CHANGELOG: `/root/msg/CHANGELOG.md` (сервер), `/root/msg.client.android/CHANGELOG.md` (Android)
