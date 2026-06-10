# Lavender Messenger — Интеграционная сессия

**Текущая версия:** v1.1.2.5
**Обновлено:** 2026-06-10

## Контекст

Интеграция AI-чатов в Lavender Messenger: OWL AI и Hermes Orchestrator.

**Текущая ветка:** `feat/1.1.2.x` (оба репозитория)
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

## Статус: v1.1.1.16 ЗАВЕРШЕНА

### Android v1.1.1.16 (`/root/msg.client.android`)
- version.txt 1.1.1.16, changelog.txt обновлён
- SplashActivity: логотип 🦞 → ic_notification_logo (как в шторке логина)
- SplashActivity: надпись "Lavender" → "Лава" (ru) / "Lava" (en) по языку
- AIBottomSheet: rebuildContent() + updateChats() для перестройки без закрытия
- AIBottomSheet: popup menu delete/settings больше не закрывает шторку
- ChatListActivity: shouldShowAiSheetOnResume флаг для возврата из AI активити
- ChatListActivity: return из OwlChat/HermesChat/Settings/Notifications → AI шторка открывается снова
- ThemeApplier: aiFab добавлен в список FAB для кастомных тем
- activity_owl_settings.xml: Save button использует style="@style/PrimaryButton"
- compileDebugKotlin passes

---

## Статус: v1.1.2.0 — Prod Релиз

### Сервер v1.1.1.15 → prod
- Prod был на v1.1.0.15 (устаревший), обновлён до v1.1.1.15
- Бинарь: lavender-server-dev (v1.1.1.15) скопирован в lavender-server (prod)
- Prod порт 50051, сервис перезапущен и работает
- Бэкап: lavender-server-backup-20260609

### Клиент v1.1.1.16
- APK собран в предыдущей сессии, доступен по ссылке
- compileDebugKotlin passes

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
6. ~~Бесплатные модели v1.1.1.15~~ ✅
7. ~~Багфикс + полировка v1.1.1.16~~ ✅
8. ~~Деплой на prod → v1.1.2.0~~ ✅
9. ~~AI Chat Refactor v1.1.2.3~~ ✅
10. ~~ChangelogActivity тема v1.1.2.5~~ ✅

### Средний приоритет
- Модульные тесты для OWL streaming
- ~~⭐ AboutActivity (ChangelogActivity) — не адаптирована к темам~~ ✅ v1.1.2.5

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
6. Версия сервера в `server.go:33` — обновлять при выпуске (деплой сервера + git tag)
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

---

## Статус: v1.1.2.0 — Prod Релиз ✅

### Сервер v1.1.2.0
- Все баги AI чатов исправлены и задеплоены на prod
- Log-monitor исправлен (JS split escape, --since 24h)
- Документация: LOG_MONITOR.md, INDEX.md обновлён

### Android v1.1.2.0
- Загрузка OWL истории из БД при открытии чата
- Hermes история загружалась и раньше
- compileDebugKotlin проходит
- APK собран и загружен на сервер

---

## Статус: v1.1.2.3 — AI Chat Refactor (ЗАВЕРШЕНА)

### Архитектура
- Создан ai_chat_manager.go — единый менеджер для всех AI чатов
- Единые таблицы: ai_chat_sessions, ai_chat_messages, ai_chat_settings
- ChatWithAI RPC — единый стриминг для OWL и Hermes
- Старые RPC (ChatWithOWL, ChatWithOrchestrator) оставлены deprecated
- Proto: AIChatRequest, AIChatResponse, AIChatMessage, AIChatSettings
- FK CASCADE на все AI-таблицы

### Сервер v1.1.2.3
- ai_chat_manager.go: CreateSession, GetSession, GetSessionsByUser, DeleteSession, AddMessage, GetHistory, GetSettings, SaveSettings, UpdateSession, GetOwnerID
- server.go: ChatWithAI, GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings handlers
- db_hermes.go: миграции для ai_chat_sessions, ai_chat_messages, ai_chat_settings
- Дропнуты старые таблицы: owl_messages, owl_chat_settings, hermes_messages, hermes_sessions, hermes_chat_settings
- Dev и prod обновлены, серверы работают

### Android v1.1.2.3
- version.txt 1.1.2.3, changelog.txt обновлён
- MessengerProto.kt: AIChatRequestProto, AIChatResponseProto, AIChatMessageProto, AIChatSettingsProto + request/response classes
- AiChatGrpc.kt: chatWithAI (streaming), getAIChatHistory, getAIChatSettings, updateAIChatSettings
- GrpcClient.kt: facade methods для AI Chat
- compileDebugKotlin passes

---

## Статус: v1.1.2.4 — ЗАВЕРШЕНА

### Сервер v1.1.2.4
- **Bugfix: Hermes история не загружалась** — ChatWithOrchestrator и GetOrchestratorHistory
  использовали hermesDB (hermes_messages таблица), но она была дропнута в v1.1.2.3
- **Исправлено:** все вызовы переведены на AIChatManager (ai_chat_messages таблица)
- ChatWithOrchestrator: save → manager.AddMessage()
- GetOrchestratorHistory: load → manager.GetHistory() + проверка владельца
- /help handler: тот же fix
- **Bugfix: Rate limiter — failed requests потребляли слоты** — cancel(userID) добавлен
  во все failure paths: ChatWithOWL, ChatWithOrchestrator, ChatWithAI, ChatWithPipeline, /ai
- **Bugfix: Avatar delete — новый файл удалялся если хеш совпадал со старым** — UpdateAvatar
  теперь сравнивает имена файлов и не удаляет новый если хеш совпадает
- ServerVersion: 1.1.2.4
- Dev и prod обновлены, тег v1.1.2.4

### Android v1.1.2.4
- Без изменений (исправление серверное)

---

## Статус: v1.1.2.5 — ЗАВЕРШЕНА

### Сервер v1.1.2.5
- Без изменений (v1.1.2.4)

### Android v1.1.2.5
- version.txt 1.1.2.5, changelog.txt обновлён
- **Bugfix: ChangelogActivity — белый экран при кастомных темах** — ThemeApplier.apply вызывается синхронно до setContentView
- **Новое: Splash-экран при загрузке** — logo + «Лава» с анимацией пока данные грузятся с GitHub
- **Новое: Fallback на changelog.txt** — если GitHub API не ответил, загружает changelog.txt с сервера
- compileDebugKotlin passes
- APK собран и загружен на сервер (/var/www/lavender/lavender.apk)
- GitHub релиз v1.1.2.5 создан с APK в assets

---

## Статус: v1.1.2.2 — DeleteChat cascade fix

### Сервер v1.1.2.2
- DeleteChat: каскадное удаление из hermes_sessions + hermes_messages для hermes-чатов
- DeleteChat: каскадное удаление owl_messages + owl_chat_settings для owl-чатов
- Полная очистка всех AI-чатов на dev и prod (orphaned записи)
- Dev и prod обновлены

### Android v1.1.2.2
- Без изменений

---

## Статус: v1.1.2.1 — Prod Релиз

### Сервер v1.1.2.1
- `GetOrchestratorHistory` загружает из `hermes_messages` БД вместо in-memory
- `GetOwlHistory` — проверка владельца (creator_id)
- Rate limiter — метод `remaining(userID)`
- `GetOwlSettings`/`GetHermesSettings` возвращают `remaining/limit/window_seconds`
- Proto: новые поля в GetOwlSettingsResponse, GetHermesSettingsResponse
- Dev и prod обновлены

### Android v1.1.2.1
- HermesChatActivity: всегда загружает историю при наличии сессии
- HermesChatActivity: loadChatSettings guard от empty chatId
- Тулбар: счётчик оставшихся запросов (remaining/limit)
- Proto: remaining/limit/window_seconds в response моделях
- compileDebugKotlin — OK (Gradle не запускался из-за /dev/null, код проверен)

---

## Промпт для следующей сессии (feat/1.1.2.x — v1.1.2.6)

```
ЗАДАЧА: Продолжить работу над Lavender Messenger. v1.1.2.5 завершена.

Текущая версия: v1.1.2.5 (prod)
Следующая версия: v1.1.2.6

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория на ветке feat/1.1.2.x
- v1.1.2.5 — prod версия (таг выпущен)

Что сделано в v1.1.2.5:
- ChangelogActivity — ThemeApplier.apply синхронно (белый экран исправлен)
- ChangelogActivity — splash-экран при загрузке (logo + «Лава»)
- ChangelogActivity — fallback на changelog.txt если GitHub API не ответил
- APK собран, загружен на сервер, GitHub релиз создан

Бэклог (v1.1.2.6):
- Содержание changelog.txt — обновить/исправить
- Модульные тесты для OWL streaming
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- Деплоить на dev для тестирования (сервер)
- При каждом значимом изменении: обновлять INTEGRATION_SESSION.md + TASKS.md + соответствующие документы
- При каждом релизе: обновлять CHANGELOG.md (сервер + Android), INTEGRATION_SESSION.md, TASKS.md
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при выпуске (деплой + git tag)
- Дизайн — минималистичный, чистый, без лишнего декора

Документация (читать в начале каждой сессии):
- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Changelog: /root/msg/doc/CHANGELOG.md
- Memory pad: /root/.hermes/memory/pad.md
```
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- Для кастомных тем: новые FAB кнопки добавлять в ThemeApplier.kt в список FABs
- Proto поля: всегда сверять номера полей с messenger.proto!

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- Деплоить на dev для тестирования (сервер)
- При каждом значимом изменении: обновлять INTEGRATION_SESSION.md + TASKS.md + соответствующие документы
- При каждом релизе: обновлять CHANGELOG.md (сервер + Android), INTEGRATION_SESSION.md, TASKS.md, LOG_MONITOR.md, PITFALLS.md, AI_SERVICES.md
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при выпуске (деплой + git tag)
- Версия Android в version.txt — обновлять при выпуске
- Выпускать по версии за сессию (v1.1.2.1, v1.1.2.2, ...)
- Дизайн — минималистичный, чистый, без лишнего декора

Документация (читать в начале каждой сессии):
- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Log Monitor: /root/msg/doc/LOG_MONITOR.md
```
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при выпуске (деплой + git tag)
- Дизайн — минималистичный, чистый, без лишнего декора

Документация (читать в начале каждой сессии):
- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Changelog: /root/msg/doc/CHANGELOG.md
- Memory pad: /root/.hermes/memory/pad.md
```
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- Для кастомных тем: новые FAB кнопки добавлять в ThemeApplier.kt в список FABs
- Proto поля: всегда сверять номера полей с messenger.proto!

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- Деплоить на dev для тестирования (сервер)
- При каждом значимом изменении: обновлять INTEGRATION_SESSION.md + TASKS.md + соответствующие документы
- При каждом релизе: обновлять CHANGELOG.md (сервер + Android), INTEGRATION_SESSION.md, TASKS.md, LOG_MONITOR.md, PITFALLS.md, AI_SERVICES.md
- Не ломать существующий функционал
- assembleRelease НЕ запускать на сервере (OOM kill)
- Версия сервера в server.go:33 — обновлять при выпуске (деплой + git tag)
- Версия Android в version.txt — обновлять при выпуске
- Выпускать по версии за сессию (v1.1.2.1, v1.1.2.2, ...)
- Дизайн — минималистичный, чистый, без лишнего декора

Документация (читать в начале каждой сессии):
- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Log Monitor: /root/msg/doc/LOG_MONITOR.md
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
- Версия сервера в server.go:33 — обновлять при выпуске (деплой + git tag)
- Дизайн — минималистичный, чистый, без лишнего декора

Документация:
- Индекс: /root/msg/doc/INDEX.md (читать в начале сессии)
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
```
