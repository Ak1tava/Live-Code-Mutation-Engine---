#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
LCME_RUNNER=local LCME_ADDR=127.0.0.1:8080 LCME_PUBLIC_URL=http://localhost:5173 ./bin/lcme &
backend_pid=$!
frontend_pid=
cleanup() {
  kill "$backend_pid" 2>/dev/null || true
  if [ -n "$frontend_pid" ]; then kill "$frontend_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM
(cd web && exec npm run dev -- --host 127.0.0.1) &
frontend_pid=$!
wait "$backend_pid"
