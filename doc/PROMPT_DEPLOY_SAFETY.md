# Промт: Безопасный деплой Lavender Messenger

**Версия:** 1.0
**Создано:** 2026-07-26
**Серверы:** Dev (13.140.25.249:50052/8083), Prod (13.140.25.249:50051/8082)

---

## Краткий статус

| Сервер | Версия | Статус | systemd |
|--------|--------|--------|---------|
| Dev | 1.3.3.2 | ✅ Active | `lavender-server-dev` |
| Prod | 1.3.3.2 | ✅ Active | `lavender-server` |

---

## Деплой-процедура

### Через deploy-скрипты (рекомендуется)

```bash
# Dev
./scripts/deploy-dev-local.sh

# Prod (с backup)
./scripts/deploy-prod-local.sh
```

Скрипты делают: cross-compile → SCP → stop → replace → start → verify.

### Ручной деплой (если скрипт не работает)

```bash
# 1. Сборка
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/lavender-server .

# 2. Загрузка
scp /tmp/lavender-server lava:/tmp/lavender-server

# 3. Остановка
ssh lava "systemctl stop lavender-server"

# 4. Замена
ssh lava "cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server && chmod +x /root/LavenderMessenger/run/lavender-server"

# 5. Запуск
ssh lava "systemctl start lavender-server"

# 6. Проверка
curl -s http://13.140.25.249:8082/health
```

---

## Перед деплоем — ЧЕКЛИСТ

| # | Проверка | Команда |
|---|----------|---------|
| 1 | Компиляция | `go build -o lavender-server .` |
| 2 | Тесты | `go test ./... -count=1 -timeout 120s` |
| 3 | Disk space | `ssh lava "df -h /"` — минимум 1G свободно |
| 4 | PostgreSQL | `ssh lava "pg_isready"` — accepting connections |
| 5 | Redis | `ssh lava "redis-cli ping"` — PONG |
| 6 | Dev работает | `curl http://13.140.25.249:8083/health` |
| 7 | Prod работает | `curl http://13.140.25.249:8082/health` |

---

## После деплоя — ЧЕКЛИСТ

| # | Проверка | Команда |
|---|----------|---------|
| 1 | Health | `curl http://13.140.25.249:8082/health` |
| 2 | Версия | `curl http://13.140.25.249:8082/info` |
| 3 | OIDC | `curl http://13.140.25.249:8082/.well-known/openid-configuration` |
| 4 | Логи | `ssh lava "journalctl -u lavender-server --no-pager -n 20"` |
| 5 | Auth | Войти в приложение, проверить чаты |
| 6 | gRPC | `grpcurl -plaintext 13.140.25.249:50051 list` |

---

## Известные проблемы

### Диск заполняется journal логами

**Симптом:** `dial tcp [::1]:5432: connection refused` — PostgreSQL падает из-за нехватки места.

**Причина:** journal логи растут быстро (особенно при частых ошибках).

**Решение:**
1. `journalctl --vacuum-size=200M` — очистить
2. Cron уже настроен: `0 */4 * * * journalctl --vacuum-size=200M`
3. Если cron не помогает — проверить `du -sh /var/log/journal`

**Порядок восстановления:**
```bash
ssh lava "journalctl --vacuum-size=200M && service postgresql start"
# Сервер переподключится к PG автоматически
```

### Порт 8083 недоступен извне

**Симптом:** `curl http://13.140.25.249:8083/health` — timeout.

**Причина:** Firewall блокирует порт 8083.

**Решение:** Dev сервер работает, просто недоступен снаружи. Проверять через `ssh lava "curl http://localhost:8083/health"`.

---

## SSH ключи

| Ключ | Путь | Пользователь |
|------|------|-------------|
| Lava | `~/.ssh/lava` | root |
| Ferz (prod old) | `~/.ssh/ferzz@x-cart.com` | ferz |
| VPS | `~/.ssh/vps_key` | root |

---

## Версии и файлы

| Файл | Версия | Описание |
|------|--------|----------|
| `server.go` | ServerVersion | Основная версия сервера |
| `doc/ANALYSIS_OIDC_SSO.md` | — | Анализ + OIDC дизайн |
| `doc/PROMPT_REVOKE_TOKEN_IP.md` | — | Задачи security fixes |
| `doc/PROMPT_DEPLOY_SAFETY.md` | — | Этот файл |
| `CHANGELOG.md` | — | История изменений |

---

## Контакты

- **Автор:** Pavel Davydov (ferz)
- **Сервер:** 13.140.25.249
- **Логи prod:** `http://13.140.25.249/server-logs`
- **Логи dev:** `http://13.140.25.249/server-logs-dev`
