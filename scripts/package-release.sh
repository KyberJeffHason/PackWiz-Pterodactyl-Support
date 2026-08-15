#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.."&&pwd)";ARCH="${GOARCH:-amd64}";OUT="$ROOT/dist";mkdir -p "$OUT";TMP="$(mktemp -d)";trap 'rm -rf -- "$TMP"' EXIT
install -m 0755 "$OUT/packwiz-manager" "$TMP/packwiz-manager";install -m 0755 "${PACKWIZ_BINARY:?set PACKWIZ_BINARY}" "$TMP/packwiz";cp "$ROOT/PACKWIZ_REF" "$ROOT/BOOTSTRAP_LOCK" "$TMP/";cp "$ROOT/installer/packwiz-manager.service" "$ROOT/installer/cloudflare.sh" "$TMP/";tar -czf "$OUT/packwiz-manager-linux-$ARCH.tar.gz" -C "$TMP" .
bash "$ROOT/scripts/build-blueprint.sh" "$OUT";cp "$ROOT/installer/"{install,uninstall}.sh "$OUT/";(cd "$OUT"&&sha256sum packwiz-manager-linux-*.tar.gz packwizmanager.blueprint install.sh uninstall.sh >SHA256SUMS)
