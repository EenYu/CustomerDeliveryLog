#!/bin/bash

set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_BIN="$BASE_DIR/bin/customer-delivery-log"
APP_ENV="$BASE_DIR/config/app.env"
LOG_DIR="$BASE_DIR/logs"
RUN_DIR="$BASE_DIR/run"
PID_FILE="$RUN_DIR/app.pid"
OUT_FILE="$LOG_DIR/app.out"

mkdir -p "$LOG_DIR" "$RUN_DIR" "$BASE_DIR/uploads"

if [ ! -f "$APP_BIN" ]; then
  echo "binary not found: $APP_BIN"
  exit 1
fi

find_running_pids() {
  pgrep -f "$APP_BIN" || true
}

if [ -f "$PID_FILE" ]; then
  OLD_PID="$(cat "$PID_FILE")"
  if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" >/dev/null 2>&1; then
    echo "application already running, pid=$OLD_PID"
    exit 0
  fi
  rm -f "$PID_FILE"
fi

mapfile -t RUNNING_PIDS < <(find_running_pids)
if [ "${#RUNNING_PIDS[@]}" -gt 0 ]; then
  echo "${RUNNING_PIDS[0]}" > "$PID_FILE"
  echo "application already running, pid=${RUNNING_PIDS[0]}"
  exit 0
fi

if [ -f "$APP_ENV" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$APP_ENV"
  set +a
fi

cd "$BASE_DIR"

chmod +x "$APP_BIN"

nohup "$APP_BIN" >> "$OUT_FILE" 2>&1 &
APP_PID=$!
echo "$APP_PID" > "$PID_FILE"

sleep 1
if kill -0 "$APP_PID" >/dev/null 2>&1; then
  echo "application started successfully, pid=$APP_PID"
  echo "log file: $OUT_FILE"
else
  echo "application failed to start, check log: $OUT_FILE"
  exit 1
fi
