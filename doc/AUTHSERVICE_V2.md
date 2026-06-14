# Lavender Messenger — AuthService v2 Integration Index

**Версия:** v1.1.4.0
**Дата:** 2026-06-14
**Статус:** Сервер готов, клиенты в процессе

---

## Обзор

Единая система авторизации на основе JWT токенов для всех клиентов.
Заменяет legacy Chat stream auth (password в первом Message).

---

## Архитектура

```
Клиент → SignInV2(username, password, deviceInfo) → JWT access + refresh
Клиент → каждый gRPC запрос + Bearer <access_token>
Сервер → валидирует JWT → извлекает user_id + device_id
Клиент → access истёк → RefreshToken(refresh_token) → новые access + refresh
```

**Токены:**
- Access JWT: 15 минут, содержит user_id, username, device_id, type="access"
- Refresh JWT: 30 дней, содержит user_id, device_id, type="refresh", jti
- Refresh token rotation: при каждом refresh — новый refresh token, старый инвалидируется

**Device Management:**
- При логине устройство регистрируется/обновляется в БД
- Пользователь может видеть список устройств и отзывать их
- При обнаружении reuse refresh token — все устройства отзываются

---

## Статус по компонентам

### Сервер (Go) — ✅ ГОТОВ

| Файл | Назначение |
|------|-----------|
| `auth_jwt.go` | JWT генерация/валидация |
| `auth_service_v2.go` | SignInV2/SignUpV2/RefreshToken/SignOut/RevokeDevice/GetDevices |
| `auth_interceptor.go` | gRPC Bearer token interceptor |
| `db_auth_devices.go` | CRUD для user_devices + device_auth_log |
| `db_auth_migrations.go` | миграция существующих таблиц |
| `messenger.proto` | V2 сообщения (SignInRequestV2, AuthResponseV2, etc.) |

**Деплой:** dev сервер обновлён и работает.

### Android (Kotlin) — 🔄 ЧАСТИЧНО

| Файл | Статус |
|------|--------|
| `AuthManager.kt` | ✅ Создан (JWT storage, parsing, refresh logic) |
| `CredentialStore.kt` | ✅ Добавлен getAuthPrefs() |
| `MessengerProto.kt` | ✅ Добавлены V2 proto классы |
| `RealGrpcClient.kt` | ✅ Добавлены V2 методы + marshallers |
| `GrpcClient.kt` | ✅ Добавлены facade методы |
| `SessionManager.kt` | ⬜ Нужна интеграция (вызов V2 вместо startChat) |
| `ChatListActivity.kt` | ⬜ Нужна интеграция |
| `ServersActivity.kt` | ⬜ Нужна интеграция |

### Web (TypeScript) — 📋 ПЛАН

| Файл | Что изменить |
|------|-------------|
| `grpcClient.ts` | Заменить signIn/signUp на V2, обновить interceptor с auto-refresh |
| `authStore.ts` | TokenPair вместо accessToken, localStorage ключи |
| Login/Register UI | Вызовы V2 + deviceInfo |
| Settings | Секция управления устройствами |

**План:** `/root/msg.client.web/doc/PROMPT.md`

### iOS (Swift) — 📋 ПЛАН

| Файл | Что изменить |
|------|-------------|
| `GRPCManager.swift` | Добавить V2 методы + AuthInterceptor |
| `AuthViewModel.swift` | Полная переработка (убрать legacy, добавить V2) |
| `TokenStore.swift` | Создать (Keychain storage для JWT) |
| `CredentialStore.swift` | Убрать хранение password |
| `AuthView.swift` | Обновить вызовы |
| `DevicesView.swift` | Создать (управление устройствами) |

**План:** `/root/msg.client.ios/doc/PROMPT.md`

### macOS (Go) — 📋 ПЛАН

| Файл | Что изменить |
|------|-------------|
| `auth_v2.go` | Создать (AuthClient + TokenStore) |
| `auth_interceptor.go` | Создать (gRPC interceptor) |
| `keychain.go` | Создать (file-based token storage) |
| `main.go` | Удалить legacy login, добавить V2 flow |

**План:** `/root/msg.client.macos/msg.client.macos/doc/PROMPT.md`

---

## Миграционная стратегия

### Фаза 2 (v1.1.4.0) — ТЕКУЩАЯ
- ✅ AuthService v2 на сервере
- 🔄 Клиенты переходят на V2
- Legacy Chat stream auth продолжает работать

### Фаза 3 (v1.2.0) — БУДУЩАЯ
- Deprecate Chat stream auth
- Сервер требует JWT для новых фич
- Удаление legacy кода из клиентов
- Timeline: 2-4 недели после перехода всех клиентов

---

## Безопасность

- JWT подписан HMAC-SHA256, секрет из JWT_SECRET env (мин 32 байта)
- Access token не хранится на сервере (stateless)
- Refresh token JTI хранится в БД — можно отозвать
- Token rotation при каждом refresh
- Device audit log для всех auth событий
- При обнаружении reuse — все устройства отзываются

---

## Команды

```bash
# Сервер — сборка и деплой на dev
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Тесты
go test -v -run TestGenerateAndValidateTokenPair .
go test ./...
```
