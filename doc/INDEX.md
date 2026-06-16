# Лава — Серверная Документация

Индекс всех документов проекта.

**Версия:** v1.2.0.1
**Обновлено:** 2026-06-16 (сессия 24)
**Ветка:** feat/1.2.0.x
**Тег:** v1.2.0.1

---

## Быстрый старт

1. **PROMPT_SERVER.md** — промпт для серверных сессий
2. **TASKS.md** — таск-трекер
3. **CHANGELOG.md** — история версий сервера
4. **INTEGRATION_SESSION.md** — интеграционная сессия

---

## Файлы документации

### Текущая работа

| Файл | Назначение | Когда читать |
|------|-----------|-------------|
| `PROMPT_SERVER.md` | Промпт для серверных сессий | **Всегда в начале** |
| `TASKS.md` | Таск-трекер | В начале сессии |
| `INTEGRATION_SESSION.md` | Интеграционная сессия | В начале сессии |

### Архитектура и дизайн

| Файл | Назначение | Когда читать |
|------|-----------|-------------|
| `AI_SERVICES.md` | AI-сервисы: архитектура, API | При работе с AI |
| `PITFALLS.md` | Подводные камни | **Перед началом работы** |
| `AUTHSERVICE_V2.md` | AuthService v2: JWT | При работе с авторизацией |
| `HERMES_ORCHESTRATOR_DOC.md` | Hermes Orchestrator | При работе с Hermes |
| `PROJECT_MEMORY.md` | Проектная память | Для общего контекста |

### Android клиент

| Файл | Назначение |
|------|-----------|
| `/root/msg.client.android/doc/INDEX.md` | Индекс документации Android |
| `/root/msg.client.android/doc/PROMPT_ANDROID.md` | Промпт для Android-сессий |
| `/root/msg.client.android/doc/TASKS.md` | Таск-трекер Android |
| `/root/msg.client.android/doc/PATTERNS.md` | Паттерны разработки Android |
| `/root/msg.client.android/doc/SESSION_NOTES.md` | Заметки сессий Android |
| `/root/msg.client.android/doc/CHANGELOG.md` | История изменений Android |
| `/root/msg.client.android/doc/ARCH_ANALYSIS_V2_V1.md` | Анализ архитектуры v2 vs v1 |
| `/root/msg.client.android/doc/PLAN_REFACTOR_GRPC.md` | План рефакторинга RealGrpcClient |

---

## Правила

- При старте новой сессии: PROMPT_SERVER.md → TASKS.md → PITFALLS.md
- После каждого значимого изменения: обновлять TASKS.md + CHANGELOG.md
- При каждом релизе: обновлять CHANGELOG.md, TASKS.md, PITFALLS.md
- Версия сервера в server.go:34
- changelog.txt БОЛЬШЕ НЕ ИСПОЛЬЗУЕТСЯ
- Все серверы (включая dev) доступны всем пользователям
- Использовать только userId (UUID), НЕ username в RPC
- ⚠️ Gradle wrapper удалён с сервера — НЕ компилировать Android на сервере
