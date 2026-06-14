# Лава — Server Changelog

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
