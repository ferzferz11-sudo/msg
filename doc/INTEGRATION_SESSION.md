# Lavender Messenger — Интеграционная сессия

## Контекст

Интеграция AI-чатов в Lavender Messenger: OWL AI (простой чат с AI) и Hermes Orchestrator (мульти-агентная система).

**Текущая ветка:** `feat/1.1.1.x` (оба репозитория)
**Сервер:** dev на порту 50052, prod на 50051

---

## Архитектура разделения

```
СЕРВЕР:
├── owl.go              — OWL AI: ChatWithOWL streaming, сессии, история
├── bot_commands.go     — Bot Commands: /status, /deploy, /logs, /restart, /ai, /help, /version
├── hermes_orchestrator.go — Hermes: оркестратор, маршрутизация агентов
├── hermes_agent_service.go — Hermes: управление агентами
└── server.go           — gRPC handlers, маршрутизация запросов

ANDROID:
├── OwlGrpc.kt          — OWL: chatWithOwl, processBotCommand, getBotCommands, getOWLStatus
├── HermesGrpc.kt       — Hermes: chatWithOrchestrator, agent management
├── GrpcClient.kt       — единая точка доступа
├── OwlChatActivity.kt  — UI чата с OWL
├── OwlChatViewModel.kt — ViewModel (отдельные owlTyping/owlResponses flows)
└── HermesChatActivity.kt — UI чата с Hermes
```

Принцип: полная изоляция OWL и Hermes — разные файлы, разные SharedFlows, разные rate limiters.

---

## Статус: v1.1.1.7 ЗАВЕРШЕНА

### Сервер v1.1.1.7 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Версия 1.1.1.7 | ✅ | `server.go:33` |
| notificationService readStates | ✅ | `bot_commands.go` |
| GetNotificationHistory с is_read | ✅ | `bot_commands.go` |
| MarkNotificationsRead (реальная логика) | ✅ | `bot_commands.go` |
| GetUnreadCount RPC | ✅ | `bot_commands.go` |
| Proto is_read field | ✅ | `messenger.proto` |
| Proto GetUnreadCount RPC | ✅ | `messenger.proto` |

### Android v1.1.1.7 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| ServerNotificationProto isRead | ✅ | `data/proto/MessengerProto.kt` |
| GetUnreadCount RPC client | ✅ | `data/grpc/OwlGrpc.kt` |
| getUnreadCount() в GrpcClient | ✅ | `data/grpc/GrpcClient.kt` |
| NotificationAdapter badge + isRead | ✅ | `ui/notification/NotificationAdapter.kt` |
| NotificationActivity mark as read | ✅ | `ui/notification/NotificationActivity.kt` |
| SheetAction badge field | ✅ | `ui/widget/WidgetSystem.kt` |
| AIBottomSheet badge display | ✅ | `ui/widget/AIBottomSheet.kt` |
| widget_action_item.xml badge | ✅ | `res/layout/widget_action_item.xml` |
| badge_background.xml | ✅ | `res/drawable/badge_background.xml` |
| notification_unread_bg color | ✅ | `res/values/colors.xml` |
| ChatListActivity unreadNotifCount | ✅ | `ChatListActivity.kt` |
| compileDebugKotlin | ✅ | passes |

### Статус: v1.1.1.6 ЗАВЕРШЕНА

### Сервер v1.1.1.6 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Версия 1.1.1.6 | ✅ | `server.go:33` |
| getNextChatNumber() | ✅ | `server.go` |
| generateChatName() | ✅ | `server.go` |
| CreateOwlChat нумерация | ✅ | `server.go` |
| CreateHermesSession нумерация | ✅ | `server.go` |
| Proto name field | ✅ | `messenger.proto` |

### Android v1.1.1.6 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| createOwlChat() | ✅ | `data/grpc/OwlGrpc.kt` |
| OwlChatActivity CHAT_ID from intent | ✅ | `ui/owl/OwlChatActivity.kt` |
| OwlSettingsActivity CHAT_ID from intent | ✅ | `ui/owl/OwlSettingsActivity.kt` |
| AIBottomSheet существующие чаты | ✅ | `ChatListActivity.kt` |
| refreshAiChats() | ✅ | `ChatListActivity.kt` |
| MessengerProto CreateOwlChat* | ✅ | `data/proto/MessengerProto.kt` |
| HermesGrpc name parsing | ✅ | `data/grpc/HermesGrpc.kt` |
| compileDebugKotlin | ✅ | passes |

### Сервер v1.1.1.5 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Версия 1.1.1.5 | ✅ | `server.go:33` |
| HermesSession → chats INSERT | ✅ | `server.go` |
| DeleteChat Hermes fallback | ✅ | `server.go` |
| GetOwlSettings RPC | ✅ | `server.go` |
| creator_id миграция | ✅ | `db_hermes.go` |
| Все проверки владельца по UUID | ✅ | `server.go` |

### Android v1.1.1.5 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| OwlSettingsActivity | ✅ | `ui/owl/OwlSettingsActivity.kt` |
| activity_owl_settings.xml | ✅ | `res/layout/activity_owl_settings.xml` |
| OwlGrpc.kt getOwlSettings/updateOwlSettings | ✅ | `data/grpc/OwlGrpc.kt` |
| MessengerProto.kt OWL settings classes | ✅ | `data/proto/MessengerProto.kt` |
| AndroidManifest регистрация | ✅ | `AndroidManifest.xml` |
| AIBottomSheet → OwlSettingsActivity | ✅ | `ChatListActivity.kt` |
| compileDebugKotlin | ✅ | passes |

### Статус: v1.1.1.4 ЗАВЕРШЕНА

| Компонент | Статус | Файл |
|-----------|--------|------|
| Версия 1.1.1.4 | ✅ | `server.go:33` |
| OWL FK fix (авто-создание чата) | ✅ | `server.go` |
| HermesSession username→UUID | ✅ | `server.go` |

### Android v1.1.1.4 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| [AI] кнопка рядом с [+] | ✅ | `activity_chat_list.xml` |
| AIBottomSheet (группы + divider) | ✅ | `ui/widget/AIBottomSheet.kt` |
| showAIActionSheet() | ✅ | `ChatListActivity.kt` |
| AI-пункты перенесены из [+] | ✅ | `ChatListActivity.kt` |
| HermesChatActivity uses UUID | ✅ | `HermesChatActivity.kt` |
| compileDebugKotlin | ✅ | passes |

---

## Теги

| Версия | Сервер | Android |
|--------|--------|---------|
| v1.1.0.13 | ✅ | ✅ |
| v1.1.0.14 | ✅ | ✅ |
| v1.1.0.15 | ✅ | ✅ |
| v1.1.0.16 | ✅ | ✅ |
| v1.1.1.1 | ✅ | ✅ |
| v1.1.1.2 | ✅ | ✅ |
| v1.1.1.3 | ✅ | ✅ |
| v1.1.1.4 | ✅ | ✅ |
| v1.1.1.7 | ✅ | ✅ |

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично)
- ~~Невалидный JSON в participants при создании OWL/Hermes чатов~~ ✅ Исправлено

---

## Что НЕ сделано (по приоритету)

### Высокий приоритет
- **Исправить DeleteChat для Hermes чатов** — `sql: no rows in result set` при удалении hermes чата (чат есть в `hermes_sessions`, но не в `chats`)
- **Деплой на prod** — после исправления DeleteChat

### Средний приоритет
- ~~NotificationActivity — badge с количеством непрочитанных~~ ✅ v1.1.1.7
- **Graceful reconnect** — переподключение без потери стримов → v1.1.1.8
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## Правила работы

1. **Коммитить после каждого значимого изменения**
2. **Пушить в `feat/1.1.1.x`** (не в main!)
3. **Деплоить на dev сервер для тестирования**
4. **Обновлять CHANGELOG.md** с каждым фиксом (серверный — только сервер, клиентский — только клиент)
5. **Не ломать существующий функционал**
6. **Версия сервера** в `server.go:33` — всегда обновлять при релизе
7. **Разделение архитектуры** — каждый AI-сервис в своём файле, не смешивать
8. **userId (UUID)** — всегда использовать UUID как ключ, не username

---

## Команды

```bash
# === СЕРВЕР ===
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Сборка и деплой на dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# === ANDROID ===
cd /root/msg.client.android
./gradlew compileDebugKotlin    # проверка компиляции
./gradlew assembleDebug         # сборка APK (локально!)
# assembleRelease НЕ запускать на сервере — OOM
```

---

## Важно

- **НЕ использовать `--go_out=.`** при proto gen (генерирует в корень, ломает сборку)
- **go PATH:** `export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- **Dev DB:** `chat_db_dev` (порт 5432, user: lavender)
- **Prod DB:** `chat_db` (порт 5432, user: lavender)
- **Dev config:** `/root/LavenderMessenger/run/.env.dev`
- **Prod config:** `/root/LavenderMessenger/run/.env`

---

## Промпт для следующей сессии (v1.1.1.8)

```
Продолжаем работу над Lavender Messenger. v1.1.1.7 завершена и протестирована.

Новая версия: v1.1.1.8 (feat/1.1.1.x на обоих репозиториях)

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Сервер обновлён и деплоен на dev, сборка проходит
- Теги v1.1.1.7 на обоих репозиториях

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL
- HermesGrpc.kt — отдельный файл для Hermes
- НЕ смешивать OWL и Hermes код — полная изоляция
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца, creator_username — только для отображения
- Множественные чаты: каждый новый чат создаётся через серверный CreateOwlChat/CreateHermesSession
- Нумерация: Лава ИИ #1, #2... / Оркестратор #1, #2... (MAX+1, не переиспользуется)
- participants ВСЕГДА формировать через json.Marshal — никогда не собирать строку вручную

Что сделано (v1.1.1.7):
- Notification badge ✅
- Per-user read tracking на сервере (in-memory) ✅
- GetUnreadCount RPC ✅
- Badge на пункте "Уведомления" в AIBottomSheet ✅
- Визуальное отличие непрочитанных в NotificationActivity ✅
- Автоматическая отметка при открытии NotificationActivity ✅
- Исправлен невалидный JSON в participants (json.Marshal) ✅

Следующие шаги для v1.1.1.8 (по приоритету):
1. **Graceful reconnect** — переподключение без потери стримов
   - При разрыве соединения (keepalive failed / network loss) автоматически
     переподключаться с экспоненциальным backoff
   - Не терять активные streaming calls (OWL chat, Hermes chat, notifications)
   - Показывать индикатор переподключения в UI
2. **Деплой на prod** — после завершения graceful reconnect

Правила:
- Коммитить после каждого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования
- Обновлять CHANGELOG.md (новая версия наверху)
- Не ломать существующий функционал
- Версия сервера в server.go:33 — обновлять при релизе
- assembleRelease НЕ запускать на сервере (OOM kill)
- Теги: git tag v1.1.1.8 <commit> && git push origin feat/1.1.1.x --tags
- userId (UUID) — всегда как ключ, не username
- creator_id (UUID) — всегда для проверки владельца
- Деплой на prod — только после завершения ВСЕХ задач интеграции
- participants ВСЕГДА через json.Marshal, никогда вручную

Документация:
- Индекс: /root/msg/doc/INDEX.md (читать в начале сессии)
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
Команды сборки: см. раздел "Команды" выше
```
