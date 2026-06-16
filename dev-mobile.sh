#!/usr/bin/env bash
# Bring up the full local dev environment (postgres + vrcx-proxy + vrcx-collector
# via docker compose) and allowlist the given VRChat user ID so they can log in,
# then start the mobile frontend dev server.
set -euo pipefail
cd "$(dirname "$0")"

USER_ID="${1:-}"
if [ -z "$USER_ID" ]; then
    echo "Usage: ./dev-mobile.sh <vrchat_user_id>" >&2
    exit 1
fi

if [ ! -f .env ]; then
    # .env.example ships with CRLF line endings; strip them so docker compose's
    # own .env parser (and our `source` below) don't choke on a trailing \r.
    tr -d '\r' < .env.example > .env
    KEY=$(openssl rand -base64 32)
    sed -i.bak "s|^COOKIE_ENCRYPTION_KEY=.*|COOKIE_ENCRYPTION_KEY=${KEY}|" .env
    rm -f .env.bak
    echo "Generated .env with a new COOKIE_ENCRYPTION_KEY"
fi
set -a
# shellcheck disable=SC1090,SC1091
source <(tr -d '\r' < .env)
set +a

docker compose up -d --build

echo "Waiting for vrcx-proxy to become healthy..."
for _ in $(seq 1 60); do
    if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

DB_URL="postgres://vrcx:${POSTGRES_PASSWORD:-devpassword}@localhost:5432/vrcx_mobile"
(cd vrcx-server && DATABASE_URL="$DB_URL" go run ./cmd/admin allowlist add "$USER_ID" "dev-mobile.sh")

echo ""
echo "Backend ready:"
echo "  proxy:    http://localhost:8080"
echo "  postgres: localhost:5432"
echo "Allowlisted user: $USER_ID"
echo ""

cd mobile
[ -d node_modules ] || pnpm install
exec pnpm dev
