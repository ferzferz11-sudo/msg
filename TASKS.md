# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.11
**Обновлено:** 2026-06-04
**Статус:** ✅ Android + Dev сервер работают, оркестратор отвечает

---

## ✅ Исправлено (v1.1.0.11)

- **Proto mismatch в CreateHermesSession** — перепутаны номера полей в response marshaller (field 1=success/bool, field 2=session_id/string). Вызывало `CANCELLED: Failed to read message`. Исправлено.
- **LogViewerActivity не в AndroidManifest** — ClassCastException при возврате. Добавлена в манифест.
- **ThemeUi.bind() в LogViewerActivity** — вызывал recreate()/ClassCastException при возврате. Убран.
- **SuperAdmin по username** — не работал на dev (другой профиль). Переведён на user_id (UUID) с fallback.
- **Finished: true в welcome message** — клиент закрывал  stream. Исправлено на false.
- **AppLog + LogViewerActivity** — система логирования ошибок с просмотром из админки.

---

## 🔧 В процессе

- Tool Calling Loop — адаптивный лимит итераций (убрать жёсткий max 3, детектить завершение по отсутствию tool calls)

---

## ⏳ Не начато

- RemoteAgentManager.SendTask() — только заглушка
- Auth токены для удалённых агентов (бэклог)
- Qdrant + CLIP для production RAG
- Agent settings bottom sheet (long click на агента в чате)

---

## 🧪 Готово к проверке

- ✅ gRPC ChatWithPipeline — стриминг ответов + tool calling (пройдено!)
- ✅ 8 агентов реестра — роутинг по agent_id (пройдено!)
- ✅ CreateHermesSession — создание сессии работает (v1.1.0.11)
- LogViewerActivity — просмотр логов из админки (после исправления ClassCastException)

---

## Следующий шаг

Доработать Tool Calling Loop: убрать жёсткий лимит 3 итерации, добавить авто-финализацию когда LLM перестаёт вызывать tools. Файл: `core/pipeline/pipeline.go`.
