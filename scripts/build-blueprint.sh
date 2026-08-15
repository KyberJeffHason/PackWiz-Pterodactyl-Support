#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.."&&pwd)";OUT="${1:-$ROOT/dist}"
mkdir -p "$OUT";OUT="$(cd "$OUT"&&pwd)";TMP="$(mktemp -d)";trap 'rm -rf -- "$TMP"' EXIT
cp -a "$ROOT/extension/." "$TMP/";(cd "$TMP"&&zip -qr "$OUT/packwizmanager.blueprint" .)
