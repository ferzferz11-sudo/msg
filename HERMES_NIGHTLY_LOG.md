# Hermes Orchestrator — Ночной лог (03-04 июня 2026)

## Сессия: 04.06.2026 01:00 UTC (ночная)

---

## Задача 1: Диагностика gRPC соединения Android → Dev Server

### Результаты диагностики

#### ✅ Сервер работает
- `systemctl status lavender-server-dev`: **active (running)** с 20:51:25 UTC
- PID: 694111, память: 5.9M, CPU: 203ms
- Версия: **1.1.0.9** (в коде `server.go:33`, не обновлена до 1.1.0.13 — косметика)

#### ✅ Порты открыты
- `ss -tlnp`: 50052 (gRPC) и 8083 (HTTP) слушаются на `*:port`
- Нет активных соединений (ночь, телефон не подключён)

#### ✅ Firewall разрешает
- `iptables -L INPUT`: `ACCEPT tcp -- 0.0.0.0/0 0.0.0.0/0 tcp dpt:50052`
- Политика INPUT: DROP, но порт 50052 явно разрешён

#### ✅ gRPC keepalive настроен на сервере
```go
// main.go:111-121
grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
    MinTime:             5 * time.Second,
    PermitWithoutStream: true,
}),
grpc.KeepaliveParams(keepalive.ServerParameters{
    MaxConnectionIdle:     15 * time.Minute,
    MaxConnectionAge:      30 * time.Minute,
    MaxConnectionAgeGrace: 5 * time.Second,
    Time:                  20 * time.Second,  // Ping every 20s
    Timeout:               20 * time.Second,  // 20s to respond
}),
```

#### ✅ IP адрес верный
- Внешний IP: `13.140.25.249` (совпадает с конфигурацией Android)
- Интерфейс: `eth0 inet 13.140.25.249/25`

#### Анализ ошибок в логах

Все 15 ошибок `context canceled` за последние 3 часа — из **CALL стрима** (звонки/чат), **НЕ из Hermes**:

```
Jun 03 19:53:39 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 19:59:07 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 19:59:13 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:02:55 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:06:56 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:07:03 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:10:05 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:10:08 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:10:24 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:10:37 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:10:44 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:11:09 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
Jun 03 20:11:15 [CALL] Error receiving signal: rpc error = Canceled desc = context canceled
Jun 03 20:14:54 [CALL] Error receiving signal: rpc error: code = Canceled desc = context canceled
```

**Hermes Orchestrator не вызывался после рестарта сервера в 20:51.**
Единственная запись: `Hermes Orchestrator initialized with 8 agents`

#### Выводы

**Сервер работает корректно.** Проблема gRPC соединения — **на стороне Android клиента**.

Корневая причина: Android клиент создаёт gRPC канал и сразу (или через короткое время) отменяет контекст. Это происходит на уровне `ManagedChannelBuilder` или при создании стрима.

### Рекомендации для исправления на Android

1. **Порядок `usePlaintext()` и `build()`** — в `RealGrpcClient.kt` убедиться что `usePlaintext()` вызывается ДО `build()`:
   ```kotlin
   val channel = ManagedChannelBuilder
       .forAddress("13.140.25.249", 50052)
       .usePlaintext()  // ДО build()!
       .build()
   ```

2. **Keepalive на клиенте** — добавить в `ManagedChannelBuilder`:
   ```kotlin
   .keepAliveTime(30, TimeUnit.SECONDS)
   .keepAliveTimeout(10, TimeUnit.SECONDS)
   .keepAliveWithoutCalls(true)
   ```

3. **VPN/Прокси** — на телефоне может быть активен VPN который блокирует HTTP/2 (gRPC). Попробовать без VPN.

4. **Таймаут подключения** — увеличить:
   ```kotlin
   .maxInboundMessageSize(4 * 1024 * 1024)
   .idleTimeout(5, TimeUnit.MINUTES)
   ```

5. **Переподключение** — реализовать retry с exponential backoff при `UNAVAILABLE`.

### Что НЕ нужно делать на сервере
- ❌ Пересобирать сервер — код актуален, версия 1.1.0.9 работает
- ❌ Менять keepalive — уже настроен (20s/20s)
- ❌ Менять firewall — порт открыт
- ❌ Перезапускать — работает стабильно

### Версия в коде (косметика)
- `server.go:33`: `const ServerVersion = "1.1.0.9"` — не обновлена до 1.1.0.13
- Последний коммит `22b7b8f` меняет только `CHANGELOG.md`
- Можно обновить до `1.1.0.14-dev` при следующем деплое

---

## Задача 2: AgentSettingsBottomSheet

### Статус: Не начата (требует Android код)

Файлы для изменения находятся локально у пользователя:
- `ui/hermes/AgentListActivity.kt` — добавить long click handler
- `ui/hermes/AgentSettingsBottomSheet.kt` — НОВЫЙ файл
- `res/layout/bottom_sheet_agent_settings.xml` — НОВЫЙ layout

Подробная спецификация: `AGENT_SETTINGS_SHEET_PROMPT.md`

---

## Задача 3: Welcome Message

### Статус: Реализован на сервере, не тестирован на Android

Код в `server.go:3111-3123` и `server.go:3149-3171`:
- Welcome message отправляется с `Finished: true` для новых сессий
- Содержит markdown-форматированный список агентов
- Не тестирован — Hermes не вызывался после рестарта

---

## Задача 4: Улучшения оркестратора

### Статус: Не начата

---

## Итого по ночи

| Задача | Статус | Результат |
|--------|--------|-----------|
| 1. gRPC диагностика | ✅ Завершена | Сервер OK, проблема на Android |
| 2. AgentSettingsBottomSheet | ⏸ Ожидает | Требует Android код |
| 3. Welcome Message | ✅ Реализован | Не тестирован |
| 4. Улучшения | ⏸ Ожидает | Не начата |

### Следующие шаги
1. **Пользователю**: исправить gRPC клиент на Android (keepalive, usePlaintext порядок)
2. **Пользователю**: собрать debug APK локально и протестировать
3. **Сервер**: обновить `ServerVersion` до `1.1.0.14-dev` при следующем изменении кода
