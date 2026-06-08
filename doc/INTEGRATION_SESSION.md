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

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично)

---

## Что НЕ сделано (по приоритету)

### Высокий приоритет
- **Исправить DeleteChat для Hermes чатов** — `sql: no rows in result set` при удалении hermes чата (чат есть в `hermes_sessions`, но не в `chats`)
- **Деплой на prod** — после исправления DeleteChat

### Средний приоритет
- NotificationActivity — badge с количеством непрочитанных
- Graceful reconnect при keepalive failed
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

## Промпт для следующей сессии (v1.1.1.6)

```
Продолжаем работу над Lavender Messenger. v1.1.1.5 частично завершена (HermesSession→chats, DeleteChat fix, creator_id migration — сделаны).

Новая версия: v1.1.1.6 (feat/1.1.1.x на обоих репозиториях)

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Сервер обновлён и деплоен на dev
- Сборка проходит: go build + compileDebugKotlin

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL (chatWithOwl, bot commands, OWL status, notifications)
- HermesGrpc.kt — отдельный файл для Hermes (orchestrator, agent management)
- НЕ смешивать OWL и Hermes код — полная изоляция
- userId (UUID) — всегда использовать как ключ, НЕ username
- creator_id (UUID) — для проверки владельца чата, creator_username — только для отображения

Что сделано (v1.1.1.5, server):
- GetOwlSettings + UpdateOwlSettings handlers ✅
- DeleteChat для Hermes fallback ✅
- HermesSession → chats INSERT ✅
- creator_id колонка + миграция ✅
- Все проверки владельца теперь по creator_id (UUID) ✅
- UpdateOwlSettings: исправлен баг creator_username vs UUID ✅

Что НЕ доделано (v1.1.1.5, Android — нужно доделать в начале сессии):
1. getOwlSettings()/updateOwlSettings() в OwlGrpc.kt — НЕ ДОДЕЛАНО
2. Регистрация OwlSettingsActivity в AndroidManifest.xml — НЕ ДОДЕЛАНО
3. Подключение AIBottomSheet → OwlSettingsActivity (вместо Toast) — НЕ ДОДЕЛАНО

Эти 3 пункта доделать ПЕРВЫМИ, потом переходить к v1.1.1.6.

Следующие шаги для v1.1.1.6 (по приоритету):

1. **Множественные OWL/Hermes чаты с нумерацией (Server)**
   - Старый подход: один OWL чат на пользователя (chatId = "owl-$userId"), один Hermes
   - Новый: каждый новый чат уникален с порядковым номером
   - OWL: `Лава ИИ #1`, `Лава ИИ #2`, ... (русский) / `Lava AI #1`, `Lava AI #2`, ... (english)
   - Hermes: `Оркестратор #1`, `Оркестратор #2`, ... (русский) / `Orchestrator #1`, `Orchestrator #2`, ... (english)
   - Номер = MAX(existing_number) + 1 для данного user_id и type в chats
   - При удалении номера НЕ переиспользуются (всегда инкремент от максимального)
   - SQL для определения следующего номера:
     SELECT COALESCE(MAX(CAST(SUBSTRING(name FROM '#(\d+)$') AS INTEGER)), 0) + 1
     FROM chats WHERE user_id = $1 AND type = $2
   - chatID оставляем UUID-based (owl-$userId-$uuid8), меняем только name

2. **Множественные OWL/Hermes чаты с нумерацией (Android)**
   - Убрать генерацию chatId на клиенте (owl-$userId, hermes-$userId)
   - Для "Создать новый" — вызывать серверный CreateOwlChat/CreateHermesSession
   - Сервер вернёт chatID и name — использовать их
   - AIBottomSheet: "Lava AI" → создать новый чат (первый раз или +1)
   - AIBottomSheet: показывать существующие чаты с номерами

3. **NotificationActivity badge (Android)** — счётчик непрочитанных на иконке колокольчика

4. **Graceful reconnect (Android)** — переподключение без потери стримов

Правила:
- Коммитить после каждого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования
- Обновлять CHANGELOG.md (новая версия наверху)
- Серверный CHANGELOG — только серверные изменения
- Android CHANGELOG — только клиентские изменения
- Не ломать существующий функционал
- Версия сервера в server.go:33 — обновлять при релизе
- assembleRelease НЕ запускать на сервере (OOM kill)
- Теги: git tag v1.1.1.6 <commit> && git push origin feat/1.1.1.x --tags
- userId (UUID) — всегда использовать как ключ, не username
- creator_id (UUID) — всегда для проверки владельца
- Деплой на prod — только после завершения ВСЕХ задач интеграции

Документация: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
Документация Android: /root/msg.client.android/doc/TASKS.md
Индекс документации: /root/msg/doc/INDEX.md
Команды сборки: см. раздел "Команды" выше
```
