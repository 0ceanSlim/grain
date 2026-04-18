#!/bin/bash
# Build nostrdb and all dependencies into a single static library for CGO linking.
# Run this from the server/db/nostrdb/ directory.
# Requires: gcc, make, autotools (autoconf, automake, libtool)
#
# Environment variables:
#   CC           — C compiler (default: cc)
#   EXTRA_CFLAGS — extra flags prepended to CFLAGS (e.g. "-arch x86_64")

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
NDB_DIR="$SCRIPT_DIR/c/nostrdb"
BUILD_DIR="$SCRIPT_DIR/lib"
INCLUDE_DIR="$SCRIPT_DIR/include"

CC="${CC:-cc}"

# Ensure a writable temp directory exists (MinGW ar.exe needs it on Windows)
export TMPDIR="${TMPDIR:-/tmp}"
mkdir -p "$TMPDIR" 2>/dev/null || true

mkdir -p "$BUILD_DIR" "$INCLUDE_DIR"

echo "=== Building nostrdb dependencies ==="

cd "$NDB_DIR"

# 1. Build LMDB
echo "--- Building LMDB ---"
make CC="${CC}" -C deps/lmdb liblmdb.a

# 2. Build secp256k1
echo "--- Building secp256k1 ---"
if [ ! -f deps/secp256k1/configure ]; then
    cd deps/secp256k1
    ./autogen.sh
    cd "$NDB_DIR"
fi
if [ ! -f deps/secp256k1/config.log ]; then
    cd deps/secp256k1
    CC="${CC}" ./configure --disable-shared --enable-module-ecdh --enable-module-schnorrsig --enable-module-extrakeys
    cd "$NDB_DIR"
fi
make -C deps/secp256k1 -j libsecp256k1.la

# 3. Build libsodium
echo "--- Building libsodium ---"
if [ ! -f deps/libsodium/config.log ]; then
    cd deps/libsodium
    CC="${CC}" ./configure --disable-shared --enable-minimal
    cd "$NDB_DIR"
fi
cd deps/libsodium/src/libsodium
make -j libsodium.la
cd "$NDB_DIR"

# 4. Build nostrdb object files
echo "--- Building nostrdb ---"
CFLAGS="${EXTRA_CFLAGS:-} -Wall -Wno-misleading-indentation -Wno-unused-function -O2 -Isrc -Ideps/secp256k1/include -Ideps/lmdb -Ideps/flatcc/include -Isrc/bolt11/ -Iccan/ -Ideps/libsodium/src/libsodium/include/ -DCCAN_TAL_NEVER_RETURN_NULL=1 -fPIC"

SRCS="
src/base64.c
src/hmac_sha256.c
src/hkdf_sha256.c
src/nip44.c
src/nostrdb.c
src/invoice.c
src/nostr_bech32.c
src/content_parser.c
src/block.c
src/binmoji.c
src/metadata.c
src/bolt11/bolt11.c
src/bolt11/bech32.c
src/bolt11/amount.c
src/bolt11/hash_u5.c
deps/flatcc/src/runtime/json_parser.c
deps/flatcc/src/runtime/verifier.c
deps/flatcc/src/runtime/builder.c
deps/flatcc/src/runtime/emitter.c
deps/flatcc/src/runtime/refmap.c
ccan/ccan/utf8/utf8.c
ccan/ccan/tal/tal.c
ccan/ccan/tal/str/str.c
ccan/ccan/list/list.c
ccan/ccan/mem/mem.c
ccan/ccan/crypto/sha256/sha256.c
ccan/ccan/take/take.c
"

OBJS=""
for src in $SRCS; do
    obj="${src%.c}.o"
    echo "  CC $src"
    ${CC} $CFLAGS -c -o "$obj" "$src"
    OBJS="$OBJS $obj"
done

# 5. Create combined static library
echo "--- Creating combined static library ---"

# First create nostrdb archive from our objects
ar rcs libnostrdb.a $OBJS

# Combine all static libraries into one.
# macOS ar doesn't support MRI scripts, so use libtool -static there.
case "$(uname -s)" in
    Darwin)
        libtool -static -o "$BUILD_DIR/libnostrdb_full.a" \
            "$NDB_DIR/libnostrdb.a" \
            "$NDB_DIR/deps/lmdb/liblmdb.a" \
            "$NDB_DIR/deps/secp256k1/.libs/libsecp256k1.a" \
            "$NDB_DIR/deps/libsodium/src/libsodium/.libs/libsodium.a"
        ;;
    *)
        cat > "$BUILD_DIR/combine.mri" << EOF
create $BUILD_DIR/libnostrdb_full.a
addlib $NDB_DIR/libnostrdb.a
addlib $NDB_DIR/deps/lmdb/liblmdb.a
addlib $NDB_DIR/deps/secp256k1/.libs/libsecp256k1.a
addlib $NDB_DIR/deps/libsodium/src/libsodium/.libs/libsodium.a
save
end
EOF
        ar -M < "$BUILD_DIR/combine.mri"
        ;;
esac

echo "=== Static library built: $BUILD_DIR/libnostrdb_full.a ==="

# 6. Copy headers (including transitive dependencies referenced by nostrdb.h)
echo "--- Copying headers ---"
cp src/nostrdb.h "$INCLUDE_DIR/"
cp src/cursor.h "$INCLUDE_DIR/"
cp src/str_block.h "$INCLUDE_DIR/"
cp src/typedefs.h "$INCLUDE_DIR/"
cp src/win.h "$INCLUDE_DIR/"
cp src/nip44.h "$INCLUDE_DIR/"
cp deps/lmdb/lmdb.h "$INCLUDE_DIR/"
cp deps/secp256k1/include/secp256k1.h "$INCLUDE_DIR/"

# Copy config.h needed by ccan headers
cp src/config.h "$INCLUDE_DIR/"

# Copy ccan headers referenced by cursor.h and other nostrdb headers
cp -r ccan/ccan "$INCLUDE_DIR/ccan"

# Copy flatcc headers referenced by nostrdb.h
cp -r deps/flatcc/include/flatcc "$INCLUDE_DIR/flatcc"

echo "=== Build complete ==="
echo "Library: $BUILD_DIR/libnostrdb_full.a"
echo "Headers: $INCLUDE_DIR/"
