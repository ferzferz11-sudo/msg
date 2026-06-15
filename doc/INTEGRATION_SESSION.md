# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.2.0.1 (сервер dev) / v1.1.3.16 (Android)
**Обновлено:** 2026-06-16 (сессия 12)
**Тег:** v1.1.3.15 (stable prod)
**Ветка сервера:** feat/1.2.0.x
**Ветка Android:** feat/1.1.3.x

---

## Сессия 12 — ChatList v2 UI + разделение v1/v2

### Что сделано

#### ChatList v2 UI (Android)
1. **ChatListActivityV2** — новый Activity с определением версии сервера (v1/v2)
2. **ChatListFragmentV2** — фрагмент с SwipeRefresh + RecyclerView
3. **ChatAdapterV2** — адаптер с секциями (Pinned/Favorites/All Chats)
4. **ChatListViewModelV2** — ViewModel: loadChats, pinChat, archiveChat, searchChats
5. **ChatListSections.kt** — управление секциями
6. **TabLayout** — табы All / AI / Groups (заглушка)
7. **v2 context menu** — Pin/Mute/Delete в списке чатов (long press)
8. **Fallback на v1** — при подключении к prod серверу автоматически запускается ChatListActivity v1
9. **i18n** — 17 новых строк (en + ru)

#### Архитектура v2 (уточнено, сессия 13)
- **Long press на чате** = режим выбора (toolbar с действиями: Pin/Delete/Edit)
- **Короткий тап** = вход в чат/группу
- **Pin Chat** — в toolbar в режиме выбора (long press)
- **Pin Message** — в шторке сообщения (bottom sheet)
- **Archive** — отдельная сущность, заархивированные но не удалённые чаты
- **Favorites** — существующий чат "Личное хранилище" (не Archive!)
- **Секции списка**: Pinned / Favorites / All Chats + Archived
- **Табы**: All / AI / Groups

#### Исправления
- Data binding NPE: `@++id/` → `@+id/`, ConstraintLayout в CoordinatorLayout, несуществующий TextAppearance
- Компиляция: parseSafeColor defaultColor, ThemeApplier.apply signature, ServerAuthBottomSheet params

#### Коммиты
- `7d087bc` — v2 scaffold
- `0f500ce` — fix ConstraintLayout attrs
- `23a2a79` — fix TextAppearance
- `6fb3453` — fix @++id/
- `28c2715` — fix compilation errors
- `f0b06e1` — restore version.txt to 1.1.3.15
- `bf00543` — remove unused menu, restore i18n
- `e338ed4` — docs: session 12 wrap-up

---

## Сессия 11 — ChatStream v2 + ChatList v2

### Что сделано

#### ChatStream v2 (сервер)
1. **messenger.proto** — добавлен `jwt_token` (field 26) в Message для ChatStream v2 auth
2. **server_chat.go** — Chat stream поддерживает оба метода auth:
   - `jwt_token` (v2): валидация JWT, извлечение user_id/username из claims
   - `password` (v1): полная обратная совместимость
3. **ChatServiceVersion** = "2.0" в server.go

#### ChatList v2 (сервер)
1. **messenger.proto** — добавлены RPC методы: PinChat, UnPinChat, SearchChats, ArchiveChat, UnarchiveChat
2. **messenger.proto** — добавлены `is_pinned`, `is_muted`, `is_archived`, `pinned_at` в ChatInfo
3. **messenger.proto** — добавлены `limit`, `offset`, `filter` в GetChatsRequest (пагинация)
4. **server_chatlist_v2.go** — реализация всех новых RPC методов
5. **db_chatlist_v2.go** — миграции (user_chat_metadata: pinned/pinned_at/archived), методы DB
6. **server_chats.go** — GetAllChats включает v2 поля

#### ChatStream v2 + ChatList v2 (Android)
1. **ProfileClient** — `fetchServerInfo()` парсит все версии сервисов (chat/auth/profile/ai)
   - Добавлены `isChatV2Supported()`, `isAuthV2Supported()`
   - Fallback на v1 если /info недоступен
2. **BearerTokenInterceptor** — убран пропуск Chat stream для v2 серверов
3. **RealGrpcClient.startChat()** — JWT token для v2, password для v1
4. **RealGrpcClient** — `pinChat()`, `unpinChat()`, `searchChats()`, `archiveChat()`, `unarchiveChat()`
5. **GrpcClient** — публичные методы ChatList v2
6. **MessengerProto.kt** — новые proto classes, обновлён ChatInfoProto
7. **ChatInfo model** — `isPinned`, `isArchived`, `pinnedAt`
8. **MessageProtoMarshaller** — сериализация/deserialization jwt_token (field 26), isE2Ee, e2EePayload

### Коммиты (сервер)
- `0daf87b` — feat: ChatStream v2 (JWT auth) + ChatList v2 (Pin/Search/Archive) + proto updates
- `840a708` — chore: fix server version to 1.2.0.1
- `de3d55d` — docs: update version to v1.2.0.1, branch to feat/1.2.0.x

### Коммиты (Android)
- `cd2294d` — feat: ChatStream v2 + ChatList v2 Android client
- `cc759b7` — fix: add jwtToken to MessageProto, fix searchChats mapping
- `a4a29ae` — fix: wrap suspendCancellableCoroutine in try-catch
- `bfe0412` — fix: use expression body for suspendCancellableCoroutine
- `f15500f` — fix: use explicit CancellableContinuation type parameter
- `ff6bba2` — fix: use explicit imports and no generics in lambda
- `cb1cf84` — fix: use kotlinx.coroutines.suspendCancellableCoroutine
- `8731367` — fix: add onCancellation parameter to cont.resume()
- `5bb47b6` — docs: update CHANGELOG, SESSION_NOTES, PATTERNS

### Pitfalls learned (Kotlin 2.3.21)
- `CancellableContinuation.resume()` требует `onCancellation = {}` параметр
- `import kotlinx.coroutines.suspendCancellableCoroutine` (не `kotlin.coroutines`)
- data class с `repeated` proto полем использует `List<T>` напрямую (не `getXxxList()`)

---

## Сессия 10 — Документация + ProfileClient fixes
(см. подробности в CHANGELOG)

---

## Сессия 9 — ProfileService v2 + Typing/CallSession compat
(см. подробности в CHANGELOG)

---

## Архитектура

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.0.1", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor (unary + streaming)
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц (включая user_settings)
db_chatlist_v2.go          — ChatList v2 DB methods (PinChat, SearchChats, etc.)
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_chatlist_v2.go      — ChatList v2 RPC (PinChat, SearchChats, ArchiveChat, etc.)
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info на 8082/8083)
messenger.proto            — ChatService v2, AuthService v2, ProfileService v2, AI Chat
```

### Android (/root/msg.client.android)
```
ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt    — шторка выбора входа
│   ├── LoginBottomSheet.kt         — шторка входа (prefillUsername)
│   └── RegisterBottomSheet.kt      — шторка регистрации
├── ServersActivity.kt              — управление списком серверов
├── remote/                         — Remote Agent UI
├── chat/widget/ChatWidget.kt       — общий виджет чата
└── adapter/ChatAdapter.kt          — адаптер чатов (clearAll)

data/
├── grpc/BearerTokenInterceptor.kt  — ClientInterceptor для JWT Bearer token (v2: Chat stream)
├── grpc/GrpcClient.kt              — facade (pinChat, searchChats, archiveChat, etc.)
├── grpc/RealGrpcClient.kt          — реализация gRPC (JWT auth, ChatList v2 RPC)
├── grpc/ProfileClient.kt           — ProfileService v2 client + fetchServerInfo
├── auth/AuthManager.kt             — JWT token storage, getBearerToken
├── session/CredentialStore.kt      — credentials + server list + last_username
├── session/SessionManager.kt       — loginV2 (JWT) + loginV1 (legacy fallback)
├── session/UserSession.kt          — accessToken, refreshToken, authMethod
├── models/ErrorHandler.kt          — единый обработчик ошибок
└── proto/MessengerProto.kt         — proto data classes (ChatList v2, jwt_token, etc.)
```

---

## Статус: v1.2.0.1 — DEV / v1.1.3.14 — Android

Сервер v1.2.0.1 работает на dev (порт 50052, HTTP 8083). ProfileService v2 активен. ChatStream v2 (JWT auth) + ChatList v2 (Pin/Search/Archive) реализованы.
Prod сервер: v1.1.3.10 (без ProfileService v2, без ChatStream/ChatList v2).
Android v1.1.3.14 — ChatStream v2 auth + ChatList v2 API + fetchServerInfo с fallback на v1.

---

## Правила работы

1. Коммитить и пушить после каждого значимого изменения
2. Деплоить на dev для тестирования (не на prod!)
3. Обновлять CHANGELOG.md с каждым релизом
4. Не ломать существующий функционал
5. Версия сервера в `server.go:33`, версия Android в `version.txt`
6. userId (UUID) — всегда как ключ, НЕ username
7. Для кастомных тем: новые FAB добавлять в ThemeApplier
8. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
9. JWT секрет: минимум 32 байта, НЕ коммитить
10. Proto поля: всегда сверять номера полей с messenger.proto
11. ProfileService v2 — только dev сервер (APP_ENV=dev). Prod использует legacy ChatService.
12. **Kotlin 2.3.21:** `cont.resume(value, onCancellation = {})` — всегда передавать onCancellation
13. **fetchServerInfo** — всегда использовать для определения версии сервера. Если /info недоступен → v1 fallback.

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

# Сборка и деплой на prod (НЕ делать без тестирования на dev!)
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...

# === ANDROID ===
cd /root/msg.client.android
# assembleRelease ТОЛЬКО локально!
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Имя | Lava Germany dev | Lava Germany |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Systemd | `Environment=APP_ENV=dev` | `Environment=APP_ENV=` (пусто) |
| ProfileService | v2 (JWT) | v1 (legacy ChatService) |
| ChatStream | v2 (JWT + password) | v1 (password only) |
| ChatList | v2 (Pin/Search/Archive) | v1 (basic) |
| Версия | v1.2.0.1 | v1.1.3.10 |

---

## Документация (читать в начале каждой сессии)

- Индекс: `/root/msg/doc/INDEX.md`
- Сервер: `/root/msg/doc/INTEGRATION_SESSION.md`, `/root/msg/doc/TASKS.md`
- Android: `/root/msg.client.android/doc/INDEX.md`, `/root/msg.client.android/doc/PROMPT_ANDROID.md`
- Android заметки: `/root/msg.client.android/doc/SESSION_NOTES.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Remote Agent: `/root/msg.client.android/doc/REMOTE_AGENT.md`
- Паттерны: `/root/msg.client.android/doc/PATTERNS.md`
- CHANGELOG: `/root/msg/CHANGELOG.md` (сервер), `/root/msg.client.android/CHANGELOG.md` (Android)

---

## Промпт для следующей сессии

**Версия:** v1.2.0.1 (сервер dev) / v1.1.3.14 (Android) → следующая v1.2.0.2 / v1.1.3.15

**Ветки:** сервер — feat/1.2.0.x, Android — feat/1.1.3.x (до релиза)

**Приоритеты:**
1. **ChatList v2 UI** — новая ChatListActivity с секциями (Pinned/Favorites/All), табами, search, unread badges
2. **Тесты для ProfileService v2** — unit-тесты (сервер + Android)
3. **Деплой prod сервера** — после завершения Android клиента

**Отложено (не в этой сессии):**
- Редеплой prod сервера — только после выхода Android клиента
- Выпуск Android — делается ферзем лично после завершения v2 UI

**Правила:**
- НЕ компилировать на сервере (OOM kill) — это касается и Go и Android
- НЕ деплоить новую версию на prod без прямого указания ферзя
- Все новые строки ОДНОВРЕМЕННО в values/strings.xml (en) + values-ru/strings.xml
- getString() правильно по контексту (Activity/Adapter/ViewModel)
- Коммитить и пушить после каждого значимого изменения
- НЕ деплоить на prod без тестирования на dev
- ProfileService v2 регистрировать только на dev (APP_ENV=dev)
- fetchServerInfo — всегда использовать для определения версии сервера
- Серверная ветка версий: 1.2.0.x, Android: 1.1.3.x до релиза
- Вся разработка на dev сервере, проверка обратной совместимости на prod
