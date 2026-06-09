# Lavender Messenger — Интеграционная сессия

**Текущая версия:** v1.1.1.15
**Обновлено:** 2026-06-09

## Контекст

Интеграция AI-чатов в Lavender Messenger: OWL AI и Hermes Orchestrator.

**Текущая ветка:** `feat/1.1.1.x` (оба репозитория)
**Сервер:** dev на порту 50052, prod на 50051

---

## Архитектура

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

## Статус: v1.1.1.15 ЗАВЕРШЕНА

### Сервер v1.1.1.15 (`/root/msg`)
- ServerVersion 1.1.1.15
- Таблица `free_openrouter_models` — управляемый список бесплатных моделей
- RPC `GetFreeModels`, `SetFreeModel` (admin), `RemoveFreeModel` (admin)
- `GetOwlSettings` возвращает `free_models` в ответе
- Proto: FreeModelInfo, GetFreeModelsRequest/Response, SetFreeModelRequest/Response
- Dev deployed и работает

### Android v1.1.1.15 (`/root/msg.client.android`)
- version.txt 1.1.1.15, changelog.txt обновлён
- Бесплатные модели загружаются с сервера (GetFreeModels RPC)
- Без ключа: только бесплатные модели, OWL Alpha первая
- С ключом: бесплатные + «Своя модель» (текстовый ввод)
- Поле «Своя модель» скрыто без ключа + подсказка
- Favorites flickering fix: startSync() + updateAvatarCache() offset fix
- compileDebugKotlin passes

---

## Статус: v1.1.1.14 ЗАВЕРШЕНА

### Сервер v1.1.1.14 (`/root/msg`)
- ServerVersion 1.1.1.14 — version bump
- Серверных изменений нет, все фичи v1.1.1.13 работают
- Dev deployed и работает

### Android v1.1.1.14 (`/root/msg.client.android`)
- version.txt 1.1.1.14, changelog.txt обновлён
- Анимации сообщений (fade-in + slide), typing indicator (ValueAnimator)
- Bottom sheets полировка (MaterialCardView, hover-эффекты, per-command иконки)
- Splash screen анимация, statusBarColor = bgColor
- compileDebugKotlin passes

---

## История версий (кратко)

| Версия | Что сделано |
|--------|------------|
| v1.1.1.14 | Дизайн + полировка UI (анимации, typing, bottom sheets, splash) |
| v1.1.1.13 | Полное тестирование всех фич |
| v1.1.1.12 | Bugfix: messages disappearing, unread counter, CommandBottomSheet |
| v1.1.1.11 | Key/model banner в шапке AI чатов, robot icon |
| v1.1.1.10 | Per-chat settings, rate limiting (free 20/hr), AIBottomSheet redesign |
| v1.1.1.9 | Graceful reconnect (exponential backoff, keep-alive) |
| v1.1.1.8 | Participants UUID, GetAIChats/RenameAIChat RPCs |
| v1.1.1.7 | Notification badge, GetUnreadCount RPC |
| v1.1.1.6 | Множественные OWL/Hermes чаты с нумерацией |
| v1.1.1.5 | OwlSettingsActivity, creator_id миграция |

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично)
- Favorites мерцание при обновлении списка чатов (DiffUtil пересоздаёт Favorites)

---

## Что НЕ сделано (по приоритету)

### Перед деплоем на prod
1. ~~Тестирование v1.1.1.11~~ ✅
2. ~~Bug fix: messages disappearing~~ ✅ v1.1.1.12
3. ~~Bug fix: unread counter~~ ✅ v1.1.1.12
4. ~~Полное тестирование v1.1.1.13~~ ✅
5. ~~Дизайн + полировка v1.1.1.14~~ ✅
6. **Деплой на prod → v1.1.2.0** — после подтверждения стабильности

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## Правила работы

1. Коммитить после каждого значимого изменения
2. Пушить в `feat/1.1.1.x` (не в main!)
3. Деплоить на dev сервер для тестирования
4. Обновлять CHANGELOG.md с каждым фиксом
5. Не ломать существующий функционал
6. Версия сервера в `server.go:33` — всегда обновлять при релизе
7. Разделение архитектуры — каждый AI-сервис в своём файле
8. userId (UUID) — всегда как ключ, НЕ username

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
# assembleRelease НЕ запускать на сервере — OOM
```

---

## Важно

- НЕ использовать `--go_out=.` при proto gen (генерирует в корень, ломает сборку)
- go PATH: `export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- Dev DB: `chat_db_dev` (порт 5432, user: lavender)
- Prod DB: `chat_db` (порт 5432, user: lavender)
- Dev config: `/root/LavenderMessenger/run/.env.dev`
- Prod config: `/root/LavenderMessenger/run/.env`

---

## Промпт для следующей сессии (v1.1.1.16 — Тестирование + багфикс перед релизом)

```
Продолжаем работу над Lavender Messenger. v1.1.1.15 завершена:
- Бесплатные модели загружаются с сервера (таблица free_openrouter_models)
- Без ключа: только бесплатные модели, OWL Alpha первая
- С ключом: бесплатные + «Своя модель» (текстовый ввод ID)
- Favorites flickering fix
- Dev сервер обновлён и работает
- compileDebugKotlin проходит

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория на ветке feat/1.1.1.x
- v1.1.1.15 тег на обоих репозиториях

Текущая версия: v1.1.1.15

Что нужно сделать (v1.1.1.16):
1. ТЕСТИРОВАНИЕ ФИШЕК (end-to-end на dev сервере)
2. ИСПРАВЛЕНИЕ НАЙДЕННЫХ БАГОВ
3. Если багов нет → деплой на prod → v1.1.2.0

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL
- HermesGrpc.kt — отдельный файл для Hermes
- НЕ смешивать OWL и Hermes код — полная изоляция
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования (сервер)
- Обновлять CHANGELOG.md (новая версия наверху)
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при релизе
- Дизайн — минималистичный, чистый, без лишнего декора

Документация:
- Индекс: /root/msg/doc/INDEX.md (читать в начале сессии)
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
```

```
Продолжаем работу над Lavender Messenger. v1.1.1.14 завершена:
- Дизайн + полировка UI: анимации сообщений, typing indicator, bottom sheets, splash screen
- Dev сервер обновлён и работает
- compileDebugKotlin проходит

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория на ветке feat/1.1.1.x
- v1.1.1.14 тег на обоих репозиториях

Текущая версия: v1.1.1.14

Что нужно сделать (v1.1.1.15):

1. ТЕСТИРОВАНИЕ ФИШЕК (end-to-end на dev сервере):
   - OWL AI: отправить сообщение → получить стриминг ответ
   - Hermes Orchestrator: отправить сообщение → проверить маршрутизацию к агенту
   - Бот-команды: /status, /help, /version, /ai — все должны работать
   - Rate limiting: проверить что лимиты срабатывают
   - Per-chat settings: задать свой ключ → проверить что используется
   - Notifications: подписка → получение → mark as read → счётчик
   - Graceful reconnect: убить сеть → восстановить → проверить переподключение
   - Множественные чаты: создать 3 OWL + 3 Hermes → проверить нумерацию
   - Long-press на чат → PopupMenu (Настройки / Удалить)
   - Key/model banner в шапке AI чатов
   - Проверить анимации сообщений и typing indicator

2. ИСПРАВЛЕНИЕ НАЙДЕННЫХ БАГОВ

3. Если багов нет → деплой на prod → v1.1.2.0

Архитектура (важно!):
- OwlGrpc.kt — отдельный файл для OWL
- HermesGrpc.kt — отдельный файл для Hermes
- НЕ смешивать OWL и Hermes код — полная изоляция
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.1.x
- Деплоить на dev для тестирования (сервер)
- Обновлять CHANGELOG.md (новая версия наверху)
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при релизе
- Дизайн — минималистичный, чистый, без лишнего декора

Документация:
- Индекс: /root/msg/doc/INDEX.md (читать в начале сессии)
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
```
