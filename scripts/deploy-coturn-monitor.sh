#!/bin/bash
# deploy-coturn-monitor.sh — загружает watch-services.sh на prod и настраивает cron
# Использование: ./scripts/deploy-coturn-monitor.sh

set -e

SERVER="lava"
REMOTE_DIR="/root/LavenderMessenger/run"
SCRIPT="watch-services.sh"

echo "=== 1. Upload $SCRIPT ==="
ssh $SERVER "mkdir -p ${REMOTE_DIR}/scripts"
scp "scripts/$SCRIPT" ${SERVER}:${REMOTE_DIR}/scripts/$SCRIPT
ssh $SERVER "chmod +x ${REMOTE_DIR}/scripts/$SCRIPT"
echo "✓ Uploaded"

echo "=== 2. Setup cron (every 15 min) ==="
CRON_LINE="*/15 * * * * ${REMOTE_DIR}/scripts/$SCRIPT >> /var/log/watch-services.log 2>&1"
ssh $SERVER "
    (crontab -l 2>/dev/null | grep -v 'watch-services.sh'; echo '$CRON_LINE') | crontab -
"
echo "✓ Cron configured"

echo "=== 3. Verify ==="
ssh $SERVER "crontab -l | grep watch-services"
echo ""
echo "✓ Done. Monitor runs every 15 minutes."
