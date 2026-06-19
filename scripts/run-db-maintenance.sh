#!/bin/bash
# run-db-maintenance.sh — Upload and run db_maintenance.sh on remote server
# Usage: ./run-db-maintenance.sh [--dry-run] [--prod] [--skip-files] [--skip-vacuum]
# Requirements: SSH alias "lava" in ~/.ssh/config, sudo access on server

set -euo pipefail

SERVER="lava"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REMOTE_MAINT="/tmp/db_maintenance.sh"

# Forward all arguments
ARGS="$*"

echo "=== 1. Upload db_maintenance.sh ==="
scp "$SCRIPT_DIR/db_maintenance.sh" ${SERVER}:${REMOTE_MAINT}
ssh $SERVER "chmod +x ${REMOTE_MAINT}"
echo "✓ Uploaded"

echo ""
echo "=== 2. Run on server ==="
ssh $SERVER "sudo -u postgres ${REMOTE_MAINT} ${ARGS}"

echo ""
echo "=== 3. Cleanup ==="
ssh $SERVER "rm -f ${REMOTE_MAINT}"
echo "✓ Done"
