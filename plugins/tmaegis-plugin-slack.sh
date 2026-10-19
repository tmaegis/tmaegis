#!/usr/bin/env sh
# TMAegis notifier plugin: posts a scan summary to a Slack incoming webhook.
# Config: SLACK_WEBHOOK_URL (required). Reads the JSON ScanReport on stdin.
set -eu

: "${SLACK_WEBHOOK_URL:?set SLACK_WEBHOOK_URL to a Slack incoming webhook URL}"
PAYLOAD=$(cat)
TOTAL=$(printf '%s' "$PAYLOAD" | grep -o '"total_rules":[0-9]*' | head -n1 | cut -d: -f2)
FAILED=$(printf '%s' "$PAYLOAD" | grep -o '"failed_count":[0-9]*' | head -n1 | cut -d: -f2)
HOST=$(hostname 2>/dev/null || echo unknown)

BODY=$(printf '{"text":"TMAegis scan on %s: %s of %s controls failed"}' "$HOST" "${FAILED:-0}" "${TOTAL:-0}")
curl -fsS -X POST -H 'Content-Type: application/json' --data "$BODY" "$SLACK_WEBHOOK_URL" >/dev/null
echo "Slack notification sent (${FAILED:-0}/${TOTAL:-0} failed)"
