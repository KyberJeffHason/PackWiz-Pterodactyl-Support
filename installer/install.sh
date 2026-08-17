#!/usr/bin/env bash
set -Eeuo pipefail
umask 027

VERSION="latest"; REPO="packwiz-manager/packwiz-manager"; SERVICE_ONLY=0; COMPOSE_FILE=""; EXTENSIONS_DIR=""; PACK_HOST=""; CONFIGURE_CLOUDFLARE=0
usage(){ echo "Usage: install.sh [--version v0.2.0] [--repo owner/repo] [--service-only] [--compose-file FILE] [--extensions-dir DIR] [--pack-host URL] [--configure-cloudflare]"; }
while (($#)); do case "$1" in --version) VERSION="$2";shift 2;;--repo) REPO="$2";shift 2;;--service-only) SERVICE_ONLY=1;shift;;--compose-file) COMPOSE_FILE="$2";shift 2;;--extensions-dir) EXTENSIONS_DIR="$2";shift 2;;--pack-host) PACK_HOST="$2";shift 2;;--configure-cloudflare)CONFIGURE_CLOUDFLARE=1;shift;;-h|--help)usage;exit;;*)echo "Unknown option: $1" >&2;usage;exit 2;;esac;done
[[ $EUID -eq 0 ]] || { echo "Run as root for system service installation." >&2;exit 1; }
case "$(uname -m)" in x86_64) ARCH=amd64;;aarch64|arm64)ARCH=arm64;;*)echo "Unsupported architecture" >&2;exit 1;;esac
command -v curl >/dev/null; command -v sha256sum >/dev/null; command -v systemctl >/dev/null
if [[ $VERSION == latest ]];then VERSION="$(curl -fsS --proto '=https' --tlsv1.2 "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)";fi
[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid release version" >&2;exit 1; }
TMP="$(mktemp -d)";trap 'rm -rf -- "$TMP"' EXIT
BASE="https://github.com/$REPO/releases/download/$VERSION";ASSET="packwiz-manager-linux-$ARCH.tar.gz"
curl -fL --proto '=https' --tlsv1.2 -o "$TMP/$ASSET" "$BASE/$ASSET";curl -fL --proto '=https' --tlsv1.2 -o "$TMP/SHA256SUMS" "$BASE/SHA256SUMS"
(cd "$TMP"&&sha256sum -c SHA256SUMS --ignore-missing);tar -xzf "$TMP/$ASSET" -C "$TMP"
id packwizmgr >/dev/null 2>&1||useradd --system --home /srv/packwiz-manager --shell /usr/sbin/nologin packwizmgr
install -d -o root -g packwizmgr -m 0750 /etc/packwiz-manager;install -d -o packwizmgr -g packwizmgr -m 0750 /srv/packwiz-manager/{db,projects,blobs,releases,tmp};install -d -m 0755 /usr/lib/packwiz-manager
install -m 0755 "$TMP/packwiz-manager" /usr/local/bin/packwiz-manager;install -m 0755 "$TMP/packwiz" /usr/lib/packwiz-manager/packwiz
if [[ ! -f /etc/packwiz-manager/service.token ]];then (umask 077;openssl rand -hex 32 >/etc/packwiz-manager/service.token);fi
TOKEN="$(</etc/packwiz-manager/service.token)";if [[ ! -f /etc/packwiz-manager/config.env ]];then { echo "PWM_SERVICE_TOKEN=$TOKEN";echo "PWM_LISTEN=127.0.0.1:8090";echo "PWM_PUBLIC_LISTEN=127.0.0.1:8091";echo "PWM_DATA_DIR=/srv/packwiz-manager";echo "PWM_PUBLIC_BASE_URL=${PACK_HOST:-http://127.0.0.1:8091/public}";echo "PWM_PACKWIZ_COMMIT=$(<"$TMP/PACKWIZ_REF")";} >/etc/packwiz-manager/config.env;chmod 0600 /etc/packwiz-manager/config.env;fi
install -m 0644 "$TMP/packwiz-manager.service" /etc/systemd/system/packwiz-manager.service
systemctl daemon-reload
systemctl enable packwiz-manager
# Always restart after replacing the executable. `enable --now` only starts an
# inactive service and leaves an already-running old binary in memory.
systemctl restart packwiz-manager
for _ in {1..30};do curl -fsS http://127.0.0.1:8090/readyz >/dev/null&&break;sleep 1;done;curl -fsS http://127.0.0.1:8090/healthz >/dev/null;curl -fsS http://127.0.0.1:8090/readyz >/dev/null
if ((CONFIGURE_CLOUDFLARE));then bash "$TMP/cloudflare.sh";fi
((SERVICE_ONLY))&&exit 0
if [[ -n $COMPOSE_FILE ]];then grep -qi blueprint "$COMPOSE_FILE"||{ echo "Stock Pterodactyl Docker detected. Service installed; extension skipped because Blueprint image is required." >&2;exit 3;};EXTENSIONS_DIR="${EXTENSIONS_DIR:-/srv/pterodactyl/extensions}";install -m 0644 "$TMP/packwizmanager.blueprint" "$EXTENSIONS_DIR/packwizmanager.blueprint";echo "Blueprint Docker extension staged at $EXTENSIONS_DIR; restart panel container to install.";elif command -v blueprint >/dev/null;then install -m 0644 "$TMP/packwizmanager.blueprint" /var/www/pterodactyl/packwizmanager.blueprint;(cd /var/www/pterodactyl&&blueprint -install packwizmanager);else echo "Blueprint not detected. Service installed; extension skipped. Install Blueprint, then rerun." >&2;exit 3;fi
