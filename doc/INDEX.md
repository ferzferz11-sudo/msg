# Лава — Документация

Индекс всех документов проекта. Читать при каждом старте новой сессии.

---

## Быстрый старт

1. **PROMPT.md** — текущий контекст сессии (ветка, статус, этапы)
2. **OPTIMIZATION_PLAN.md** — план оптимизации с прогрессом
3. **INTEGRATION_SESSION.md** — интеграционная сессия (версии, архитектура)
4. **TASKS.md** — таск-трекер

---

## Файлы документации

### Текущая работа

| Файл | Назначение |
|------|-----------|
| `PROMPT.md` | Промпт для серверных сессий: этапы, правила, команды |
| `OPTIMIZATION_PLAN.md` | План оптимизации: проблемы, фиксы, оценки |
| `INTEGRATION_SESSION.md` | Интеграционная сессия: версии, архитектура, статус |
| `TASKS.md` | Таск-трекер: сделано/не сделано |

### Архитектура и дизайн

| Файл | Назначение |
|------|-----------|
| `ARCHITECTURE.md` | Общая архитектура сервера |
| `AI_SERVICES.md` | AI-сервисы: OWL AI, Hermes Orchestrator |
| `PITFALLS.md` | Подводные камни и известные проблемы |
| `AUTHSERVICE_V2.md` | AuthService v2 (JWT) документация |
| `HERMES_ORCHESTRATOR_DOC.md` | Документация Hermes Orchestrator |

### DevOps и инфраструктура

| Файл | Назначение |
|------|-----------|
| `LOG_MONITOR.md` | Log Monitor: сборка, деплой, API |
| `TESTING.md` | Модульные тесты |
| `RELEASE.md` | Процесс релиза |

---

## Правила

- При старте сессии: читать `PROMPT.md` → `OPTIMIZATION_PLAN.md` → `INTEGRATION_SESSION.md`
- При работе с AI: читать `AI_SERVICES.md`
- При деплое: читать `RELEASE.md`
- После изменений: обновлять `INTEGRATION_SESSION.md` + `TASKS.md`
- Версия сервера в `server.go:33`, версия Android в `version.txt`
- CHANGELOG.md — сервер в `/root/msg/CHANGELOG.md`
