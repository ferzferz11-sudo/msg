# Промпт для новой сессии — v1.2.0.1

**Дата:** 2026-06-14
**Версия:** v1.2.0.1
**Ветка:** feat/1.1.3.x

---

## СТАТУС: v1.2.0.1 — DEV + PROD

Сервер: v1.2.0.1 на dev (порт 50052, HTTP 8083) и prod (порт 50051, HTTP 8082).
Android: v1.1.3.12 — BearerTokenInterceptor + proactive refresh + per-server validation.
Тестирование пройдено на dev и prod.

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.0.1", service version constants
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

### i18n
- Все строки в values/strings.xml (en) + values-ru/strings.xml
- app_version_format: "Lava: app Android %s" / "Lava: приложение Android %s"

---

## ПРАВИЛА

1. НЕ компилировать на сервере (OOM kill)
2. Коммитить и пушить после каждого значимого изменения
3. Версия сервера в server.go:33, версия Android в version.txt
4. userId (UUID) — всегда как ключ, НЕ username
5. changelog.txt БОЛЬШЕ НЕ ИСПОЛЬЗУЕТСЯ
6. JWT секрет: минимум 32 байта, НЕ коммитить
7. Темы: цвета программно через ThemeUtils.parseSafeColor()
8. i18n: все новые строки ОДНОВРЕМЕННО в values/strings.xml + values-ru/strings.xml
9. НЕ инициализировать getString() в полях класса Activity
10. Форматирование строк: позиционные форматтеры (%1$s, %2$d)
11. НЕ деплоить на prod без тестирования на dev
12. Перед деплоем на prod: проверить JWT_SECRET ≥32 bytes, .env не пустой, go build успешен

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

---

## ИЗВЕСТНЫЕ ПРОБЛЕМЫ

### 42P10 на prod БД (НЕ исправлена)
- `pq: there is no unique or exclusion constraint matching the ON CONFLICT specification`
- Таблица `user_devices` на prod создана до добавления UNIQUE constraint
- Нужно вручную выполнить на prod БД:
  ```sql
  ALTER TABLE user_devices 
  ADD CONSTRAINT user_devices_user_id_device_id_key 
  UNIQUE (user_id, device_id);
  ```
- Не критично — аутентификация работает

### Первый вход на prod — только Favorites (Android)
- Проблема в локальном кеше — после очистки всё ОК
- Не является багом нового кода
