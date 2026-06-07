#!/bin/bash
# deploy-dev.sh — Пересобрать и перезапустить dev сервер
# Использование: ./deploy-dev.sh
# Билдит lavender-server-dev + log-monitor-dev, копирует в run/, рестартает systemd

set -e

REPO="/root/msg"
RUN="/root/LavenderMessenger/run"

echo "=== Building dev server ==="
cd "$REPO"
git pull
go build -o "$RUN/lavender-server-dev" .
echo "✓ lavender-server-dev built"

echo "=== Building dev log monitor ==="
cd "$REPO/log-monitor"
go build -o "$RUN/log-monitor-dev" log-monitor-dev.go
echo "✓ log-monitor-dev built"

echo "=== Restarting services ==="
systemctl restart lavender-server-dev
systemctl restart log-monitor-dev
sleep 2

echo "=== Status ==="
systemctl is-active lavender-server-dev && echo "✓ dev server: running" || echo "✗ dev server: FAILED"
systemctl is-active log-monitor-dev && echo "✓ dev log monitor: running" || echo "✗ dev log monitor: FAILED"

echo ""
echo "Dev server: grpc :50052, http :8083"
echo "Dev logs:   http://13.140.25.249/server-logs-dev"
