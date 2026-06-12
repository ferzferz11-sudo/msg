# Лава — Задачи

**Версия:** v1.1.3.1
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-14

---

## 🔄 v1.1.3.1 — Текущая ветка

### Сервер
- Debug логи в hermes_agent_service.go обёрнуты в `os.Getenv("DEBUG")`
- **P1 fix**: GenerateAgentToken возвращает ошибку клиенту при неудачном сохранении в БД
- **P1 fix**: hermesDB == nil check — возврат "database not available" вместо nil pointer
- **P2**: Убран дубликат token RPC из messenger.proto (ChatService) — оставлен только в hermes_agent.HermesAgentService
- **P2**: Rate limiting на GenerateAgentToken (5 секунд между запросами на пользователя)
- **P3**: Health check endpoint (`/health`) на HTTP сервере (порт 8082)
- **P3**: Graceful shutdown для gRPC сервера (SIGINT/SIGTERM → GracefulStop)
- **P3**: Agent Process Management RPC — StartAgent/StopAgent/GetAgentProcessStatus
  - Сервер запускает hermes_remote_agent.py как subprocess
  - Отслеживание PID, автоочистка при выходе
  - systemd шаблон: `scripts/hermes-agent@.service`
  - Скрипт управления: `scripts/deploy_agent.sh`

### Android
- Убран Toast "Вход выполнен"
- Авто-прокрутка вниз при отправке сообщения
- Версия на SplashActivity
- Версия на SplashLoadingActivity (экран логина)
- Debug логи обёрнуты в BuildConfig.DEBUG
- **Fix**: добавлена NotificationActivity в AndroidManifest (была не зарегистрирована → краш)
- **P2**: SplashActivity + SplashLoadingActivity — версия показывается на обоих экранах
- **P2**: RemoteAgentActivity — авто-рефреш статуса агента каждые 30 сек (repeatOnLifecycle)
- **P2**: RemoteAgentSettingsActivity — кнопка "Скопировать команду" в диалоге токена
- **P2**: RemoteAgentSettingsActivity — кнопки "Запустить/Остановить агента" (запуск на сервере через gRPC)
- **P2**: AgentListActivity — вкладка "Remote" для перехода к RemoteAgentActivity
- **P2**: RemoteAgentSettingsActivity — сохранение выбранного агента в SharedPreferences
- **P2**: Индикатор "агент не подключён" в RemoteAgentActivity (уже был)

---

## ✅ Известные проблемы

### Низкий приоритет
- Сообщения пользователя видны только после ответа агента (нужна отладка на устройстве)
- Server migration warnings: `role "lavender" does not exist` при миграциях (не критично)

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
