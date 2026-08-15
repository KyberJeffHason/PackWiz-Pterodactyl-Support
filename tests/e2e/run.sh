#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE="$ROOT/tests/e2e/docker-compose.yml"
cleanup(){ rc=$?;trap - EXIT;if ((rc));then docker compose -f "$COMPOSE" logs --no-color --tail=200 panel database cache||true;fi;docker compose -f "$COMPOSE" down -v||true;exit "$rc"; }
trap cleanup EXIT
bash "$ROOT/scripts/build-blueprint.sh" "$ROOT/dist"
echo 'E2E: starting Blueprint stack'
docker compose -f "$COMPOSE" up -d --wait
ready=0
for _ in {1..60};do if docker compose -f "$COMPOSE" exec -T panel php artisan migrate --force --no-interaction;then ready=1;break;fi;sleep 2;done
((ready))||{ echo 'Panel migrations did not become ready.' >&2;exit 1; }
echo 'E2E: installing extension'
docker compose -f "$COMPOSE" exec -T panel cp /blueprint_extensions/packwizmanager.blueprint /app/packwizmanager.blueprint
docker compose -f "$COMPOSE" exec -T panel blueprint -i packwizmanager
echo 'E2E: checking installed frontend and PHP'
# Variables expand inside container shell.
# shellcheck disable=SC2016
docker compose -f "$COMPOSE" exec -T panel sh -lc 'test -n "$(find /app -name PackwizRoute.tsx -print -quit)"'
# Variables expand inside container shell.
# shellcheck disable=SC2016
docker compose -f "$COMPOSE" exec -T panel sh -lc 'file="$(find /app/app -name PackwizProxyController.php -print -quit)";test -n "$file";php -l "$file"'
docker compose -f "$COMPOSE" exec -T panel php artisan migrate:status
echo 'E2E: checking HTTP and registered client route'
curl --retry 10 --retry-all-errors --retry-delay 1 -fsS http://127.0.0.1:8088 >/dev/null
status="$(curl -sS -o /tmp/packwiz-route-response -w '%{http_code}' http://127.0.0.1:8088/api/client/extensions/packwizmanager/servers/test/projects)"
case "$status" in 401|403) ;;*) echo "Unexpected Packwiz route status: $status" >&2;cat /tmp/packwiz-route-response >&2;exit 1;;esac
echo 'E2E: passed'
