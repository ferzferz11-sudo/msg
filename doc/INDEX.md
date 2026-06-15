# Лава — Документация

Индекс всех документов проекта. Читать при каждом старте новой сессии.

**Версия:** v1.2.1.0
**Обновлено:** 2026-06-14

---

## Быстрый старт

1. **INTEGRATION_SESSION.md** — текущий контекст интеграции (версии, архитектура, что сделано, что нет)
2. **TASKS.md** — таск-трекер (сделано/не сделано по приоритетам)
3. **CHANGELOG.md** (в корне) — история версий сервера
4. **PROMPT.md** — промпт для любых сессий (общий)
5. **PROMPT_SERVER.md** — промпт для серверных сессий

---

## Файлы документации

### Текущая работа

| Файл | Назначение | Когда читать |
|------|-----------|-------------|
| `INTEGRATION_SESSION.md` | Интеграционная сессия: версии, архитектура, правила, промпт для следующей сессии | **Всегда в начале** |
| `TASKS.md` | Таск-трекер: сделано по версиям, бэклог по приоритетам | В начале сессии |

### Архитектура и дизайн

| Файл | Назначение | Когда читать |
|------|-----------|-------------|
| `AI_SERVICES.md` | AI-сервисы: архитектура, API, потоки данных, proto mapping | **При работе с AI чатами** |
| `PITFALLS.md` | Подводные камни и известные проблемы | **Перед началом работы** |
| `AUTHSERVICE_V2.md` | AuthService v2: JWT, device management, миграция | При работе с авторизацией |
| `HERMES_ORCHESTRATOR_DOC.md` | Документация Hermes Orchestrator | При работе с Hermes |
| `PROJECT_MEMORY.md` | Проектная память: ключевые решения | Для общего контекста |
| `PROMPT.md` | Промпт для любых сессий (общий) | **При старте новой сессии** |
| `PROMPT_SERVER.md` | Промпт для серверных сессий | **При старте новой серверной сессии** |

### DevOps и инфраструктура

| Файл | Назначение | Когда читать |
|------|-----------|-------------|
| `LOG_MONITOR.md` | Log Monitor: сборка, деплой, API, web UI | **При проблемах с логами** |
| `TESTING.md` | Модульные тесты: запуск, покрытие | **При работе с тестами** |

### Android клиент

| Файл | Назначение |
|------|-----------|
| `/root/msg.client.android/doc/INDEX.md` | Индекс документации Android |
| `/root/msg.client.android/doc/PROMPT_ANDROID.md` | Промпт для Android-сессий |
| `/root/msg.client.android/doc/TASKS.md` | Таск-трекер Android |
| `/root/msg.client.android/doc/PATTERNS.md` | Паттерны разработки Android |
| `/root/msg.client.android/doc/REMOTE_AGENT.md` | Документация Remote Agent |
| `/root/msg.client.android/doc/SESSION_NOTES.md` | Заметки сессий Android |
| `/root/msg.client.android/CHANGELOG.md` | История изменений Android |

---

## Правила

- При старте новой сессии: читать цепочку INDEX.md → INTEGRATION_SESSION.md → TASKS.md → PITFALLS.md
- При работе над тестами: читать doc/TESTING.md
- При работе над Android: читать /root/msg.client.android/doc/INDEX.md → PROMPT_ANDROID.md → TASKS.md
- После каждого значимого изменения: обновлять INTEGRATION_SESSION.md + TASKS.md + соответствующие документы
- При каждом релизе: обновлять CHANGELOG.md (сервер + Android), INTEGRATION_SESSION.md, TASKS.md, PITFALLS.md
- Промпт для следующей сессии всегда внизу INTEGRATION_SESSION.md
- Версия сервера в server.go:33, версия Android в version.txt
- changelog.txt БОЛЬШЕ НЕ ИСПОЛЬЗУЕТСЯ
