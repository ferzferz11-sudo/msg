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

## Статус: v1.1.1.4 ЗАВЕРШЕНА

### Сервер v1.1.1.4 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Версия 1.1.1.4 | ✅ | `server.go:33` |

### Android v1.1.1.4 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| **[AI] кнопка** рядом с [+] | ✅ | `activity_chat_list.xml` |
| **AIBottomSheet** (группы + divider) | ✅ | `ui/widget/AIBottomSheet.kt` |
| **showAIActionSheet()** | ✅ | `ChatListActivity.kt` |
| **AI-пункты перенесены** из [+] | ✅ | `ChatListActivity.kt` |
| **widget_ai_bottom_sheet.xml** | ✅ | `res/layout/` |
| **widget_section_header.xml** | ✅ | `res/layout/` |
| **widget_section_divider.xml** | ✅ | `res/layout/` |
| compileDebugKotlin | ✅ | passes |

| Компонент | Статус | Файл |
|-----------|--------|------|
| Bot Command Processor (7 команд) | ✅ | `bot_commands.go` |
| Rate limiting (30 cmd/min, 10 AI/min) | ✅ | `bot_commands.go` |
| Bot command detection в Chat stream | ✅ | `server.go` |
| OWL Status RPC | ✅ | `bot_commands.go` |
| NotificationService (broadcast + history) | ✅ | `bot_commands.go` |
| SendServerNotification в deploy/restart | ✅ | `bot_commands.go` |
| /ai улучшенный промпт | ✅ | `bot_commands.go` |
| OWL ChatWithOWL streaming | ✅ | `owl.go` |
| Hermes Orchestrator | ✅ | `hermes_orchestrator.go` |
| Unit tests для bot_commands | ✅ | `bot_commands_test.go` |
| Исправлен fmt.Sprintf в /logs | ✅ | `bot_commands.go:320` |
| Версия 1.1.1.3 | ✅ | `server.go:33` |

### Android v1.1.1.3 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| **OwlGrpc.kt** (отдельный файл) | ✅ | `data/grpc/OwlGrpc.kt` |
| — chatWithOwl() streaming | ✅ | `OwlGrpc.kt` |
| — processBotCommand() | ✅ | `OwlGrpc.kt` |
| — getBotCommands() | ✅ | `OwlGrpc.kt` |
| — getOWLStatus() | ✅ | `OwlGrpc.kt` |
| — subscribeNotifications() streaming | ✅ | `OwlGrpc.kt` |
| — getNotificationHistory() | ✅ | `OwlGrpc.kt` |
| — markNotificationsRead() | ✅ | `OwlGrpc.kt` |
| — serverNotifications SharedFlow | ✅ | `OwlGrpc.kt` |
| **HermesGrpc.kt** (очищен от OWL) | ✅ | `data/grpc/HermesGrpc.kt` |
| OwlChatActivity | ✅ | `ui/owl/OwlChatActivity.kt` |
| OwlChatViewModel (отдельный поток) | ✅ | `ui/owl/OwlChatViewModel.kt` |
| Slash command detection (/) | ✅ | `OwlChatActivity.kt` |
| Bot Commands UI (dialog) | ✅ | `OwlChatActivity.kt` |
| OWL AI кнопка в bottom sheet | ✅ | `ChatListActivity.kt` |
| **NotificationActivity** | ✅ | `ui/notification/NotificationActivity.kt` |
| **NotificationAdapter** | ✅ | `ui/notification/NotificationAdapter.kt` |
| Кнопка "Уведомления" в bottom sheet | ✅ | `ChatListActivity.kt` |
| Proto: BotCommand*, OWLStatus*, ServerNotification* | ✅ | `data/proto/MessengerProto.kt` |
| Proto: OwlRequestProto, OwlResponseProto | ✅ | `data/proto/MessengerProto.kt` |
| OwlMessage data class | ✅ | `data/models/HermesModel.kt` |
| activity_owl_chat.xml, ic_owl.xml | ✅ | `res/layout/`, `res/drawable/` |
| activity_notification.xml, item_notification.xml | ✅ | `res/layout/` |
| ic_notifications.xml, ic_arrow_back.xml, circle_background.xml | ✅ | `res/drawable/` |
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

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично)

---

## Что НЕ сделано (по приоритету)

### Высокий приоритет
- Тестирование на dev — проверить бот-команды, OWL чат, ChatWithOWL streaming
- Проверить deploy-dev.sh / deploy-prod.sh
- Деплой на prod

### Средний приоритет
- Server Notifications UI на Android (SubscribeNotifications)
- Rate limiting для уведомлений
- Модульные тесты для бот-команд
- APK сборка и тест на устройстве

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- Graceful reconnect при keepalive failed
- NewChatActivity → ChatWidget миграция

---

## Правила работы

1. **Коммитить после каждого значимого изменения**
2. **Пушить в `feat/1.1.1.x`** (не в main!)
3. **Деплоить на dev сервер для тестирования**
4. **Обновлять CHANGELOG.md** с каждым фиксом
5. **Не ломать существующий функционал**
6. **Версия сервера** в `server.go:33` — всегда обновлять при релизе
7. **Разделение архитектуры** — каждый AI-сервис в своём файле, не смешивать

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

## Промпт для следующей сессии (v1.1.1.5)

```
Продолжаем работу над Lavender Messenger. v1.1.1.4 завершена.
Новая версия: v1.1.1.5 (feat/1.1.1.x на обоих репозиториях)

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория чистые, все запушены, теги v1.1.1.4
- Серверы работают (lavender-server-dev, lavender-server)
- Сборка проходит: go build + compileDebugKotlin + go test

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL (chatWithOwl, bot commands, OWL status, notifications)
- HermesGrpc.kt — отдельный файл для Hermes (orchestrator, agent management)
- НЕ смешивать OWL и Hermes код — полная изоляция
- Каждый сервис имеет свои SharedFlows, marshallers, rate limiters

Что сделано (v1.1.1.4):
- [AI] кнопка в списке чатов: AIBottomSheet с группировкой (Оркестратор/OWL)
- AI-пункты перенесены из [+] в [AI] шторку
- Server version bump 1.1.1.3 → 1.1.1.4

Следующие шаги для v1.1.1.5 (по приоритету):
1. **Исправить пути скриптов деплоя** — проверить что /deploy dev и /prod работают корректно
2. **NotificationActivity — badge с количеством непрочитанных**
3. **Graceful reconnect** при keepalive failed
4. **Auth токены для удалённых агентов** (JWT)
5. APK сборка и тест на устройстве
6. Деплой на prod

Правила:
- Коммитить после каждого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования
- Обновлять CHANGELOG.md (новая версия наверху)
- Не ломать существующий функционал
- Версия сервера в server.go:33 — обновлять при релизе
- assembleRelease НЕ запускать на сервере (OOM kill)
- Теги: git tag v1.1.1.5 <commit> && git push origin feat/1.1.1.x --tags
- Разделение архитектуры: каждый AI-сервис в своём файле
- Каждый значимый коммит — с описанием что и почему

Документация: /root/msg/INTEGRATION_SESSION.md, /root/msg/TASKS.md
Команды сборки: см. раздел "Команды" выше
```
