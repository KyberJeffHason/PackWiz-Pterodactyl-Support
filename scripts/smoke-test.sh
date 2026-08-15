#!/usr/bin/env bash
set -Eeuo pipefail
DIR="${1:-dist}";test -s "$DIR/packwizmanager.blueprint";test -s "$DIR/SHA256SUMS";(cd "$DIR"&&sha256sum -c SHA256SUMS);unzip -t "$DIR/packwizmanager.blueprint" >/dev/null;tar -tzf "$DIR"/packwiz-manager-linux-*.tar.gz >/dev/null
