#!/usr/bin/env sh
# TMAegis notifier plugin: sends a scan summary to a Telegram chat.
# Config: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID (both required).
set -eu

: "${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"
: "${TELEGRAM_CHAT_ID:?set TELEGRAM_CHAT_ID}"
PAYLOAD=$(cat)
TOTAL=$(printf '%s' "$PAYLOAD" | grep -o '"total_rules":[0-9]*' | head -n1 | cut -d: -f2)
FAILED=$(printf '%s' "$PAYLOAD" | grep -o '"failed_count":[0-9]*' | head -n1 | cut -d: -f2)
HOST=$(hostname 2>/dev/null || echo unknown)

curl -fsS "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  --data-urlencode "chat_id=${TELEGRAM_CHAT_ID}" \
  --data-urlencode "text=TMAegis scan on ${HOST}: ${FAILED:-0} of ${TOTAL:-0} controls failed" >/dev/null
echo "Telegram notification sent (${FAILED:-0}/${TOTAL:-0} failed)"
