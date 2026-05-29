#!/bin/bash
# deploy_apk.sh — Upload APK, version.txt, changelog.txt to nginx on 13.140.25.249
# Run from server repo root: ./scripts/deploy_apk.sh [path/to/app-release.apk]
# If no arg, uses app/build/outputs/apk/release/app-release.apk from Android repo

set -euo pipefail

REMOTE_HOST="13.140.25.249"
REMOTE_USER="root"
WEB_DIR="/var/www/lavender"
SERVER_DIR="/root/LavenderMessenger/run"
LOCAL_DIR="$(cd "$(dirname "$0")/.. && pwd)"

# Determine APK path
APK_SRC="${1:-}"
if [ -z "$APK_SRC" ]; then
    # Try Android repo relative to server repo
    ANDROID_APK="$LOCAL_DIR/../msg.client.android/app/build/outputs/apk/release/app-release.apk"
    if [ -f "$ANDROID_APK" ]; then
        APK_SRC="$ANDROID_APK"
    else
        echo "❌ APK not found. Provide path: ./scripts/deploy_apk.sh /path/to/app-release.apk"
        exit 1
    fi
fi

if [ ! -f "$APK_SRC" ]; then
    echo "❌ APK not found: $APK_SRC"
    exit 1
fi

REMOTE="${REMOTE_USER}@${REMOTE_HOST}"

# Version
VERSION=$(cat "$LOCAL_DIR/version.txt" 2>/dev/null || echo "unknown")
echo "📱 APK: $APK_SRC"
echo "🏷️  Version: $VERSION"

# Ensure remote dirs
ssh -o StrictHostKeyChecking=no "$REMOTE" "mkdir -p $WEB_DIR /var/apk"

# Upload APK
echo "⬆ Uploading APK..."
rsync -avz --progress -e "ssh -o StrictHostKeyChecking=no" \
    "$APK_SRC" "$REMOTE:/var/apk/lavender.apk"

# Generate and upload version.txt + changelog.txt on remote
echo "⬆ Uploading version.txt and changelog.txt..."
ssh -o StrictHostKeyChecking=no "$REMOTE" "
    echo '$VERSION' > $WEB_DIR/version.txt
    cp $SERVER_DIR/changelog.txt $WEB_DIR/changelog.txt 2>/dev/null || echo 'No changelog.txt in server repo'
    chown -R www-data:www-data $WEB_DIR
"

# Verify
echo "🔍 Verifying..."
ssh -o StrictHostKeyChecking=no "$REMOTE" "
    echo ' version.txt: \$(cat $WEB_DIR/version.txt)'
    echo ' APK size:    \$(du -h /var/apk/lavender.apk | cut -f1)'
    echo ' HTTP check:'
    curl -s -o /dev/null -w '  /version.txt → %{http_code}\n' http://127.0.0.1/version.txt
    curl -s -o /dev/null -w '  /download   → %{http_code}\n' http://127.0.0.1/download
    curl -s -o /dev/null -w '  /           → %{http_code}\n' http://127.0.0.1/
"

echo "🚀 APK deploy complete!"
echo "🌐 http://13.140.25.249"
