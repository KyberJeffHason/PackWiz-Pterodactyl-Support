#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE="$ROOT/tests/e2e/docker-compose.yml"
trap 'docker compose -f "$COMPOSE" down -v' EXIT
bash "$ROOT/scripts/build-blueprint.sh" "$ROOT/dist"
docker compose -f "$COMPOSE" up -d --wait
for _ in {1..60};do docker compose -f "$COMPOSE" exec -T panel php artisan migrate:status >/dev/null 2>&1&&break;sleep 2;done
docker compose -f "$COMPOSE" exec -T panel php artisan migrate:status >/dev/null
docker compose -f "$COMPOSE" exec -T panel cp /blueprint_extensions/packwizmanager.blueprint /app/packwizmanager.blueprint
docker compose -f "$COMPOSE" exec -T panel blueprint -install packwizmanager
docker compose -f "$COMPOSE" exec -T panel php artisan route:list --path=api/client/extensions/packwizmanager | grep -q packwizmanager
docker compose -f "$COMPOSE" exec -T panel sh -lc 'test -d /app/resources/scripts/components/blueprint/extensions/packwizmanager || grep -Rql "Packwiz" /app/resources/scripts'
docker compose -f "$COMPOSE" exec -T panel sh -lc 'find /app/app -name "Packwiz*Controller.php" -exec php -l {} \; | grep -q "No syntax errors"'
docker compose -f "$COMPOSE" exec -T panel php artisan migrate:status >/dev/null
curl -fsS http://127.0.0.1:8088 >/dev/null
