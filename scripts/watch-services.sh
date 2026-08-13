#!/bin/bash
# watch-services.sh — проверяет и перезапускает сервисы lavender + log-monitor + coturn
# Запускается каждые 15 минут через cron

SERVICES=(
    "lavender-server"
    "lavender-server-dev"
    "log-monitor"
    "log-monitor-dev"
    "coturn"
)

LOG="/var/log/watch-services.log"
DATE=$(date '+%Y-%m-%d %H:%M:%S')

for svc in "${SERVICES[@]}"; do
    if ! systemctl is-active --quiet "$svc"; then
        echo "[$DATE] $svc is DOWN — restarting..." >> "$LOG"
        systemctl restart "$svc"
        sleep 2
        if systemctl is-active --quiet "$svc"; then
            echo "[$DATE] $svc restarted OK" >> "$LOG"
        else
            echo "[$DATE] $svc FAILED to restart!" >> "$LOG"
        fi
    fi
done

# Port check for coturn (service may be active but port unresponsive)
if ! nc -zvu 127.0.0.1 3478 -w 3 2>/dev/null; then
    echo "[$DATE] coturn port 3478 unreachable — restarting..." >> "$LOG"
    systemctl restart coturn
    sleep 2
    if nc -zvu 127.0.0.1 3478 -w 3 2>/dev/null; then
        echo "[$DATE] coturn port 3478 restored OK" >> "$LOG"
    else
        echo "[$DATE] coturn port 3478 STILL unreachable after restart!" >> "$LOG"
    fi
fi
