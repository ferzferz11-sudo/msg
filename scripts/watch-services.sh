#!/bin/bash
# watch-services.sh — проверяет и перезапускает сервисы lavender + log-monitor
# Запускается каждые 15 минут через cron

SERVICES=(
    "lavender-server"
    "lavender-server-dev"
    "log-monitor"
    "log-monitor-dev"
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
