#!/bin/bash

set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_BIN="$BASE_DIR/bin/customer-delivery-log"
RUN_DIR="$BASE_DIR/run"
PID_FILE="$RUN_DIR/app.pid"

declare -A PID_SET=()

if [ -f "$PID_FILE" ]; then
  APP_PID="$(cat "$PID_FILE")"
  if [ -n "$APP_PID" ] && kill -0 "$APP_PID" >/dev/null 2>&1; then
    PID_SET["$APP_PID"]=1
  fi
fi

while IFS= read -r pid; do
  if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
    PID_SET["$pid"]=1
  fi
done < <(pgrep -f "$APP_BIN" || true)

if [ "${#PID_SET[@]}" -eq 0 ]; then
  rm -f "$PID_FILE"
  echo "application is not running"
  exit 0
fi

for pid in "${!PID_SET[@]}"; do
  kill "$pid" >/dev/null 2>&1 || true
done

for _ in $(seq 1 20); do
  REMAINING=0
  for pid in "${!PID_SET[@]}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      REMAINING=1
      break
    fi
  done
  if [ "$REMAINING" -eq 0 ]; then
    rm -f "$PID_FILE"
    echo "application stopped"
    exit 0
  fi
  sleep 1
done

for pid in "${!PID_SET[@]}"; do
  kill -9 "$pid" >/dev/null 2>&1 || true
done
rm -f "$PID_FILE"
echo "application stopped forcefully"
