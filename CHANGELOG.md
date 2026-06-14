# Лава — Server Changelog

## [1.2.0.0] - 2026-06-14

### Новое: AuthService v2 (JWT) — основной метод аутентификации
- **SignInV2/SignUpV2** — JWT access (15min) + refresh (30 days) tokens
- **RefreshToken** — ротация refresh token с обнаружением reuse
- **SignOut/RevokeDevice/GetDevices** — управление сессиями
- **gRPC Bearer token interceptor** — валидация JWT на каждом вызове
- **Device management** — user_devices, device_auth_log таблицы
- **Auth audit log** — логирование всех auth событий

### Deprecated: AuthService v1 (Chat stream auth)
- v1 продолжает работать для совместимости со старыми клиентами
- При входе по v1 сервер отправляет warning:
  `DEPRECATED: AuthService v1 is deprecated. Please upgrade to v2 (JWT).`
- Все функции v1 работают без ограничений

---

## [1.1.3.10] - 2026-06-14

### Исправлено
- Онлайн-статус: очистка истекших grace period (30с) в `GetOnlineUsers()`
  Раньше пользователи оставались "онлайн" навсегда после отключения

---

## [1.1.3.9] - 2026-06-13
- ServerVersion обновлён до 1.1.3.9

## [1.1.3.8] - 2026-06-13

### Новое: DeployAgentTaskStream
- Один финальный `done=True` с полными данными (stdout, stderr, exit_code, duration_ms)
- 6 unit-тестов

### Рефакторинг
- Remote Agent RPC вынесен в `server_remote.go`
- Graceful degradation + stale detection

## [1.1.3.7] - 2026-06-13

### Новое: Streaming RPC
- `DeployAgentTaskStream` — server-side streaming для real-time вывода задач
- `HandleTaskStream` + `RemoteTaskStreamUpdate` callback

## [1.1.3.5] - 2026-06-13
- Remote Agent: foreground service + singleton manager

## [1.1.3.4] - 2026-06-12
- Hermes Gateway (SSH туннель), 40 unit tests

## [1.1.3.3] - 2026-06-12
- Reconnect + token filtering + task results

## [1.1.3.2] - 2026-06-12
- Health check, graceful shutdown, agent process management

## [1.1.3.1] - 2026-06-12
- Token UX fixes, rate limiting

## [1.1.3.0] - 2026-06-12
- Agent Token RPCs без IsSuperAdmin, Platform Adapter
