#!/usr/bin/env sh
# Sample TMAegis Executable Plugin
# Reads JSON ScanReport from STDIN and prints a formatted notification summary.

PAYLOAD=$(cat)
TOTAL=$(echo "$PAYLOAD" | grep -o '"total_rules":[0-9]*' | cut -d: -f2)
PASSED=$(echo "$PAYLOAD" | grep -o '"passed_count":[0-9]*' | cut -d: -f2)
FAILED=$(echo "$PAYLOAD" | grep -o '"failed_count":[0-9]*' | cut -d: -f2)

echo "Webhook Notification Dispatched -> Total Rules: ${TOTAL:-0}, Passed: ${PASSED:-0}, Failed: ${FAILED:-0}"
