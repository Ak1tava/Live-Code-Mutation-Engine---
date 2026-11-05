#!/bin/sh
set -eu

# /tmp is a 256 MiB tmpfs supplied by Docker.Run. Copying keeps Go's warmed
# cache readable and writable without allowing writes to the container layer.
mkdir -p /tmp/gocache
cp -R /opt/gocache/. /tmp/gocache/
exec "$@"
