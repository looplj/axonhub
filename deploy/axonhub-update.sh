#!/bin/bash
# AxonHub one-shot seamless updater for Baota managed Docker deployments.
#
# Usage:
#   bash axonhub-update.sh              # update to latest
#   bash axonhub-update.sh dev-1bc55dd  # pin a specific image tag
#
# Env:
#   AXONHUB_DIR    deployment directory (default: auto-detect /www/wwwroot/axonhub)
#   IMAGE          full image reference (default: ghcr.io/shifuf/axonhub:latest)
#   PULL_IMAGES    set to 1 to run "docker compose pull" first (default: skip,
#                  avoids ghcr.io timeouts in mainland networks)
#   SKIP_BACKUP    set to 1 to skip the pre-update backup (not recommended)

set -uo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()  { echo -e "${BLUE}[INFO]${NC} $1" >&2; }
ok()    { echo -e "${GREEN}[OK]${NC} $1" >&2; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1" >&2; }
fail()  { echo -e "${RED}[FAIL]${NC} $1" >&2; exit 1; }

DEFAULT_IMAGE="ghcr.io/shifuf/axonhub:latest"
AXONHUB_DIR="${AXONHUB_DIR:-}"
IMAGE="${IMAGE:-}"
VERSION_ARG="${1:-}"
PULL_IMAGES="${PULL_IMAGES:-0}"
SKIP_BACKUP="${SKIP_BACKUP:-0}"

# ---------------------------------------------------------------- locate deploy dir
find_deploy_dir() {
  local candidates=(
    "${AXONHUB_DIR}"
    "/www/wwwroot/axonhub"
    "/opt/axonhub"
    "/root/axonhub"
  )
  for dir in "${candidates[@]}"; do
    [ -z "$dir" ] && continue
    [ ! -d "$dir" ] && continue
    if [ -f "$dir/docker-compose.yml" ] || [ -f "$dir/compose.yaml" ]; then
      echo "$dir"; return 0
    fi
  done
  local hit
  hit=$(grep -rls --include='docker-compose.yml' --include='compose.yaml' 'axonhub' \
        /www/wwwroot /opt /root /www/docker 2>/dev/null | head -n1 || true)
  if [ -n "$hit" ]; then dirname "$hit"; return 0; fi
  return 1
}

if [ -z "$AXONHUB_DIR" ]; then
  AXONHUB_DIR="$(find_deploy_dir)" || fail "deploy dir not found; use AXONHUB_DIR=/path bash $0"
fi
cd "$AXONHUB_DIR" || fail "cannot enter $AXONHUB_DIR"

COMPOSE_FILE="docker-compose.yml"
[ -f "$COMPOSE_FILE" ] || COMPOSE_FILE="compose.yaml"
[ -f "$COMPOSE_FILE" ] || fail "no docker-compose.yml in $AXONHUB_DIR"

command -v docker >/dev/null 2>&1 || fail "docker not found"
if docker compose version >/dev/null 2>&1; then
  dc() { docker compose -f "$COMPOSE_FILE" "$@"; }
else
  command -v docker-compose >/dev/null 2>&1 || fail "docker compose plugin not found"
  dc() { docker-compose -f "$COMPOSE_FILE" "$@"; }
fi
info "deploy dir: $AXONHUB_DIR"

# ---------------------------------------------------------------- resolve target image
# This updater always targets the FORK image (shifuf/axonhub): deployments
# that still run the upstream image get switched over on the first run.
# CURRENT_IMAGE (from the old .env) is only kept for rollback.
if [ -f ".env" ] && grep -q '^AXONHUB_IMAGE=' .env; then
  CURRENT_IMAGE="$(grep '^AXONHUB_IMAGE=' .env | head -n1 | cut -d= -f2-)"
else
  CURRENT_IMAGE="$DEFAULT_IMAGE"
fi

if [ -n "$IMAGE" ]; then
  NEW_IMAGE="$IMAGE"
elif [ -n "$VERSION_ARG" ]; then
  case "$VERSION_ARG" in
    dev-*) NEW_IMAGE="${DEFAULT_IMAGE%%:*}:${VERSION_ARG}" ;;
    *)     NEW_IMAGE="${DEFAULT_IMAGE%%:*}:${VERSION_ARG#v}" ;;
  esac
else
  NEW_IMAGE="$DEFAULT_IMAGE"
fi
info "target image: $NEW_IMAGE"

# ---------------------------------------------------------------- backup
backup_stamp="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="$AXONHUB_DIR/backups"
mkdir -p "$BACKUP_DIR"

if [ "$SKIP_BACKUP" != "1" ]; then
  info "online backup..."
  if dc ps --quiet postgres 2>/dev/null | grep -q .; then
    # docker exec lands as root; local peer auth would prompt for a password
    # and hang forever in -T mode. Pass PGPASSWORD explicitly and hard-timeout
    # the dump so a stuck auth can never freeze the whole update.
    if timeout 180 dc exec -T postgres sh -lc 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' \
      > "$BACKUP_DIR/axonhub-pg-$backup_stamp.sql"; then
      gzip -f "$BACKUP_DIR/axonhub-pg-$backup_stamp.sql" \
        && ok "Postgres backup: axonhub-pg-$backup_stamp.sql.gz"
    else
      rm -f "$BACKUP_DIR/axonhub-pg-$backup_stamp.sql"
      warn "pg_dump failed/timed out; continuing WITHOUT a database backup"
      warn "press Ctrl+C now to abort and back up manually (SKIP_BACKUP=1 to skip this check)"
      sleep 5
    fi
  fi
  shopt -s nullglob
  for db in "$AXONHUB_DIR"/data/*.db "$AXONHUB_DIR"/*.db; do
    cp -f "$db" "$BACKUP_DIR/$(basename "$db" .db)-$backup_stamp.db"
    ok "SQLite backup: $(basename "$db" .db)-$backup_stamp.db"
  done
  shopt -u nullglob
else
  warn "backup skipped (SKIP_BACKUP=1)"
fi

# ---------------------------------------------------------------- write image tag
cp -f "$COMPOSE_FILE" "${COMPOSE_FILE}.bak.$backup_stamp" 2>/dev/null || true
[ -f .env ] && cp -f .env ".env.bak.$backup_stamp"

if grep -q '^AXONHUB_IMAGE=' .env 2>/dev/null; then
  sed -i "s|^AXONHUB_IMAGE=.*|AXONHUB_IMAGE=$NEW_IMAGE|" .env
else
  echo "AXONHUB_IMAGE=$NEW_IMAGE" >> .env
fi
ok ".env -> $NEW_IMAGE"

# ---------------------------------------------------------------- recreate container
info "recreating container..."
if [ "$PULL_IMAGES" = "1" ]; then
  dc pull --ignore-pull-failures || warn "image pull failed (common in CN), continuing with local image"
fi
dc up -d --force-recreate --no-deps axonhub || fail "container recreate failed"

# ---------------------------------------------------------------- health check + rollback
PORT="$(grep -oP 'AXONHUB_HOST_PORT=\K[0-9]+' .env 2>/dev/null || echo 8090)"
health_url="http://127.0.0.1:${PORT}/health"
info "health check $health_url ..."
healthy=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 "$health_url" >/dev/null 2>&1; then healthy=1; break; fi
  sleep 2
done

if [ "$healthy" != "1" ]; then
  dc logs --tail 30 axonhub >&2 || true
  warn "health check failed, rolling back..."
  OLD_IMAGE="$(grep '^AXONHUB_IMAGE=' ".env.bak.$backup_stamp" 2>/dev/null | head -n1 | cut -d= -f2-)"
  if [ -n "$OLD_IMAGE" ] && [ "$OLD_IMAGE" != "$NEW_IMAGE" ]; then
    sed -i "s|^AXONHUB_IMAGE=.*|AXONHUB_IMAGE=$OLD_IMAGE|" .env
    dc up -d --force-recreate --no-deps axonhub || true
    for _ in $(seq 1 15); do
      if curl -fsS --max-time 5 "$health_url" >/dev/null 2>&1; then
        warn "rolled back to $OLD_IMAGE; check logs and retry"
        exit 1
      fi
      sleep 2
    done
  fi
  fail "update failed and could not recover; backups in $BACKUP_DIR / .env.bak.$backup_stamp"
fi

ok "update complete: $NEW_IMAGE"
echo
echo "verify:"
echo "  curl $health_url"
echo "  docker compose logs -f axonhub"
echo "  backups: $BACKUP_DIR"