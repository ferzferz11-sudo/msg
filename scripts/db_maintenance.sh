#!/bin/bash
# db_maintenance.sh — Database integrity check and cleanup for Lavender Messenger
# Usage: ./db_maintenance.sh [--dry-run] [--prod] [--skip-files] [--skip-vacuum]
#   --dry-run      — show what would be deleted, don't execute
#   --prod         — run against prod DB (chat_db), default is dev (chat_db_dev)
#   --skip-files   — skip slow file orphan cleanup
#   --skip-vacuum  — skip VACUUM/ANALYZE at the end

set -euo pipefail

# === Configuration ===
DB_HOST="localhost"
DB_USER="postgres"
DB_NAME="chat_db_dev"
DRY_RUN=false
SKIP_FILES=false
SKIP_VACUUM=false

for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=true ;;
        --prod) DB_NAME="chat_db" ;;
        --skip-files) SKIP_FILES=true ;;
        --skip-vacuum) SKIP_VACUUM=true ;;
    esac
done

PSQL="psql -h $DB_HOST -U $DB_USER -d $DB_NAME -t -A -q"

echo "============================================"
echo "  Lavender Messenger DB Maintenance"
echo "  Database: $DB_NAME"
echo "  Mode: $([ "$DRY_RUN" = true ] && echo 'DRY RUN' || echo 'LIVE')"
echo "  Time: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "============================================"
echo ""

TOTAL_DELETED=0

# === Helper functions ===
run_count() {
    local sql="$1"
    $PSQL -c "$sql" 2>/dev/null | tr -d '[:space:]' || echo "0"
}

run_delete() {
    local description="$1"
    local count_sql="$2"
    local delete_sql="$3"
    local count
    count=$(run_count "$count_sql")

    if [ "$count" = "0" ] || [ -z "$count" ]; then
        echo "  ✅ $description: 0"
        return
    fi

    if [ "$DRY_RUN" = true ]; then
        echo "  ⚠️  $description: $count (would delete)"
    else
        echo "  🗑️  $description: $count — deleting..."
        $PSQL -c "$delete_sql" >/dev/null 2>&1
        echo "     Done."
    fi
    TOTAL_DELETED=$((TOTAL_DELETED + count))
}

# === 1. Orphaned messages ===
echo "=== 1. Orphaned messages ==="

run_delete "messages without chat" \
    "SELECT COUNT(*) FROM messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = messages.room_id)" \
    "DELETE FROM messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = messages.room_id)"

run_delete "reactions without message" \
    "SELECT COUNT(*) FROM reactions WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.message_id = reactions.message_id)" \
    "DELETE FROM reactions WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.message_id = reactions.message_id)"

run_delete "favorites without chat" \
    "SELECT COUNT(*) FROM favorites WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = favorites.room_id)" \
    "DELETE FROM favorites WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = favorites.room_id)"

run_delete "draft_messages without chat" \
    "SELECT COUNT(*) FROM draft_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = draft_messages.room_id)" \
    "DELETE FROM draft_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = draft_messages.room_id)"

echo ""
echo "=== 2. Orphaned chat metadata ==="

run_delete "muted_chats without chat" \
    "SELECT COUNT(*) FROM muted_chats WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = muted_chats.room_id)" \
    "DELETE FROM muted_chats WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = muted_chats.room_id)"

run_delete "user_chat_metadata without chat" \
    "SELECT COUNT(*) FROM user_chat_metadata WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = user_chat_metadata.room_id)" \
    "DELETE FROM user_chat_metadata WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = user_chat_metadata.room_id)"

echo ""
echo "=== 3. Orphaned secret chat + AI records ==="

run_delete "secret_chat_keys without chat" \
    "SELECT COUNT(*) FROM secret_chat_keys WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = secret_chat_keys.chat_id)" \
    "DELETE FROM secret_chat_keys WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = secret_chat_keys.chat_id)"

run_delete "owl_chat_settings without chat" \
    "SELECT COUNT(*) FROM owl_chat_settings WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_chat_settings.chat_id)" \
    "DELETE FROM owl_chat_settings WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_chat_settings.chat_id)"

run_delete "owl_messages without chat" \
    "SELECT COUNT(*) FROM owl_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_messages.chat_id)" \
    "DELETE FROM owl_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_messages.chat_id)"

run_delete "hermes_chat_settings without chat" \
    "SELECT COUNT(*) FROM hermes_chat_settings WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = hermes_chat_settings.chat_id)" \
    "DELETE FROM hermes_chat_settings WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = hermes_chat_settings.chat_id)"

run_delete "calls without chat" \
    "SELECT COUNT(*) FROM calls WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = calls.chat_id)" \
    "DELETE FROM calls WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = calls.chat_id)"

echo ""
echo "=== 4. Orphaned Hermes records ==="

run_delete "hermes_messages without session" \
    "SELECT COUNT(*) FROM hermes_messages WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_messages.session_id)" \
    "DELETE FROM hermes_messages WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_messages.session_id)"

run_delete "hermes_agent_runs without session" \
    "SELECT COUNT(*) FROM hermes_agent_runs WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_agent_runs.session_id)" \
    "DELETE FROM hermes_agent_runs WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_agent_runs.session_id)"

run_delete "hermes_sessions without user" \
    "SELECT COUNT(*) FROM hermes_sessions WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_sessions.user_id)" \
    "DELETE FROM hermes_sessions WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_sessions.user_id)"

run_delete "hermes_custom_agents without user" \
    "SELECT COUNT(*) FROM hermes_custom_agents WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_custom_agents.created_by)" \
    "DELETE FROM hermes_custom_agents WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_custom_agents.created_by)"

echo ""
echo "=== 5. Orphaned user records ==="

run_delete "user_tokens without user" \
    "SELECT COUNT(*) FROM user_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_tokens.user_id)" \
    "DELETE FROM user_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_tokens.user_id)"

run_delete "user_devices without user" \
    "SELECT COUNT(*) FROM user_devices WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_devices.user_id)" \
    "DELETE FROM user_devices WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_devices.user_id)"

run_delete "user_themes without user" \
    "SELECT COUNT(*) FROM user_themes WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_themes.user_id)" \
    "DELETE FROM user_themes WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_themes.user_id)"

run_delete "contacts without user" \
    "SELECT COUNT(*) FROM contacts WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = contacts.user_id)" \
    "DELETE FROM contacts WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = contacts.user_id)"

run_delete "password_reset_tokens without user" \
    "SELECT COUNT(*) FROM password_reset_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = password_reset_tokens.user_id)" \
    "DELETE FROM password_reset_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = password_reset_tokens.user_id)"

echo ""
echo "=== 6. Stale data cleanup ==="

run_delete "password_reset_tokens > 24h" \
    "SELECT COUNT(*) FROM password_reset_tokens WHERE created_at < NOW() - INTERVAL '24 hours'" \
    "DELETE FROM password_reset_tokens WHERE created_at < NOW() - INTERVAL '24 hours'"

run_delete "device_auth_log > 90 days" \
    "SELECT COUNT(*) FROM device_auth_log WHERE created_at < NOW() - INTERVAL '90 days'" \
    "DELETE FROM device_auth_log WHERE created_at < NOW() - INTERVAL '90 days'"

run_delete "expired user_devices (refresh_token_expires_at < now)" \
    "SELECT COUNT(*) FROM user_devices WHERE refresh_token_expires_at < NOW() AND is_active = TRUE" \
    "UPDATE user_devices SET is_active = FALSE WHERE refresh_token_expires_at < NOW() AND is_active = TRUE"

echo ""
echo "=== 7. File resources cleanup ==="

if [ "$SKIP_FILES" = true ]; then
    echo "  Skipped (--skip-files)"
else
    UPLOAD_DIR="/root/LavenderMessenger/run/uploads"
    if [ -d "$UPLOAD_DIR" ]; then
        # Get all filenames and referenced filenames, compare
        ALL_FILES=$(find "$UPLOAD_DIR" -type f -printf '%f\n' 2>/dev/null | sort -u)
        TOTAL_FILES=$(echo "$ALL_FILES" | grep -c . || echo "0")

        if [ "$TOTAL_FILES" = "0" ]; then
            echo "  ✅ No files in $UPLOAD_DIR"
        else
            # Single query to get all referenced filenames
            REFERENCED=$($PSQL -c "
                SELECT DISTINCT filename FROM (
                    SELECT unnest(string_to_array(COALESCE(avatar_url, ''), '/')) AS filename FROM users
                    UNION ALL
                    SELECT unnest(string_to_array(COALESCE(avatar_url, ''), '/')) FROM chats
                    UNION ALL
                    SELECT unnest(string_to_array(COALESCE(image_url, ''), '/')) FROM messages
                    UNION ALL
                    SELECT unnest(string_to_array(COALESCE(voice_url, ''), '/')) FROM messages
                ) sub WHERE filename != '' AND filename LIKE '%.%'
            " 2>/dev/null | sort -u)

            ORPHAN_COUNT=0
            while IFS= read -r fname; do
                [ -z "$fname" ] && continue
                if ! echo "$REFERENCED" | grep -qF "$fname"; then
                    ORPHAN_COUNT=$((ORPHAN_COUNT + 1))
                    if [ "$DRY_RUN" = true ]; then
                        echo "    ⚠️  Would delete: $fname"
                    else
                        find "$UPLOAD_DIR" -name "$fname" -type f -delete 2>/dev/null
                        echo "    🗑️  Deleted: $fname"
                    fi
                fi
            done <<< "$ALL_FILES"

            echo "  Total: $TOTAL_FILES, Orphans: $ORPHAN_COUNT"
            [ "$ORPHAN_COUNT" = "0" ] && echo "  ✅ No orphaned files"
            TOTAL_DELETED=$((TOTAL_DELETED + ORPHAN_COUNT))
        fi
    else
        echo "  Upload dir not found: $UPLOAD_DIR"
    fi
fi

echo ""
echo "=== 8. VACUUM / ANALYZE ==="

if [ "$SKIP_VACUUM" = true ]; then
    echo "  Skipped (--skip-vacuum)"
elif [ "$DRY_RUN" = true ]; then
    echo "  ⚠️  Would VACUUM ANALYZE (can't do in dry-run, requires no active transactions)"
else
    echo "  Running VACUUM ANALYZE..."
    $PSQL -c "VACUUM ANALYZE;" 2>/dev/null && echo "  ✅ VACUUM ANALYZE done" || echo "  ⚠️  VACUUM skipped (active transactions)"
fi

echo ""
echo "=== 9. Summary ==="

echo "  Table sizes:"
$PSQL -c "
SELECT '  ' || relname || ': ' || n_live_tup || ' rows'
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY n_live_tup DESC
LIMIT 15;
" 2>/dev/null

echo ""
echo "  DB size:"
$PSQL -c "SELECT '  ' || pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null

echo ""
echo "============================================"
echo "  Deleted/updated: $TOTAL_DELETED rows"
echo "  Maintenance complete."
echo "============================================"
