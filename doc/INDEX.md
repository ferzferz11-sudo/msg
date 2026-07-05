# Лава — Документация

Индекс документов проекта. Читать при каждом старте новой сессии.

**Модуль:** `LavenderMessenger` | **Go:** 1.26 | **Сервер:** v1.3.3.1

---

## Быстрый старт

1. **PROMPT.md** — контекст сессии (ветка, статус, команды, правила)
2. **CLIENT_INTEGRATION.md** — **полная интеграция клиентов** (все gRPC методы, HTTP endpoints, AI v2, marketplace, company system)

---

## Файлы документации

| Файл | Назначение |
|------|-----------|
| `PROMPT.md` | Промпт для серверных сессий: статус, правила, команды |
| `CLIENT_INTEGRATION.md` | **Единый документ интеграции клиентов** — все gRPC методы, HTTP endpoints, auth, AI v2, marketplace, company system |
| `ARCHITECTURE.md` | Архитектура сервера: структура файлов, auth, AI pipeline |
| `AI_SERVICES.md` | AI-сервисы: провайдеры, пресеты, инструменты |
| `TASKS.md` | Таск-трекер |
| `PITFALLS.md` | Подводные камни и известные проблемы |
| `TESTING.md` | Модульные тесты |
| `RELEASE.md` | Процесс релиза |
| `LOG_MONITOR.md` | Log Monitor: сборка, деплой, API |

---

## Правила

- При старте сессии: читать `PROMPT.md`
- При интеграции нового клиента: `CLIENT_INTEGRATION.md`
- При работе с AI: `AI_SERVICES.md`
- При деплое: `RELEASE.md`
- Версия сервера в `server.go`
- **Актуальный код сервера всегда доступен локально** — перед работой всегда читай файлы из `/Users/paveld/LavenderMessenger-server/`
- Android: `/root/msg.client.android/doc/` — документация клиента
- ⚠️ Android собирается ТОЛЬКО локально (нет памяти на сервере)
- ⚠️ v1 совместимость удалена: только ChatV2 stream, ProfileService v2, Messages v2
