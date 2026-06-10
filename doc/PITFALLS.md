# Lavender Messenger — Pitfalls

Подводные камни и известные проблемы. Читать перед началом работы!

**Обновлено:** 2026-06-09

---

## Android

### SplashActivity
- `UserSession`, не `Session` (`data/session/UserSession.kt`)

### StandardBottomSheet
- `root` — `MaterialCardView`, не `LinearLayout`

### Bottom sheet items
- `bg_action_item_hover.xml` заменяет `selectableItemBackground`

### AccelerateDecelerateInterpolator
- `android.view.animation`, НЕ `androidx`

### ValueAnimator
- Всегда вызывайте `cancel()` перед запуском нового (TypingHolder leak)

### Favorites — отображение при пустом списке чатов (известная проблема)

**Статус:** не исправлено, v1.1.2.7

**Симптом:** при входе после очистки памяти (или первого входе) Favorites не отображается если у пользователя нет созданных чатов. Появляется только после создания первого чата.

**Причина:** `chatAdapter.setChats()` вызывается с списком `[Favorites]`, но `displayedChats` остаётся пустым (Favorites извлекается в отдельный `favoritesItem`). `notifyDataSetChanged()` внутри `setChats` может не дойти до RecyclerView из-за тайминга.

**Попытки исправления:**
- `selectedPositions.clear()` в `setChats` — не помогло
- `post { notifyDataSetChanged() }` — не помогло
- Убран `loadChatsFromCache` при пустом ответе сервера — не помогло

**Нужно:** разобраться почему `getItemCount()` возвращает 1 но RecyclerView не отображает элемент. Возможные причины:
- `notifyDataSetChanged()` вызывается до `setAdapter()`
- SwipeRefreshLayout перехватывает обновление
- `displayedChats` пуст и DiffUtil не считает изменение значимым

### Favorites flickering
- Исправлено в c873fbc: `sendSync()` передавал list без favoritesItem, вызывая remove/insert каждые 5с
- Паттерн: статический first item в RecyclerView должен быть ВКЛЮЧЁН во все background updates

### ChangelogActivity — bundled changelog
- **changelog.txt УДАЛЁН** из проекта и из деплоя на сервер (v1.1.2.6)
- Вместо него: `app/src/main/assets/changelog_bundled.txt` — встроен в APK, показывается мгновенно
- При каждом релизе: обновлять `assets/changelog_bundled.txt` вместе с `CHANGELOG.md`
- Формат: emoji-заголовки, буллеты `—`, секции по версиям
- Если bundled не обновлён — пользователь увидит устаревший ченджлог из APK
- **Цвета в fallback**: устанавливаются программно из `ThemeStore` (не через XML-атрибуты)

### ChangelogAdapter — цвета на кастомных темах (известная проблема)
- `ChangelogAdapter` использует `resolveColorAttr(context, android.R.attr.textColorPrimary)` для цветов текста
- На кастомных тёмных темах этот атрибут может быть тёмным → текст нечитаем
- **Не исправлено** — приоритет низкий, GitHub API работает и основной контент нормальный
- Для fallback проблема решена (цвета из `ThemeStore`)

### HermesGrpc proto mapping
- `CreateHermesSessionResponse`: field 1=success(bool), field 2=session_id(string) — НЕ наоборот!
- `CreateAgentResponse`: field 1=success(bool), field 2=agent_id(string)
- `AgentInfo`: field 4=is_preset(bool), 5=system_prompt(string), 6=model(string)

---

## Server

### hermes_sessions owner
- Таблица должна принадлежать `lavender`, не `postgres`
- Исправление: `cd /tmp && sudo -u postgres psql -d chat_db -c "ALTER TABLE hermes_sessions OWNER TO lavender;"`

### getOrCreateSession создаёт дубли
- Старое поведение: создавал сессию с `id = "hermes-" + userID` каждый раз
- Исправлено: ищет существующую сессию по `user_id`

### JSON в SQL
- Никогда не собирайте JSON через конкатенацию: `"["+username+"]"` → невалидный JSON
- Всегда `json.Marshal`

### DeleteChat и AI чаты
- DeleteChat удаляет из chats, но НЕ из hermes_sessions (orphaned sessions копятся)
- Исправлено в v1.1.2.2: каскадное удаление из hermes_sessions + hermes_messages
- Для OWL: FK CASCADE на owl_messages срабатывает, но owl_chat_settings — нет, нужно явное удаление

### Prod vs Dev
- Dev: порт 50052, DB `chat_db_dev`, config `.env.dev`
- Prod: порт 50051, DB `chat_db`, config `.env`
- Версия сервера в `server.go:33`

### Hermes история — всегда из БД
- `GetOrchestratorHistory` должен загружать из `hermes_messages` через `HermesDB.GetOrchestratorHistory()`
- НЕ использовать `session.Messages` (in-memory) — пропадает после рестарта сервера
- `getOrCreateSession` создаёт пустую сессию без загрузки истории из БД

### Rate limiter — refund on failure
- `allow()` добавляет timestamp ДО выполнения запроса
- При ошибке (OpenRouter, orchestrator) timestamp остаётся — слот потерян
- **Правило:** всегда вызывать `cancel(userID)` в failure path после успешного `allow()`
- `remaining()` возвращает `limit - len(valid)` — корректно отражает оставшиеся запросы

### /dev/null сломан после OOM
- Если `/dev/null` стал файлом вместо device node: `rm /dev/null && mknod /dev/null c 1 3 && chmod 666 /dev/null`
- Без этого `go build` падает с "open /dev/null: no such file or directory"

---

## JWT Agent Auth

### Секретный ключ
- `JWT_SECRET` — минимум 32 байта, хранится в `.env` / `.env.dev`
- Никогда не коммитить в git!
- При компрометации — немедленно перегенерировать все токены

### Валидация токена
- `validateToken()` проверяет: HMAC подпись, expiration, agent_id match, revoked в БД
- Пустой токен = отклонение (нет backward compat с неавторизованными агентами)
- Для тестирования без токена — нужно явно создать токен через `GenerateAgentToken`

### Хранение
- В БД хранится только SHA-256 хеш токена, не сам токен
- Токен показывается клиенту только один раз при генерации
- `RevokeAgentToken` — помечает `revoked = TRUE`, существующие подключения продолжают работать до реконнекта

### Admin-only
- `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — требуют `IsSuperAdmin()`
- `admin_user_id` в запросе должен совпадать с супер-админом в БД

---

## ThemeApplier

### FAB кнопки
- Новые FAB кнопки добавлять в список в `ThemeApplier.kt`:
  `listOf(R.id.aiFab, R.id.addChatFab, R.id.addContactFab, R.id.addThemeFab)`
- ThemeApplier устанавливает `backgroundTintList=customPrimary` и `imageTintList=customOnPrimary`
- Без этого FAB остаётся default `colorSecondaryContainer`
