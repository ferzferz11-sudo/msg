# Лава — Задачи

**Версия:** v1.1.3.2
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-12

---

## ✅ v1.1.3.2 — Remote Agent Token Management

### Android
- **Генерация JWT токенов** — работает через `hermes_agent.HermesAgentService/GenerateAgentToken`
- **Список токенов** — отображается сразу после генерации (локальный кэш)
- **Копирование токена/команды** — кнопки в каждом элементе списка
- **Отзыв токена** — кнопка "Отозвать" с подтверждением
- **Запуск/остановка агента** — StartAgent/StopAgent RPC
- **UI статуса** — зелёный индикатор при запущенном агенте
- **Персистентность** — выбранный агент сохраняется в SharedPreferences
- **Исправлено**: диалог токена не закрывается при копировании
- **Исправлено**: ошибки сервера переведены на русский

### Сервер
- Prod сервер обновлён до v1.1.3.2

---

## 🔄 v1.1.3.x — Текущая ветка

### Известные проблемы (P1)
- **Агент завершается сразу после запуска** — `hermes_remote_agent.py` падает в `connect()` при отправке `AgentMessage`. Root cause: protobuf marshaling в Python. Нужно исправить скрипт.
- **Токены не фильтруются по пользователю** — `ListAgentTokens` возвращает все токены из БД. Нужно добавить фильтр по `created_by`.

---

## 📋 Бэклог

### Средний приоритет
- [ ] Модульные тесты для OWL streaming
- [ ] Модульные тесты для AuthService (SignIn/SignUp)
- [ ] Рефакторинг server.go → пакеты (server/, auth/, chat/, ai/)

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG) — ночная задача
- [ ] Structured logging на сервере (zap/logrus)
- [ ] Prometheus метрики (request count, latency, active connections)

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    —  Entry point, gRPC server, graceful shutdown
hermes_agent_service.go    —  HermesAgentServiceServer (Connect, tokens, agent process mgmt)
hermes_remote.proto        —  Proto для HermesAgentService
db_hermes.go              —  HermesDB (CRUD для agent_tokens, hermes_sessions, etc.)
server_ai.go              —  AI Chat + RemoteAgent RPC (ListRemoteAgents, DeployAgentTask, etc.)
http_server.go            —  HTTP server (/health, /upload-*, /avatars/, etc.)
hermes_orchestrator.go    —  Orchestrator + RemoteAgentManager
hermes_remote_manager.go  —  RemoteAgent, RemoteTask, task execution
gen/hermes_agent/          —  Сгенерированный Go код из hermes_remote.proto
scripts/deploy_agent.sh    —  Управление агентом через systemd
scripts/hermes-agent@.service —  systemd unit шаблон
```

### Android (Kotlin)
```
data/grpc/HermesGrpc.kt                    —  gRPC клиент (все unary/streaming RPCs)
data/grpc/GrpcClient.kt                    —  Facade над HermesGrpc + RealGrpcClient
data/proto/MessengerProto.kt               —  Hand-written proto типы
ui/remote/RemoteAgentActivity.kt           —  Чат с remote agent, авто-рефреш 30с
ui/remote/RemoteAgentSettingsActivity.kt   —  Токены + запуск/остановка агента
ui/remote/RemoteAgentViewModel.kt          —  ViewModel для RemoteAgentActivity
ui/remote/TokenDialog.kt                   —  Диалог генерации токена
ui/hermes/AgentListActivity.kt             —  Список AI-агентов + вкладка Remote
ui/hermes/AgentSettingsActivity.kt         —  Настройки AI-агента
SplashActivity.kt                          —  Стартовый splash (версия отображается)
SplashLoadingActivity.kt                   —  Splash при логине (версия отображается)
AndroidManifest.xml                        —  NotificationActivity зарегистрирована
```
