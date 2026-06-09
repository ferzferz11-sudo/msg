# Lavender Messenger — Задачи

**Версия:** v1.1.1.15
**Обновлено:** 2026-06-09
**Статус:** ✅ v1.1.1.15 — Бесплатные модели + своя модель

---

## ✅ v1.1.1.15 — Бесплатные модели + своя модель

### Server
- **free_openrouter_models table** — model_id, display_name, is_active, sort_order
- **GetFreeModels RPC** — получение списка бесплатных моделей
- **SetFreeModel / RemoveFreeModel RPC** — админ-управление
- **GetOwlSettings** возвращает free_models в ответе
- **Proto:** FreeModelInfo, GetFreeModelsRequest/Response, SetFreeModelRequest/Response
- Dev deployed

### Android
- **version.txt** — bumped to 1.1.1.15
- **changelog.txt** — обновлён
- **Бесплатные модели с сервера** — GetFreeModels RPC вместо хардкода
- **Своя модель** — текстовый ввод ID модели (только с ключом)
- **Favorites flickering fix** — startSync() + updateAvatarCache() offset
- compileDebugKotlin ✅

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
