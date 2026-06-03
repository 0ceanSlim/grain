#!/usr/bin/env bash
# Throwaway local-CI helper. Builds the nostrdb cgo dependency from a clean
# in-container copy of the tree (the host checkout has CRLF line endings from
# core.autocrlf=true, which break the vendored autotools scripts in Linux) and
# runs the Go build/tests for the memory-leak fixes. Safe to delete.
set -euo pipefail

echo "=== installing toolchain (autotools) ==="
apt-get update -qq
apt-get install -y -qq autoconf automake libtool >/dev/null

echo "=== copying tree to /build (excluding .git) ==="
mkdir -p /build
tar -C /src --exclude=./.git -cf - . | tar -C /build -xf -
cd /build

echo "=== forcing a clean Linux build of vendored C deps ==="
find server/db/nostrdb/c -type f \( -name '*.o' -o -name '*.a' -o -name '*.lo' -o -name '*.la' \) -delete 2>/dev/null || true
rm -rf server/db/nostrdb/lib/* server/db/nostrdb/include/* 2>/dev/null || true

echo "=== normalizing line endings under nostrdb (sed; mirrors tests/docker/Dockerfile) ==="
find server/db/nostrdb -type f \! -name '*.o' \! -name '*.a' \! -name '*.png' \! -name '*.pdf' -exec sed -i 's/\r$//' {} +

echo "=== building nostrdb static lib (slow, one-time) ==="
( cd server/db/nostrdb && CC=cc bash build.sh >/tmp/ndb-build.log 2>&1 ) || { echo "--- BUILD FAILED, tail of log: ---"; tail -60 /tmp/ndb-build.log; exit 1; }
echo "nostrdb lib built OK."

echo "=== go build ./... (full compile, all packages) ==="
go build -buildvcs=false ./...

echo "=== go vet ./server/... ./config/... ./client/... ==="
go vet -buildvcs=false ./server/... ./config/... ./client/...

echo "=== go test ./server/... ./config/... (unit tests; excludes integration) ==="
go test -buildvcs=false ./server/... ./config/...

echo "=== ALL DONE ==="
