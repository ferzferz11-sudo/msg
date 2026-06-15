# Лава — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.2.0.1
**Ветка:** feat/1.2.0.x
**Тег:** v1.1.3.10 (stable)

---

## Контекст

- Сервер: `/root/msg`, dev порт 50052, prod порт 50051
- Android: `/root/msg.client.android`
- Remote Agent: `/root/msg.remote.agent`
- Оба репозитория на ветке `feat/1.2.0.x`

---

## Что сделано (v1.2.0.1)

### Сервер
- ✅ **AuthService v2 (JWT)** — SignInV2, SignUpV2, RefreshToken, SignOut, RevokeDevice, GetDevices
- ✅ **AuthInterceptor** — gRPC Bearer token interceptor (streaming + unary)
- ✅ **/info endpoint** — версии сервисов для client capability negotiation
- ✅ **APP_ENV support** — загрузка `.env.<APP_ENV>` для dev сервера
- ✅ **Device management** — user_devices, device_auth_log таблицы
- ✅ **Token rotation** — обнаружение refresh token reuse
- ✅ **ServerVersion** = "1.2.0.1" (server.go:33)
- ✅ Миграция UNIQUE constraint на user_devices (db_auth_devices.go)

### Android
- ✅ **BearerTokenInterceptor** — автоматическая подстановка JWT Bearer token в gRPC
- ✅ **Proactive token refresh** — проверка каждые 60с, refresh за 5 минут до истечения
- ✅ **Per-server token validation** — токены привязаны к серверу, очистка при смене
- ✅ **3 auth видьеты** — ServerAuthBottomSheet, LoginBottomSheet, RegisterBottomSheet
- ✅ **Server switch** — корректная смена prod/dev серверов
- ✅ **i18n** — values/strings.xml (en) + values-ru/strings.xml

---

## Архитектура

```
┌─────────────┐  gRPC          ┌──────────────┐
│  Android    │ ──────────────→ │   Server     │
│  Client     │  Bearer token   │   (Go)       │
│  v1.1.3.12  │  (JWT v2)       │   v1.2.0.1   │
└─────────────┘                 └──────────────┘
```

**gRPC сервисы:**
- `messenger.ChatService` — Chat (streaming), GetChats, GetHistory, SendMessage, etc.
- `messenger.AuthService` — SignInV2, SignUpV2, RefreshToken, SignOut, RevokeDevice, GetDevices (v2, JWT)
- `messenger.AuthService` — SignIn, SignUp (v1, deprecated)
- `hermes_agent.HermesAgentService` — Connect, GenerateAgentToken, etc.

**Auth flow (v2):**
```
Client → /info → services.auth >= "2.0" → JWT workflow
  → SignInV2 → access_token + refresh_token
  → Bearer token в metadata для всех последующих вызовов
  → Proactive refresh за 5 минут до истечения

Client → /info = 404 или auth < "2.0" → Legacy workflow
  → Chat stream с password в первом сообщении
  → BearerTokenInterceptor = no-op (нет токена)
```

**Порты:**
- 50051 — prod
- 50052 — dev

---

## Критические файлы

### Сервер
- `server.go` — ServerVersion, service version constants, InitDB
- `auth_service_v2.go` — SignInV2, SignUpV2, RefreshToken, SignOut, RevokeDevice, GetDevices
- `auth_interceptor.go` — AuthInterceptor (unary), AuthStreamInterceptor (streaming)
- `auth_jwt.go` — GenerateTokenPair, ValidateToken, ExtractJTI
- `db_auth_devices.go` — UpsertDevice, ValidateRefreshToken, device management
- `http_server.go` — /health, /info endpoints
- `main.go` — entry point, gRPC server interceptors

### Android
- `data/grpc/BearerTokenInterceptor.kt` — ClientInterceptor для JWT Bearer token
- `data/grpc/GrpcClient.kt` — фасад
- `data/grpc/RealGrpcClient.kt` — реализация gRPC (connect, getChats, signInV2, refreshToken)
- `data/auth/AuthManager.kt` — JWT token storage, getBearerToken, needsRefresh, clearTokens
- `data/session/SessionManager.kt` — loginV2 + loginV1 fallback, startTokenRefresh, per-server validation
- `data/session/CredentialStore.kt` — credentials, jwt_server_address tracking
- `ui/widget/ServerAuthBottomSheet.kt` — шторка выбора входа

---

## Правила

- **НЕ** запускать assembleRelease на сервере (OOM)
- **НЕ** компилировать на сервере без крайней необходимости
- **НЕ** деплоить на prod без тестирования на dev
- JWT_SECRET: минимум 32 байта, НЕ коммитить
- Коммитить и пушить после каждого значимого изменения
- server.go:33 — версия сервера
- version.txt — версия Android

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Версия | v1.2.0.1 | v1.2.0.1 (после редеплоя) |
| JWT | да | да |
| /info | да | да (после редеплоя) |

---

## Команды

```bash
# Сборка
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .   # dev
go build -o /tmp/lavender-server .        # prod

# Деплой на dev
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Деплой на prod (после тестирования на dev!)
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Логи
journalctl -u lavender-server-dev -f
journalctl -u lavender-server -f
```

---

## Известные проблемы

### 42P10 на prod БД (НЕ исправлена)
- `Failed to register device ... pq: there is no unique or exclusion constraint matching the ON CONFLICT specification`
- Причина: таблица `user_devices` на prod создана до добавления UNIQUE constraint
- **Решение**: вручную выполнить на prod БД:
  ```sql
  ALTER TABLE user_devices 
  ADD CONSTRAINT user_devices_user_id_device_id_key 
  UNIQUE (user_id, device_id);
  ```
- Не критично — пользователь аутентифицируется, но device registration не проходит
