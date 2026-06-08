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

## Статус: v1.1.1.10 ЗАВЕРШЕНА

### Сервер v1.1.1.10 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| hermes_chat_settings таблица | ✅ | `db.go` |
| GetHermesSettings RPC | ✅ | `server.go` |
| UpdateHermesSettings RPC | ✅ | `server.go` |
| hermesSettingsManager | ✅ | `owl.go` |
| freeTierRateLimiter (20/час) | ✅ | `owl.go` |
| Rate limit в ChatWithOWL (custom 10/min, free 20/hr) | ✅ | `server.go` |
| Rate limit в ChatWithOrchestrator (custom 10/min, free 20/hr) | ✅ | `server.go` |
| GetOwlSettings: is_using_custom_key | ✅ | `server.go` |
| GetAIChats: is_using_custom_key + model | ✅ | `server.go` |
| Proto: Hermes settings messages + fields | ✅ | `messenger.proto` |
| ServerVersion 1.1.1.10 | ✅ | `server.go:33` |
| Dev deployed | ✅ | работает |

### Android v1.1.1.10 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| AIBottomSheet полный редизайн | ✅ | `ui/widget/AIBottomSheet.kt` |
| widget_ai_chat_item.xml (нов layout) | ✅ | `res/layout/widget_ai_chat_item.xml` |
| Шестерёнка настроек у каждого чата | ✅ | layout + AIBottomSheet |
| Long-press → PopupMenu (Настройки / Удалить) | ✅ | AIBottomSheet |
| Единый список AI чатов (OWL + Hermes) | ✅ | AIBottomSheet |
| Divider + блоки "Лава ИИ" и "OWL агент" | ✅ | AIBottomSheet |
| OwlSettingsActivity: unified OWL+Hermes | ✅ | `ui/owl/OwlSettingsActivity.kt` |
| Key source indicator (свой/общий ключ) | ✅ | layout + activity |
| Rate limit info (20/час для free tier) | ✅ | layout + activity |
| Динамический список моделей по ключу | ✅ | OwlSettingsActivity |
| HermesGrpc: get/updateSettings | ✅ | `data/grpc/HermesGrpc.kt` |
| OwlGrpc: обновлённый парсер | ✅ | `data/grpc/OwlGrpc.kt` |
| RealGrpcClient: AIChatInfo парсер | ✅ | `data/grpc/RealGrpcClient.kt` |
| AIChatInfo: isUsingCustomKey + model | ✅ | `data/models/Message.kt` |
| MessengerProto: новые классы | ✅ | `data/proto/MessengerProto.kt` |
| compileDebugKotlin | ✅ | passes |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.9 ЗАВЕРШЕНА

### Сервер v1.1.1.9 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| Grace period (30s) в hub | ✅ | `hub.go` |
| StartGracePeriod / IsInGracePeriod / ClearGracePeriod | ✅ | `hub.go` |
| GetOnlineUsers включает grace period users | ✅ | `hub.go` |
| ClearGracePeriod при re-auth | ✅ | `server.go:Chat` |
| ServerVersion 1.1.1.9 | ✅ | `server.go:33` |

### Android v1.1.1.9 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| ConnectionStatus.RECONNECTING | ✅ | `RealGrpcClient.kt` |
| Exponential backoff reconnect | ✅ | `RealGrpcClient.kt` |
| subscribeNotifications retry | ✅ | `OwlGrpc.kt` |
| chatWithOwl retry | ✅ | `OwlGrpc.kt` |
| chatWithOrchestrator retry | ✅ | `HermesGrpc.kt` |
| onError → RECONNECTING | ✅ | `RealGrpcClient.kt` |
| Keep-alive 10s/5s | ✅ | `RealGrpcClient.kt` |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.8 ЗАВЕРШЕНА

### Сервер v1.1.1.8 (`/root/msg`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| participants хранит UUID | ✅ | `server.go` (3 места) |
| GetUserChats исключает AI | ✅ | `db.go:GetUserChats` |
| GetAllChats без AI | ✅ | `server.go:GetAllChats` |
| GetAIChats RPC | ✅ | `server.go` |
| RenameAIChat RPC | ✅ | `server.go` |
| DeleteChat skip AI notify | ✅ | `server.go:DeleteChat` |
| Proto AI messages | ✅ | `messenger.proto` |
| ServerVersion 1.1.1.8 | ✅ | `server.go:33` |

### Android v1.1.1.8 (`/root/msg.client.android`)

| Компонент | Статус | Файл |
|-----------|--------|------|
| getAIChats() в GrpcClient | ✅ | `data/grpc/` |
| refreshAiChats() через RPC | ✅ | `ChatListActivity.kt` |
| AIBottomSheet selection mode | ✅ | `ui/widget/AIBottomSheet.kt` |
| showAIActionSheet тулбар | ✅ | `ChatListActivity.kt` |
| AIChatInfo data class | ✅ | `data/models/Message.kt` |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.7 ЗАВЕРШЕНА

| Компонент | Статус | Файл |
|-----------|--------|------|
| Notification badge (сервер) | ✅ | `bot_commands.go` |
| GetUnreadCount RPC | ✅ | `bot_commands.go` |
| Notification badge (Android) | ✅ | `AIBottomSheet.kt`, `ChatListActivity.kt` |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.6 ЗАВЕРШЕНА

| Компонент | Статус | Файл |
|-----------|--------|------|
| Множественные OWL/Hermes чаты с нумерацией | ✅ | `server.go` |
| createOwlChat() | ✅ | `OwlGrpc.kt` |
| AIBottomSheet существующие чаты | ✅ | `ChatListActivity.kt` |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.5 ЗАВЕРШЕНА

| Компонент | Статус | Файл |
|-----------|--------|------|
| OwlSettingsActivity | ✅ | `ui/owl/OwlSettingsActivity.kt` |
| getOwlSettings/updateOwlSettings | ✅ | `OwlGrpc.kt` |
| creator_id миграция | ✅ | `db_hermes.go` |
| compileDebugKotlin | ✅ | passes |

---

## Статус: v1.1.1.4 ЗАВЕРШЕНА

| Компонент | Статус | Файл |
|-----------|--------|------|
| [AI] кнопка рядом с [+] | ✅ | `activity_chat_list.xml` |
| AIBottomSheet (группы + divider) | ✅ | `ui/widget/AIBottomSheet.kt` |
| OWL FK fix | ✅ | `server.go` |
| compileDebugKotlin | ✅ | passes |

---

## ✅ Сделано (ранее)

- v1.1.1.3 — NotificationActivity, bot tests
- v1.1.1.2 — SendServerNotification, OWL/Hermes разделение
- v1.1.1.1 — Bot Commands, Rate Limiting, NotificationService
- v1.1.0.16 — Favorites fix
- v1.1.0.15 — Force reconnect + Registration fix + Cache clearing
- v1.1.0.14 — Hermes sessions in chat list
- v1.1.0.13 — ChatWidget + Mention system
- v1.1.0.12 — Unified Chat Widget
- v1.1.0.11 — Hermes Orchestrator
- v1.1.0.10 — Agent Management gRPC

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
| v1.1.1.5 | ✅ | ✅ |
| v1.1.1.6 | ✅ | ✅ |
| v1.1.1.7 | ✅ | ✅ |
| v1.1.1.8 | ✅ | ✅ |
| v1.1.1.9 | ✅ | ✅ |
| v1.1.1.10 | ✅ | ✅ |

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично)
- ~~Невалидный JSON в participants при создании OWL/Hermes чатов~~ ✅ Исправлено

---

## Что НЕ сделано (по приоритету)

### Перед деплоем на prod (обязательно)
1. **Сессия: Тестирование (v1.1.1.11)** — полное тестирование всех новых фич: graceful reconnect, notification retry, AI chats, новых настроек (OwlSettings, rate limits, Hermes settings), edge cases. Затем деплой на prod.
2. **Деплой на prod → v1.1.2.0** — только после завершения тестирования

### Средний приоритет
- ~~NotificationActivity badge~~ ✅ v1.1.1.7
- ~~Graceful reconnect~~ ✅ v1.1.1.9
- ~~AI Bottom Sheet редизайн~~ ✅ v1.1.1.10
- ~~Hermes per-chat settings~~ ✅ v1.1.1.10
- ~~Rate limiting (free tier 20/hr)~~ ✅ v1.1.1.10
- Показ ключа/модели в шапке чата (OwlChatActivity + HermesChatActivity)
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

## Промпт для следующей сессии (v1.1.1.11 — тестирование и полировка)

```
Продолжаем работу над Lavender Messenger. v1.1.1.10 завершена:
- AI Bottom Sheet полностью переписан (единый список, long-press popup menu, шестерёнки настроек)
- Hermes per-chat settings (таблица, RPCs, UI)
- Rate limiting: свой ключ = 10/min, бесплатный = 20/hour
- Настройки показывают источник ключа и лимит запросов

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория на ветке feat/1.1.1.x
- v1.1.1.10 тег на обоих репозиториях
- Dev сервер обновлён и работает

Текущая версия: v1.1.1.10

Что нужно сделать (v1.1.1.11 — тестирование и доработка):

1. **Показать информацию о ключе/модели в шапке AI чатов**
   - OwlChatActivity: показать "Общий ключ / 20 запросов/час" или "Ваш ключ / все модели"
   - HermesChatActivity: аналогично
   - Маленький баннер/текст под toolbar

2. **Протестировать весь флоу AI чатов**:
   - Создание Hermes чата через AIBottomSheet → открытие → отправка сообщений → rate limit
   - Создание OWL чата → то же самое
   - Удаление чата через long-press
   - Настройки: ввести свой ключ → проверить что модели расширились
   - Настройки: убрать ключ → проверить что модели сузились, лимит 20/час
   - Graceful reconnect: выключить сеть → включить → чаты не теряются
   - Notification retry: убить соединение → уведомления приходят после reconnect

3. **Исправить найденные баги**

4. **После тестирования — деплой на prod и таг v1.1.2.0**

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL
- HermesGrpc.kt — отдельный файл для Hermes
- НЕ смешивать OWL и Hermes код — полная изоляция
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- ConnectionStatus: DISCONNECTED, CONNECTING, READY, RECONNECTING, FAILED

Правила:
- Коммитить после каждого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования
- Обновлять CHANGELOG.md (новая версия наверху)
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при релизе

Документация:
- Индекс: /root/msg/doc/INDEX.md (читать в начале сессии)
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
```
