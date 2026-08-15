#!/usr/bin/env bash
set -Eeuo pipefail
PURGE=0;REMOVE_EXTENSION=0
for arg in "$@";do case "$arg" in --purge-data)PURGE=1;;--remove-extension)REMOVE_EXTENSION=1;;*)echo "Unknown option: $arg" >&2;exit 2;;esac;done
[[ $EUID -eq 0 ]]||{ echo "Run as root." >&2;exit 1; }
systemctl disable --now packwiz-manager 2>/dev/null||true
rm -f /etc/systemd/system/packwiz-manager.service /usr/local/bin/packwiz-manager /usr/lib/packwiz-manager/packwiz
systemctl daemon-reload
if ((REMOVE_EXTENSION))&&command -v blueprint >/dev/null;then (cd /var/www/pterodactyl&&blueprint -remove packwizmanager);fi
if ((PURGE));then rm -rf -- /srv/packwiz-manager /etc/packwiz-manager;userdel packwizmgr 2>/dev/null||true;else echo "Preserved /srv/packwiz-manager and /etc/packwiz-manager. Use --purge-data to remove.";fi
