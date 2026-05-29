#!/bin/bash
# deploy_apk.sh — Copy APK + version/changelog to nginx
# Usage: ./scripts/deploy_apk.sh [path/to/app-release.apk]
set -euo pipefail

WEB_DIR="/var/www/lavender"
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

APK_SRC="${1:-}"
if [ -z "$APK_SRC" ]; then
    ANDROID_APK="$SCRIPT_DIR/../msg.client.android/app/build/outputs/apk/release/app-release.apk"
    [ -f "$ANDROID_APK" ] && APK_SRC="$ANDROID_APK"
fi

[ -z "$APK_SRC" ] && { echo "❌ APK not found. Usage: $0 /path/to/app-release.apk"; exit 1; }
[ ! -f "$APK_SRC" ] && { echo "❌ APK not found: $APK_SRC"; exit 1; }

echo "📱 $(basename "$APK_SRC") → /var/www/lavender/lavender.apk"
cp -f "$APK_SRC" "$WEB_DIR/lavender.apk"

echo "🏷️  version.txt → $WEB_DIR/version.txt"
cp -f "$SCRIPT_DIR/version.txt" "$WEB_DIR/version.txt"

echo "📋 changelog.txt → $WEB_DIR/changelog.txt"
cp -f "$SCRIPT_DIR/changelog.txt" "$WEB_DIR/changelog.txt"

chown -R www-data:www-data "$WEB_DIR"

echo "🔍 Verification:"
echo -n "  version: "; cat "$WEB_DIR/version.txt"
echo -n "  APK:     "; du -h "$WEB_DIR/lavender.apk" | cut -f1
curl -s -o /dev/null -w "  /           → %{http_code}\n" http://127.0.0.1/
curl -s -o /dev/null -w "  /version    → %{http_code}\n" http://127.0.0.1/version.txt
curl -s -o /dev/null -w "  /download   → %{http_code}\n" http://127.0.0.1/download

echo "🚀 Done! http://13.140.25.249"
