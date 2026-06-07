# Deploy Guide — Log Monitor

Полная инструкция по развёртыванию log-monitor на новом сервере.

## Предварительные требования

- Linux с systemd
- Go 1.24+ (`/usr/local/go1.24/bin/go`)
- nginx (для внешнего доступа)
- Сервис Lavender Messenger зарегистрирован в systemd

## Быстрый старт

### 1. Сборка

```bash
cd /root/msg/log-monitor

# Prod
go build -o /root/LavenderMessenger/run/log-monitor log-monitor.go

# Dev
go build -o /root/LavenderMessenger/run/log-monitor-dev log-monitor-dev.go
```

### 2. Systemd unit — prod

Создайте `/etc/systemd/system/log-monitor.service`:

```ini
[Unit]
Description=Lavender Messenger Log Monitor — Prod
After=network.target lavender-server.service
Wants=lavender-server.service

[Service]
Type=simple
WorkingDirectory=/root/LavenderMessenger/run
ExecStart=/root/LavenderMessenger/run/log-monitor
Restart=always
RestartSec=5
Environment=PATH=/usr/local/go1.24/bin:/usr/local/bin:/usr/bin:/bin
Environment=LOG_PORT=8090
Environment=LOG_SERVICE=lavender-server

[Install]
WantedBy=multi-user.target
```

### 3. Systemd unit — dev

Создайте `/etc/systemd/system/log-monitor-dev.service`:

```ini
[Unit]
Description=Lavender Messenger Log Monitor — DEV
After=network.target lavender-server-dev.service
Wants=lavender-server-dev.service

[Service]
Type=simple
WorkingDirectory=/root/LavenderMessenger/run
ExecStart=/root/LavenderMessenger/run/log-monitor-dev
Restart=always
RestartSec=5
Environment=PATH=/usr/local/go1.24/bin:/usr/local/bin:/usr/bin:/bin
Environment=LOG_PORT=8091
Environment=LOG_SERVICE=lavender-server-dev
Environment=LOG_TITLE=Lava Dev Server Logs

[Install]
WantedBy=multi-user.target
```

### 4. Активация

```bash
systemctl daemon-reload
systemctl enable log-monitor log-monitor-dev
systemctl start log-monitor log-monitor-dev
```

### 5. Nginx

Добавьте в конфиг nginx (обычно `/etc/nginx/sites-enabled/lavender`):

```nginx
# Prod server logs
location /server-logs {
    proxy_pass http://127.0.0.1:8090;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

# Dev server logs
location /server-logs-dev {
    proxy_pass http://127.0.0.1:8091;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

```bash
nginx -t && systemctl reload nginx
```

### 6. Проверка

```bash
# Статус сервисов
systemctl status log-monitor log-monitor-dev

# Логи работают?
curl -s http://127.0.0.1:8090/server-logs/health
curl -s http://127.0.0.1:8091/server-logs-dev/health

# Веб-UI
# Откройте http://<server-ip>/server-logs
# Откройте http://<server-ip>/server-logs-dev
```

## Обновление

```bash
cd /root/msg/log-monitor
go build -o /root/LavenderMessenger/run/log-monitor log-monitor.go
systemctl restart log-monitor
```

## Troubleshooting

### Логи не отображаются

1. Проверьте что сервис работает: `systemctl status log-monitor`
2. Проверьте порт: `ss -tlnp | grep 8090`
3. Проверьте journalctl: `journalctl -u lavender-server -n 10`
4. Проверьте nginx error log: `tail -f /var/log/nginx/error.log`

### 404 на /server-logs/raw

Убедитесь что nginx проксирует на правильный порт. Путь `/server-logs/raw` должен идти на `127.0.0.1:8090`, а `/server-logs-dev/raw` — на `127.0.0.1:8091`.

### Пустая страница в браере

1. Hard refresh: `Ctrl+Shift+R`
2. Проверьте консоль браузера на ошибки
3. Убедитесь что JavaScript не заблокирован

### Сервис падает

```bash
journalctl -u log-monitor -n 50
```

Проверьте что:
- Go установлен и доступен по PATH
- Порт не занят другим процессом
- Сервис lavender-server зарегистрирован в systemd
