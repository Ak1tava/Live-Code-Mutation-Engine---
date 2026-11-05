#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
set -a
if [ -f .env ]; then . ./.env; fi
set +a
exec ./bin/lcme
