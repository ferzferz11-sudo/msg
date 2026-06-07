#!/bin/bash
# db_maintenance.sh — Database integrity check and cleanup for Lavender Messenger
# Usage: ./db_maintenance.sh [--dry-run] [--prod]
#   --dry-run  — show what would be deleted, don't execute
#   --prod     — run against prod DB (chat_db), default is dev (chat_db_dev)

set -euo pipefail

# === Configuration ===
DB_HOST="localhost"
DB_USER="postgres"
DB_NAME="chat_db_dev"
DRY_RUN=false

for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=true ;;
        --prod) DB_NAME="chat_db" ;;
    esac
done

PSQL="psql -h $DB_HOST -U $DB_USER -d $DB_NAME -t -A"

echo "============================================"
echo "  Lavender Messenger DB Maintenance"
echo "  Database: $DB_NAME"
echo "  Mode: $([ "$DRY_RUN" = true ] && echo 'DRY RUN' || echo 'LIVE')"
echo "  Time: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "============================================"
echo ""

# === Helper functions ===
count_orphans() {
    local sql="$1"
    $PSQL -c "$sql" 2>/dev/null || echo "0"
}

delete_orphans() {
    local description="$1"
    local sql="$1"
    local count
    count=$(count_orphans "$sql")

    if [ "$count" = "0" ] || [ -z "$count" ]; then
        echo "  ✅ $description: 0"
        return
    fi

    if [ "$DRY_RUN" = true ]; then
        echo "  ⚠️  $description: $count (would delete)"
    else
        echo "  🗑️  $description: $count — deleting..."
        $PSQL -c "$sql" >/dev/null 2>&1
        echo "     Done."
    fi
}

# === 1. Integrity checks ===
echo "=== 1. Orphaned records ==="

# Messages without a valid chat
delete_orphans "messages without chat" \
    "DELETE FROM messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = messages.room_id)"

# Reactions without a valid message
delete_orphans "reactions without message" \
    "DELETE FROM reactions WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.id = reactions.message_id)"

# Favorites without a valid chat
delete_orphans "favorites without chat" \
    "DELETE FROM favorites WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = favorites.room_id)"

# Muted chats without a valid chat
delete_orphans "muted_chats without chat" \
    "DELETE FROM muted_chats WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = muted_chats.room_id)"

# Chat metadata without a valid chat
delete_orphans "user_chat_metadata without chat" \
    "DELETE FROM user_chat_metadata WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = user_chat_metadata.room_id)"

# Draft messages without a valid chat
delete_orphans "draft_messages without chat" \
    "DELETE FROM draft_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = draft_messages.room_id)"

# Secret chat keys without a valid chat
delete_orphans "secret_chat_keys without chat" \
    "DELETE FROM secret_chat_keys WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = secret_chat_keys.chat_id)"

# OWL chat settings without a valid chat
delete_orphans "owl_chat_settings without chat" \
    "DELETE FROM owl_chat_settings WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_chat_settings.chat_id)"

# OWL messages without a valid chat
delete_orphans "owl_messages without chat" \
    "DELETE FROM owl_messages WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = owl_messages.chat_id)"

# Calls without a valid chat
delete_orphans "calls without chat" \
    "DELETE FROM calls WHERE NOT EXISTS (SELECT 1 FROM chats c WHERE c.id = calls.chat_id)"

echo ""
echo "=== 2. Orphaned Hermes records ==="

# Hermes sessions without a valid user
delete_orphans "hermes_sessions without user" \
    "DELETE FROM hermes_sessions WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_sessions.user_id)"

# Hermes messages without a valid session
delete_orphans "hermes_messages without session" \
    "DELETE FROM hermes_messages WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_messages.session_id)"

# Hermes agent runs without a valid session
delete_orphans "hermes_agent_runs without session" \
    "DELETE FROM hermes_agent_runs WHERE NOT EXISTS (SELECT 1 FROM hermes_sessions hs WHERE hs.id = hermes_agent_runs.session_id)"

# Hermes custom agents without a valid user
delete_orphans "hermes_custom_agents without user" \
    "DELETE FROM hermes_custom_agents WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = hermes_custom_agents.created_by)"

echo ""
echo "=== 3. Orphaned user records ==="

# User tokens without a valid user
delete_orphans "user_tokens without user" \
    "DELETE FROM user_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_tokens.user_id)"

# User devices without a valid user
delete_orphans "user_devices without user" \
    "DELETE FROM user_devices WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_devices.user_id)"

# User themes without a valid user
delete_orphans "user_themes without user" \
    "DELETE FROM user_themes WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = user_themes.user_id)"

# Contacts without a valid user
delete_orphans "contacts without user" \
    "DELETE FROM contacts WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = contacts.user_id)"

# Password reset tokens without a valid user
delete_orphans "password_reset_tokens without user" \
    "DELETE FROM password_reset_tokens WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = password_reset_tokens.user_id)"

echo ""
echo "=== 4. Stale password reset tokens ==="

# Delete password reset tokens older than 24 hours
delete_orphans "password_reset_tokens older than 24h" \
    "DELETE FROM password_reset_tokens WHERE created_at < NOW() - INTERVAL '24 hours'"

echo ""
echo "=== 5. File resources cleanup ==="

AVATAR_DIR="/root/LavenderMessenger/run/uploads/avatars"
IMAGES_DIR="/root/LavenderMessenger/run/uploads/images"

# Find avatar files not referenced by any user
if [ -d "$AVATAR_DIR" ]; then
    echo "  Checking orphaned avatars in $AVATAR_DIR..."
    ORPHAN_FILES=0
    for f in "$AVATAR_DIR"/*; do
        [ -f "$f" ] || continue
        fname=$(basename "$f")
        # Check if any user references this file
        ref_count=$($PSQL -c "SELECT COUNT(*) FROM users WHERE avatar_url LIKE '%$fname%'" 2>/dev/null || echo "0")
        if [ "$ref_count" = "0" ]; then
            ORPHAN_FILES=$((ORPHAN_FILES + 1))
            if [ "$DRY_RUN" = true ]; then
                echo "    ⚠️  Would delete: $fname"
            else
                echo "    🗑️  Deleting: $fname"
                rm -f "$f"
            fi
        fi
    done
    if [ "$ORPHAN_FILES" = "0" ]; then
        echo "  ✅ No orphaned avatars"
    else
        echo "  $([ "$DRY_RUN" = true ] && echo '⚠️' || echo '🗑️')  Total orphaned avatars: $ORPHAN_FILES"
    fi
fi

# Find image files not referenced by any message
if [ -d "$IMAGES_DIR" ]; then
    echo "  Checking orphaned images in $IMAGES_DIR..."
    ORPHAN_FILES=0
    for f in "$IMAGES_DIR"/*; do
        [ -f "$f" ] || continue
        fname=$(basename "$f")
        ref_count=$($PSQL -c "SELECT COUNT(*) FROM messages WHERE image_url LIKE '%$fname%' OR image_urls LIKE '%$fname%'" 2>/dev/null || echo "0")
        if [ "$ref_count" = "0" ]; then
            ORPHAN_FILES=$((ORPHAN_FILES + 1))
            if [ "$DRY_RUN" = true ]; then
                echo "    ⚠️  Would delete: $fname"
            else
                echo "    🗑️  Deleting: $fname"
                rm -f "$f"
            fi
        fi
    done
    if [ "$ORPHAN_FILES" = "0" ]; then
        echo "  ✅ No orphaned images"
    else
        echo "  $([ "$DRY_RUN" = true ] && echo '⚠️' || echo '🗑️')  Total orphaned images: $ORPHAN_FILES"
    fi
fi

echo ""
echo "=== 6. Summary ==="

# Table row counts
echo "  Table row counts:"
$PSQL -c "SELECT '  users              : ' || COUNT(*) FROM users; SELECT '  chats               : ' || COUNT(*) FROM chats; SELECT '  messages            : ' || COUNT(*) FROM messages; SELECT '  hermes_sessions     : ' || COUNT(*) FROM hermes_sessions; SELECT '  hermes_messages     : ' || COUNT(*) FROM hermes_messages; SELECT '  hermes_agent_runs   : ' || COUNT(*) FROM hermes_agent_runs; SELECT '  hermes_custom_agents: ' || COUNT(*) FROM hermes_custom_agents; SELECT '  contacts            : ' || COUNT(*) FROM contacts; SELECT '  user_devices        : ' || COUNT(*) FROM user_devices;" 2>/dev/null

echo ""
echo "============================================"
echo "  Maintenance complete."
echo "============================================"
