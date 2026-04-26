#!/bin/bash

# Lavender Messenger Database Maintenance Script (Run on Local Mac)
# This script deploys and executes a cleanup script on the remote server.

REMOTE_USER="ferz"
REMOTE_HOST="159.195.38.145"
REMOTE_PORT="31703"
REMOTE_KEY="$HOME/.ssh/ferzz@x-cart.com"
REMOTE_DIR="/home/ferz/LavenderMessenger"

echo "🚀 Starting database maintenance on $REMOTE_HOST..."

# Use a Here-doc to execute commands remotely
ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST << 'REMOTE_EXEC'
cd /home/ferz/LavenderMessenger

# Load database URL from .env if it exists
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

if [ -z "$DATABASE_URL" ]; then
    echo "❌ Error: DATABASE_URL not found in environment or .env file."
    exit 1
fi

# Suppress locale warnings for the current session
export LC_ALL=C
export LANG=C

echo "🔍 Analyzing and fixing database records..."

# Use psql to run maintenance queries
psql "$DATABASE_URL" << 'SQL'
\set ON_ERROR_STOP on

-- 1. Identify and fix "Encrypted Empty Strings" (length 28 in AES-GCM)
-- These cause "decrypted to empty string" warnings on the server.
-- We change them to a dummy value to trigger a decryption error instead of empty result.
-- Decryption error leads to "[Decryption Error]" text which prevents server skipping the message.
-- Client side handles this by hiding text if voice_url is present.

UPDATE messages
SET encrypted_text = 'FIXED_BY_MAINTENANCE'::bytea
WHERE length(encrypted_text) = 28 AND (voice_url IS NOT NULL AND voice_url != '');

-- 2. General cleanup for any other potentially corrupted/empty messages
UPDATE messages
SET encrypted_text = 'EMPTY_FIX'::bytea
WHERE (length(encrypted_text) < 4 OR encrypted_text IS NULL);

-- 3. Cleanup orphan reactions
DELETE FROM reactions WHERE message_id NOT IN (SELECT message_id FROM messages);

-- 4. Cleanup orphan contacts
DELETE FROM contacts WHERE user_id NOT IN (SELECT id FROM users) OR contact_id NOT IN (SELECT id FROM users);

-- 5. Targeted Vacuum
VACUUM ANALYZE messages;
VACUUM ANALYZE reactions;
VACUUM ANALYZE contacts;
VACUUM ANALYZE users;
SQL

if [ $? -eq 0 ]; then
    echo "✅ Database maintenance completed successfully!"
else
    echo "❌ Database maintenance failed!"
    exit 1
fi
REMOTE_EXEC

echo "✅ All tasks completed!"
