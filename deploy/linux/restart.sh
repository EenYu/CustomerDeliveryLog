#!/bin/bash

set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
STOP_SCRIPT="$BASE_DIR/stop.sh"
START_SCRIPT="$BASE_DIR/start.sh"

if [ -f "$STOP_SCRIPT" ]; then
  "$STOP_SCRIPT"
fi

"$START_SCRIPT"
