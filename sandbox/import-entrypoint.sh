#!/bin/sh
set -eu

# The container has only loopback; this socket reaches one scoped CONNECT proxy.
# No git settings, credentials, source directory, or Docker socket are mounted.
socat TCP4-LISTEN:3128,bind=127.0.0.1,reuseaddr,fork UNIX-CONNECT:/proxy.sock &
proxy_pid=$!
trap 'kill "$proxy_pid" 2>/dev/null || true' EXIT HUP INT TERM
attempt=0
until socat -T1 -u /dev/null TCP4:127.0.0.1:3128 2>/dev/null; do
    attempt=$((attempt + 1))
    [ "$attempt" -lt 50 ] || exit 1
    sleep 0.02
done

git -c http.proxy=http://127.0.0.1:3128 -c http.followRedirects=false \
    -c credential.helper= -c core.hooksPath=/dev/null -c init.templateDir= \
    clone --depth 1 --no-tags --single-branch -- "$1" /work/repo
tar -cf - --exclude='./.git' -C /work/repo .
