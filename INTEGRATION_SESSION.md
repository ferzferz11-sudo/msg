# Lavender Chat — Интеграционная сессия

## Контекст

Интеграция полноценного чата в Lavender Messenger: OWL AI, бот-команды, серверные уведомления.

**Текущая ветка:** `feat/1.1.1.x` (оба репозитория)
**Сервер:** dev на порту 50052, prod на 50051

---

## Статус: Фаза 1 ЗАВЕРШЕНА

### Сервер v1.1.1.1 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Bot Command Processor | ✅ | `bot_commands.go` |
| 7 команд (/status, /deploy, /logs, /restart, /ai, /help, /version) | ✅ | `bot_commands.go` |
| Rate limiting (30 cmd/min, 10 AI/min) | ✅ | `bot_commands.go` |
| Bot command detection в Chat stream | ✅ | `server.go:~300` |
| OWL Status RPC | ✅ | `bot_commands.go` |
| NotificationService (broadcast + history) | ✅ | `bot_commands.go` |
| Proto: BotCommand*, OWLStatus*, ServerNotification* | ✅ | `messenger.proto` |
| Версия 1.1.1.1 | ✅ | `server.go:33` |

### Android v1.1.1.1 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| OwlChatActivity | ✅ | `ui/owl/OwlChatActivity.kt` |
| OwlChatViewModel | ✅ | `ui/owl/OwlChatViewModel.kt` |
| Slash command detection (/) | ✅ | `OwlChatActivity.kt:~270` |
| Bot Commands UI (dialog) | ✅ | `OwlChatActivity.kt:~230` |
| OWL AI кнопка в bottom sheet | ✅ | `ChatListActivity.kt:~2001` |
| gRPC: processBotCommand, getBotCommands, getOWLStatus | ✅ | `data/grpc/HermesGrpc.kt:~837` |
| Proto: BotCommand*, OWLStatus*, ServerNotification* | ✅ | `data/proto/MessengerProto.kt:~1055` |
| OwlMessage data class | ✅ | `data/models/HermesModel.kt:~55` |
| activity_owl_chat.xml | ✅ | `res/layout/activity_owl_chat.xml` |
| ic_owl.xml | ✅ | `res/drawable/ic_owl.xml` |
| AndroidManifest registration | ✅ | `AndroidManifest.xml:~109` |
| compileDebugKotlin | ✅ | passes |

---

## Теги

Схема: `v1.1.0.X` → `v1.1.1.X` (по возрастанию)

| Версия | Сервер | Android |
|--------|--------|---------|
| v1.1.0.13 | ✅ | ✅ |
| v1.1.0.14 | ✅ | ✅ |
| v1.1.0.15 | ✅ | ✅ |
| v1.1.0.16 | ✅ | ✅ |
| v1.1.1.1 | ✅ | ✅ |

```bash
# Создать тег
git tag v1.1.1.2 <commit_hash>
git push origin feat/1.1.1.x --tags

# Удалить тег
git tag -d v1.1.1.2
git push origin :refs/tags/v1.1.1.2
```

---

## Известные проблемы (не критично)

1. **OwlChatViewModel** переиспользует `hermesTyping/hermesResponses` SharedFlows из HermesGrpc — для прототипа ок, потом нужен отдельный OWL streaming
2. **Server migration warnings** при запуске: `role "lavender" does not exist` — не критично, сервер работает
3. **/ai команда** в bot_commands.go вызывает `callOpenRouterContext` напрямую (не через оркестратор)
4. **SendServerNotification** определена но не используется в deploy/restart handlers

---

## Что НЕ сделано (будущие фазы)

- Отдельный OWL streaming gRPC для Android (сейчас переиспользует Hermes flows)
- UI для серверных уведомлений на Android (SubscribeNotifications)
- Модульное тестирование бот-команд
- Интеграция с deploy скриптами (deploy-dev.sh / deploy-prod.sh)
- Сборка APK и тестирование на устройстве
- Деплой на prod

---

## Правила работы

1. **Коммитить после каждого значимого изменения**
2. **Пушить в `feat/1.1.1.x`** (не в main!)
3. **Деплоить на dev сервер для тестирования**
4. **Обновлять CHANGELOG.md** с каждым фиксом
5. **Не ломать существующий функционал**
6. **Версия сервера** в `server.go:33` — всегда обновлять при релизе

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
