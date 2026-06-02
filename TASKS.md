# Lavender Messenger — Известные проблемы и задачи в работе

**Последнее обновление:** 2026-06-02
**Ветка:** feat/1.1.0.x
**Версия:** 1.1.0.9

---

## 🔧 В процессе

### 1. OWL чат: поле ввода перекрывает кнопки навигации
- **Статус:** частично исправлено, требует проверки на устройстве
- **Описание:** При открытии OWL чата поле ввода (bottomPanel) расположено поверх навигационных кнопок телефона. Клавиатура при открытии не сдвигает layout.
- **Что сделано:**
  - Добавлен `android:fitsSystemWindows="true"` → не помогло
  - Добавлен `adjustResize` в манифест → не помогло
  - Добавлен `adjustNothing` + ручная обработка insets → в процессе
- **Текущий подход:** `adjustResize` + `translationY` на bottomPanel при открытии клавиатки
- **Проблема:** `enableEdgeToEdge()` в `ThemeApplier.kt` конфликтует с `adjustResize`
- **Файлы:** `activity_owl.xml`, `OwlActivity.kt`, `AndroidManifest.xml`

### 2. OWL чат: keepalive failed при длительном простое
- **Статус:** наблюдается, не критично
- **Описание:** `UNAVAILABLE: Keepalive failed. The connection is gone` — gRPC канал теряется при длительном простое на мобильной сети
- **Сервер:** keepalive настроен (15s ping, 10s timeout)
- **Клиент:** автоматический reconnect в `onClose` RealGrpcClient
- **Файлы:** `RealGrpcClient.kt`

---

## ✅ Исправлено

### 1. Favorites: сообщения не отображались (empty encrypted data)
- **Статус:** ✅ исправлено (e0d5dd5)
- **Симптом:** При открытии favorites — 9 ошибок "empty encrypted data", сообщения не отображались
- **Причина:** Дубликат `COALESCE(m.is_e2ee, false)` в SQL запросе `GetMessages` для favorites (db.go:187) → 17 полей в SELECT вместо 16 → смещение в `Scan()` → `m.Encrypted` получал пустое значение
- **Исправление:** Убран дубликат, сервер пересобран и перезапущено
- **Проверка:** После перезапуска — ошибок нет, сообщения отображаются

### 2. E2EE: is_e2ee column + обработка
- **Статус:** ✅ исправлено (613f305)
- Добавлена колонка `is_e2ee` в messages
- `E2EePayload = base64(m.Encrypted)` вместо `string(m.Encrypted)`
- Combined detection: `m.IsE2EE || isSecretChat`
- GrpcClient: GlobalScope → scope

### 3. OWL чаты в списке после возврата
- Исправлен `CreateOwlChat` — резолвит username из DB
- Убран `creator == userId` check в `GetOwlHistory`
- Восстановлена колонка `last_message_text` в `chats`

### 4. Поле ввода в OWL чате
- Исправлен `textInputType` — убран невалидный `textUri`

### 5. Дублирующиеся сообщения при стриминге OWL
- Заменён `addMessage` на `updateLastAssistantMessage` для стриминга
- Убран дубль `finished=true` из `onClose`

### 6. Темы в OWL чате
- Добавлен `ThemeUi.bind(this, userId)` в `onCreate`

### 7. Удаление OWL чатов
- **Было:** `Failed to parse participants: invalid character 'e' in literal false`
- **Причина:** старый формат participants `[ferz]` вместо `["ferz"]`
- **Исправлено:** обновлены данные в БД

### 8. /key команда в OWL
- Добавлена в `showWelcomeMessage()` и в `/help`

### 9. Ветки переименованы
- `feat/1.2.0.owl` → `feat/1.1.0.x`
- Версия: 1.1.0.9

---

## 📋 Бэклог

### Высокий приоритет
- [ ] Секретный чат — заглушка "not implemented in this build"

### Средний приоритет
- [ ] Mac session logout issue — не исследовано
- [ ] Кэширование OWL чатов в локальной БД

### Низкий приоритет
- [ ] Оптимизация списка моделей OWL (23 модели — можно кэшировать)
- [ ] Graceful reconnect при keepalive failed без потери сообщений

---

## 🔑 Ключевые решения

| Решение | Обоснование |
|---------|-------------|
| OWL чаты хранятся в `chats` с `type='owl'` | Единая таблица, не нужна отдельная |
| Participants формат: `["username"]` JSON array | Совместимость с существующим парсером |
| `ThemeUi.bind()` для тем | Единообразие с остальным приложением |
| `adjustResize` + `translationY` | Edge-to-edge конфликт с adjustResize |
| `CoroutineScope` вместо `lifecycleScope` для `loadHistory()` | Предотвращает отмену корутины при смене activity |
| Favorites: сообщения хранятся в messages с `room_id='favorites_*'` | Единая таблица сообщений, favorites — виртуальная комната |

---

## 📁 Ключевые файлы

| Файл | Назначение |
|------|------------|
| `/root/msg/server.go` | Сервер, версия 1.1.0.9 |
| `/root/msg/db.go` | БД запросы, миграции, favorites |
| `/root/msg/crypto.go` | AES-256-GCM, E2EE |
| `/root/msg/hub.go` | Менеджер соединений |
| `/root/msg/http_server.go` | HTTP сервер (порт 8082) |
| `/root/msg/owl.go` | OWL AI ассистент |
| `/root/msg/messenger.proto` | gRPC определения |
| `FavoritesActivity.kt` | Экран избранного |
| `OwlActivity.kt` | OWL чат UI |
| `RealGrpcClient.kt` | gRPC канал и reconnect |
| `ChatListActivity.kt` | Список чатов |
| `ThemeApplier.kt` | Применение тем |
