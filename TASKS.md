# Lavender Messenger — Задачи

**Последнее обновление:** 2026-06-08
**Версия:** 1.1.0.16 (dev)
**Ветка:** feat/1.1.0.x

---

## 🔧 Текущие проблемы (Android)

### 1. Favorites не в списке чатов
- **Статус:** не исправлено
- **Причина:** `GetChats` не включает favorites
- **Хранится:** в `messages` с `room_id='favorites_*'` + таблица `favorites`
- **Нужно:** добавить в `GetChats` без создания записи в `chats`

---

## ✅ Исправлено в сессии 2026-06-08

- **Регистрация краш** — `startActivity+finish` → `recreate()` (гонка фокуса между двумя CLA)
- **Очистка кэша** — убрана при входе/логине, добавлена в `logout()` и `deleteProfile()`
- **Logout UI** — `FLAG_ACTIVITY_NEW_TASK|CLEAR_TASK` пересоздаёт активити, auth dialog на чистом экране
- **Dev БД** — очищена, остались только ferz11 и ferz99

## ✅ Исправлено в сессии 2026-06-07

- **Orchestrator SSE парсинг** — `streamOpenRouter` читает SSE (`data: {...}`) вместо голой JSON
- **OWL init log** — показывает `provider: OpenRouter, model: openrouter/owl-alpha`
- **Версия** — 1.1.0.13, теги приведены в порядок

---

## 📋 Бэклог

- [ ] Секретный чат — заглушка
- [ ] OWL keepalive failed при простое
- [ ] Mac session logout
