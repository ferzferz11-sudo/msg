# Lavender Messenger — Задачи

**Версия:** v1.1.1.14
**Обновлено:** 2026-06-09
**Статус:** ✅ v1.1.1.14 — Дизайн + полировка UI

---

## ✅ v1.1.1.14 — Дизайн + полировка
- Анимации сообщений (fade-in + slide), typing indicator (ValueAnimator)
- Bottom sheets: MaterialCardView, hover-эффекты, per-command иконки
- Splash screen анимация, statusBarColor = bgColor
- compileDebugKotlin ✅, dev deployed ✅

---

## 📋 Актуальный бэклог

### Высокий приоритет (v1.1.2.0 — деплой на prod)
1. **Деплой на prod → v1.1.2.0** — после подтверждения стабильности

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## 🟡 Известные баги

### Favorites — мергание при обновлении списка чатов
- **Статус:** не исправлено
- **Причина:** DiffUtil пересоздаёт Favorites при каждом обновлении
- **Решение:** Исключить Favorites из DiffUtil, хранить отдельно от `displayedChats`

---

## Известные проблемы (не критично)
- Server migration warnings: `role "lavender" does not exist` — сервер работает
