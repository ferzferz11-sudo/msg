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

### Favorites flickering
- Исправлено в c873fbc: `sendSync()` передавал list без favoritesItem, вызывая remove/insert каждые 5с
- Паттерн: статический first item в RecyclerView должен быть ВКЛЮЧЁН во все background updates

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

### Prod vs Dev
- Dev: порт 50052, DB `chat_db_dev`, config `.env.dev`
- Prod: порт 50051, DB `chat_db`, config `.env`
- Версия сервера в `server.go:33`

---

## ThemeApplier

### FAB кнопки
- Новые FAB кнопки добавлять в список в `ThemeApplier.kt`:
  `listOf(R.id.aiFab, R.id.addChatFab, R.id.addContactFab, R.id.addThemeFab)`
- ThemeApplier устанавливает `backgroundTintList=customPrimary` и `imageTintList=customOnPrimary`
- Без этого FAB остаётся default `colorSecondaryContainer`
