# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.1.3.2
**Обновлено:** 2026-06-12
**Тег:** v1.1.3.2 (выпущен)

## Контекст

Интеграция AI-чатов в Lava Messenger: OWL AI и Hermes Orchestrator.
Platform Adapter для Hermes Agent — плагин-адаптер подключает Lavender
к Hermes экосистеме через bidirectional gRPC streaming.

**Текущая ветка:** `feat/1.1.3.x`
**Сервер:** dev на порту 50052, prod на 50051

---

## Архитектура

```
СЕРВЕР:
├── server.go           — структура server, общие методы
├── server_chat.go      — Chat, Typing, CallSession, GetClients
├── server_users.go     — GetAllUsers, UpdateProfile, GetUserProfile, GetUserAvatar
├── server_chats.go     — GetAllChats, GetChats, CreateDirectChat, CreateGroupChat, DeleteChat, etc.
├── server_messages.go  — GetHistory, SetReaction, DeleteMessages, EditMessage
├── server_profile.go   — UpdateUsername, UpdatePassword, AdminUpdatePassword, MarkRead, UpdateAvatar, DeleteProfile
├── server_push.go      — RegisterToken, sendPushNotification, broadcastOnlineUsers, etc.
├── server_contacts.go  — AddContact, RemoveContact, GetContacts, GetChatListVersion
├── server_themes.go    — GetThemes, SaveTheme, SetCurrentTheme, DeleteTheme
├── server_drafts.go    — GetFCMLogs, SaveDraft, GetDraft, DeleteDraft
├── server_muted.go     — GetMutedChats, SetMutedChat
├── server_favorites.go — GetUserId, AddFavorite, RemoveFavorite, GetFavorites, etc.
├── server_ai.go        — ChatWithOWL, ChatWithAI, ChatWithOrchestrator, Hermes sessions, etc.
├── server_management.go — ServerServiceServer
├── auth_service.go     — AuthService: SignIn, SignUp
├── owl.go              — OWL AI: ChatWithOWL streaming, сессии, история
├── bot_commands.go     — Bot Commands: /status, /deploy, /logs, /restart, /ai, /help, /version
├── hermes_orchestrator.go — Hermes: оркестратор, маршрутизация агентов
├── hermes_agent_service.go — Hermes: управление агентами
└── db.go               — Database layer
```
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
2. Пушить в `feat/1.1.2.x` (не в main!)
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

## Статус: v1.1.2.7 — ЗАВЕРШЕНА

### Сервер v1.1.2.7
- Без изменений (v1.1.2.4)

### Android v1.1.2.7
- **SplashActivity**: увеличено расстояние логотип→текст (60px → 90dp)
- **SplashLoadingActivity**: новый оверлей загрузки для логина/регистрации
- **Login/Register**: показывается SplashLoadingActivity во время авторизации
- **Онбординг удалён**: welcomeContainer, onboardingProfileBubble, onboardingFabBubble
- **Чекбокс "Создать чат"**: в шторке добавления контакта, включён по умолчанию
- **Исправления**: crash при выборе чатов, getSelectedChats offset, loadingContainer удалён, statusBarColor deprecation
- compileDebugKotlin passes
- APK собран и загружен на сервер (/var/www/lavender/lavender.apk)
- GitHub релиз v1.1.2.7 создан с APK

---

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

## Статус: v1.1.2.6 — Auth токены для удалённых агентов (JWT) ЗАВЕРШЕНА

### Сервер v1.1.2.6
- **JWT аутентификация** для hermes-agent daemon при подключении к Orchestrator
- `auth/jwt.go` — генерация и валидация HS256 JWT токенов
- Claims: agent_id, agent_name, capabilities, iat, exp
- Таблица `agent_tokens` в БД (SHA-256 хеш, не сам токен)
- 3 новых admin RPC: `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens`
- `validateToken()` — полная проверка: подпись, expiration, agent_id match, revoked в БД
- Секрет из `JWT_SECRET` env (32+ байта)
- Dev и prod обновлены

### Android v1.1.2.6
- **Bundled changelog**: `app/src/main/assets/changelog_bundled.txt` — встроен в APK, показывается мгновенно без сети
- **Новая логика загрузки**: bundled (мгновенно) → GitHub API → server fallback
- **Ссылки на CHANGELOG.md**: кнопки «Ченджлог сервера (GitHub)» и «Ченджлог клиента (GitHub)»
- **changelog.txt удалён** из проекта и из деплоя на сервер
- **scripts/deploy_android.sh обновлён**: убрана загрузка changelog.txt
- **Старый deploy_android.sh удалён** (сервер 159.195.38.145 больше не поддерживается)
- **Документация обновлена**: INDEX.md, PITFALLS.md, TASKS.md
- compileDebugKotlin passes

---

## Статус: v1.1.2.10 — Рефакторинг server.go (ЗАВЕРШЕНА)

### Сервер v1.1.2.10
- **Рефакторинг server.go** — разбит на 12 файлов по доменам (server_*.go)
- Каждый файл ~300-600 строк вместо одного файла 4268 строк
- server_management.go — ServerServiceServer восстановлен
- Dev сервер обновлён и работает, старые клиенты не сломались

---

## Статус: v1.1.2.9 — AuthService (ЗАВЕРШЕНА)

### Сервер v1.1.2.9
- **AuthService** — отдельный gRPC сервис для аутентификации
- `auth_service.go` — реализация `AuthServiceServer` с методами `SignIn` и `SignUp`
- `SignIn` — проверка username/password через bcrypt, возврат UUID-токена и User
- `SignUp` — регистрация с проверкой уникальности username/email
- Proto: `User`, `SignInRequest`, `SignUpRequest`, `AuthResponse`, `AuthService`
- `db.go` — `SaveUserWithEmail` метод
- `main.go` — `gen.RegisterAuthServiceServer(s, authServer)`
- Dev сервер обновлён и работает

---

## Статус: v1.1.2.8 — AI чат улучшения (ЗАВЕРШЕНА)

### Android v1.1.2.8
- **Убран прелоадер** во время ожидания ответа агента (HermesChatActivity, OwlChatActivity)
- **Таймаут стрима 120 сек** с сбросом при каждом сообщении (OwlGrpc, HermesGrpc)
- **Шторка AI реорганизована**: чаты разделены по типам (Hermes / OWL)
- **Favorites исправлен**: показывается сразу в onCreate(), fallback при ошибке загрузки
- **ChangelogAdapter**: цвета из ThemeStore, GitHub API загружается первым
- **Контакты**: убран deprecated overridePendingTransition
- compileDebugKotlin passes
- APK: /var/www/lavender/lavender.apk
- GitHub релиз: https://github.com/ferzferz11-sudo/msg.client.android/releases/tag/v1.1.2.8
- Тег v1.1.2.8 создан и запушен

### Сервер v1.1.2.8
- Без изменений (v1.1.2.6)

---

## Промпт для следующей сессии (feat/1.1.3.x — v1.1.3.5)

```
ЗАДАЧА: Продолжить работу над Lava Messenger. v1.1.3.5 в разработке.

Текущая версия: Сервер v1.1.3.4, Android v1.1.3.5
Ветка: feat/1.1.3.x

Контекст:
- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg.client.android
- Оба репозитория на ветке feat/1.1.3.x

Что сделано в последних коммитах (Remote Agent chat):
- UI fix: send button, command menu, auto-scroll — commit ee5e115
- AuthService unit tests (10 tests + benchmarks) — commit c9b3b14
- Fix "Агент не выбран" — auto-select agent in sendMessage — commit 4cfa8be
- Фильтрация токенов по пользователю — уже реализована на сервере
- Dev сервер запущен и работает

ЗАДАЧИ НА ЭТУ ССЕССИЮ:

## 1. Модульные тесты для OWL streaming

Файл: owl_test.go

Тесты:
- TestChatWithOWL_Success — отправка сообщения → получение ответа через stream
- TestChatWithOWL_RateLimit — превышение лимита → ошибка rate limit
- TestChatWithOWL_EmptyMessage — пустое сообщение → ошибка валидации
- TestGetOwlHistory_ReturnsMessages — история чата возвращает сообщения из БД

Требования:
- Мокать OpenRouter API (не делать реальные запросы)
- Использовать httptest.Server для мока API
- Тесты должны быть быстрыми (< 5 сек на тест)
- Покрытие > 70% для owl.go

## 2. Streaming результатов задач агентом обратно клиенту

Добавить server-side streaming RPC:
- Новый proto RPC: DeployAgentTaskStream(DeployAgentTaskRequest) returns (stream DeployAgentTaskResponse)
- Сервер отправляет промежуточные результаты по мере выполнения
- Android клиент подписывается на поток результатов

ОБЩИЕ ТРЕБОВАНИЯ:
- go test ./... должен проходить без ошибок
- Тесты не должны зависеть от внешних сервисов
- Использовать t.Parallel() где возможно
```
- TestChatWithOWL_Success — отправка сообщения → получение ответа через stream
- TestChatWithOWL_RateLimit — превышение лимита → ошибка rate limit
- TestChatWithOWL_EmptyMessage — пустое сообщение → ошибка валидации
- TestGetOwlHistory_ReturnsMessages — история чата возвращает сообщения из БД
- TestGetOwlHistory_Unauthorized — неавторизованный доступ → ошибка

Требования:
- Мокать OpenRouter API (не делать реальные запросы)
- Использовать httptest.Server для мока API
- Тесты должны быть быстрыми (< 5 сек на тест)
- Покрытие > 70% для owl.go

ОБЩИЕ ТРЕБОВАНИЯ:
- go test ./... должен проходить без ошибок
- Тесты не должны зависеть от внешних сервисов (OpenRouter, Firebase)
- Использовать t.Parallel() где возможно
- Добавить бенчмарки для критичных функций (SignIn, SignUp)

Известные проблемы (не исправлено):
- Сообщения пользователя видны только после ответа агента (нужна отладка на устройстве)

Бэклог (после тестов):
- Structured logging на сервере (низкий)
- Graceful shutdown для gRPC сервера (низкий)
- Health check endpoint (низкий)
- Prometheus метрики (низкий)
- Qdrant + CLIP (production RAG) — ночная задача

Правила:
- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- При каждом релизе: git tag, CHANGELOG.md, bundled, version.txt
- assembleRelease НЕ запускать на сервере (OOM kill)
- Дизайн — минималистичный, чистый
- userId (UUID) — всегда как ключ, НЕ username
- Для кастомных тем: новые FAB добавлять в ThemeApplier
- Статический first item (Favorites) добавлять ДО загрузки данных с сервера
- Новые методы сервера класть в соответствующий server_*.go файл (не в server.go)

Документация (читать в начале каждой сессии):
- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg.client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Changelog: /root/msg/CHANGELOG.md
- Memory pad: /root/.hermes/memory/pad.md
```

### Важно (changelog)
- **changelog.txt УДАЛЁН** из проекта и из деплоя (v1.1.2.6)
- Вместо него: `app/src/main/assets/changelog_bundled.txt` — встроен в APK
- При каждом релизе: обновлять `assets/changelog_bundled.txt` вместе с `CHANGELOG.md`
- Формат: emoji-заголовки, буллеты `—`, секции по версиям

### Важно (архитектура)
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- Для кастомных тем: новые FAB кнопки добавлять в ThemeApplier.kt в список FABs
- Proto поля: всегда сверять номера полей с messenger.proto!
- JWT секрет: `JWT_SECRET` в .env, минимум 32 байта, НЕ коммитить
- Agent tokens: в БД хранится SHA-256 хеш, не сам токен
- Admin RPC: `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — требуют IsSuperAdmin()

---

## Статус: v1.1.3.6 — ЗАВЕРШЕНА (выпущена)

### Android v1.1.3.6
- **Remote Agent UI redesign** — TabLayout в настройках (Шлюз/Токен)
- **Gateway tab** — SSH туннель форма скрывается при подключении, показывает IP шлюза
- **Token tab** — генерация JWT токена, start/stop агента, список токенов
- **Инструкции** для обоих режимов подключения
- **Статус на тулбаре чата** — тип подключения (шлюз IP / токен)
- **Start/Stop кнопки** в статус-баре чата
- **Persist selected agent** в SharedPreferences
- **Команды агента** — быстрые команды через кнопку в поле ввода
- **Авто-прокрутка чата**
- **Changelog обновлён** (ориентир на улучшения, не исправления)

### Сервер v1.1.3.4 (без изменений)
- **AuthService unit tests** — 10 tests + benchmarks

---

## Промпт для следующей сессии (v1.1.3.x — Remote Agent improvements)

**Версия:** v1.1.3.2 | **Ветка:** feat/1.1.3.x

### Что сделано
- ✅ JWT токены — генерация, список, отзыв, копирование
- ✅ Запуск/остановка агента через StartAgent/StopAgent RPC
- ✅ UI статуса подключения (зелёный/белый)
- ✅ Персистентность выбранного агента
- ✅ HermesGrpc — все методы реализованы
- ✅ Debug логи обёрнуты в BuildConfig.DEBUG

### Известные проблемы (P1)
- **Агент завершается сразу после запуска** — `hermes_remote_agent.py` падает в `connect()` при отправке `AgentMessage` через gRPC stream. Root cause: protobuf marshaling. Нужно исправить Python скрипт.
- **Токены не фильтруются по пользователю** — `ListAgentTokens` возвращает все токены. Нужно фильтровать по `created_by` на сервере.

### Задачи на следующую сессию
1. **Исправить hermes_remote_agent.py** — агент должен оставаться подключённым и выполнять задачи
2. **Фильтрация токенов** — `ListAgentTokens` должен возвращать только токены текущего пользователя
3. **Streaming результатов задач** — отправка результатов выполнения задач агентом обратно клиенту

### Критические файлы
- `app/src/main/java/lavender/client/android/data/grpc/HermesGrpc.kt` — все gRPC методы
- `app/src/main/java/lavender/client/android/ui/remote/RemoteAgentSettingsActivity.kt` — UI настроек
- `app/src/main/java/lavender/client/android/ui/remote/RemoteAgentActivity.kt` — UI чата с агентом
- `hermes-agent/hermes_remote_agent.py` — Python агент (сервер)

### Правила
- НЕ запускать assembleRelease на сервере (OOM kill)
- НЕ запускать compileDebugKotlin без крайней необходимости
- version.txt обновлять ДО release.sh
- Коммитить и пушить после каждого значимого изменения
