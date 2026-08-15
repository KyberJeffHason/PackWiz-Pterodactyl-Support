#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE="$ROOT/tests/e2e/docker-compose.yml"
cleanup(){ rc=$?;trap - EXIT;if ((rc));then docker compose -f "$COMPOSE" logs --no-color --tail=200 panel database cache||true;fi;docker compose -f "$COMPOSE" down -v||true;exit "$rc"; }
trap cleanup EXIT
bash "$ROOT/scripts/build-blueprint.sh" "$ROOT/dist"
docker compose -f "$COMPOSE" up -d --wait
ready=0
for _ in {1..60};do if docker compose -f "$COMPOSE" exec -T panel php artisan migrate --force --no-interaction;then ready=1;break;fi;sleep 2;done
((ready))||{ echo 'Panel migrations did not become ready.' >&2;exit 1; }
docker compose -f "$COMPOSE" exec -T panel cp /blueprint_extensions/packwizmanager.blueprint /app/packwizmanager.blueprint
docker compose -f "$COMPOSE" exec -T panel blueprint -i packwizmanager
docker compose -f "$COMPOSE" exec -T panel php artisan route:list --path=api/client/extensions/packwizmanager | grep -q packwizmanager
docker compose -f "$COMPOSE" exec -T panel sh -lc 'test -d /app/resources/scripts/components/blueprint/extensions/packwizmanager || grep -Rql "Packwiz" /app/resources/scripts'
docker compose -f "$COMPOSE" exec -T panel sh -lc 'find /app/app -name "Packwiz*Controller.php" -exec php -l {} \; | grep -q "No syntax errors"'
docker compose -f "$COMPOSE" exec -T panel php artisan migrate:status >/dev/null
curl -fsS http://127.0.0.1:8088 >/dev/null
