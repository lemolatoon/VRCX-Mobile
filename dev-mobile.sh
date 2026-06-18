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

IS_WSL=0
if [ -r /proc/version ] && grep -qiE 'Microsoft|WSL' /proc/version; then
    IS_WSL=1
fi

# ── Agent tracking state (set by setup_windows_agent_dev) ───────────────────
AGENT_STATUS=""        # "running" | "failed" | "" (not applicable / not WSL)
AGENT_FAIL_REASON=""
AGENT_SERVER_URL=""
AGENT_EXE_WIN=""

agent_warn() {
    AGENT_FAIL_REASON="$1"
    AGENT_STATUS="failed"
}

ps_quote() {
    local value="${1//\'/\'\'}"
    printf "'%s'" "$value"
}

windows_healthcheck() {
    local url="$1"
    timeout 8s powershell.exe -NoProfile -Command "\$ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -TimeoutSec 3 -Uri $(ps_quote "$url") | Out-Null" >/dev/null 2>&1
}

detect_windows_server_url() {
    local candidates=("http://localhost:8080")
    local wsl_ip
    wsl_ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i == "src") {print $(i+1); exit}}')"
    if [ -z "$wsl_ip" ]; then
        wsl_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    fi
    if [ -n "$wsl_ip" ]; then
        candidates+=("http://${wsl_ip}:8080")
    fi

    local url
    local attempt
    for attempt in $(seq 1 3); do
        for url in "${candidates[@]}"; do
            if windows_healthcheck "$url"; then
                printf '%s\n' "$url"
                return 0
            fi
        done
        sleep 1
    done
    return 1
}

port_in_use() {
    local port="$1"
    ss -ltn "sport = :${port}" 2>/dev/null | awk 'NR > 1 { found=1 } END { exit found ? 0 : 1 }'
}

find_mobile_port() {
    local port
    for port in $(seq 5174 5199); do
        if ! port_in_use "$port"; then
            printf '%s\n' "$port"
            return 0
        fi
    done
    return 1
}

setup_windows_agent_dev() {
    local user_id="$1"
    local db_url="$2"

    if [ "$IS_WSL" -ne 1 ]; then
        echo "Windows Agent dev setup skipped: not running under WSL."
        return 0
    fi

    if ! command -v cmd.exe >/dev/null 2>&1; then
        echo "Windows Agent dev setup: cmd.exe is not available."
        echo "Manual setup: build vrcx-server/dist/vrcx-log-agent.exe, create a token, then run setup and run from Windows."
        agent_warn "cmd.exe not on PATH"
        return 0
    fi
    if ! command -v powershell.exe >/dev/null 2>&1; then
        echo "Windows Agent dev setup: powershell.exe is not available."
        echo "Manual setup: build vrcx-server/dist/vrcx-log-agent.exe, create a token, then run setup and run from Windows."
        agent_warn "powershell.exe not on PATH"
        return 0
    fi
    if ! command -v wslpath >/dev/null 2>&1; then
        echo "Windows Agent dev setup: wslpath is not available."
        echo "Manual setup: build vrcx-server/dist/vrcx-log-agent.exe, create a token, then run setup and run from Windows."
        agent_warn "wslpath not on PATH"
        return 0
    fi

    echo "Preparing Windows Agent dev setup..."
    echo "Checking whether Windows can reach the backend..."
    local server_url
    if ! server_url="$(detect_windows_server_url)"; then
        echo "Windows Agent dev setup: Windows cannot reach http://localhost:8080 or the WSL IP fallback."
        echo "  → Is the proxy up? (curl http://localhost:8080/healthz)"
        echo "Manual setup: run vrcx-log-agent.exe setup --server \"<url>\" --token \"<token>\", then vrcx-log-agent.exe run."
        agent_warn "Windows cannot reach the backend — is docker compose running and port 8080 forwarded?"
        return 0
    fi

    echo "Building Windows log agent..."
    if ! vrcx-server/scripts/build-log-agent.sh; then
        echo "Windows Agent dev setup: failed to build vrcx-log-agent.exe."
        echo "Manual setup: build vrcx-server/dist/vrcx-log-agent.exe, then run setup and run from Windows."
        agent_warn "failed to build vrcx-log-agent.exe (check Go toolchain)"
        return 0
    fi

    local localappdata_win
    localappdata_win="$(timeout 8s cmd.exe /c echo %LOCALAPPDATA% 2>/dev/null | tr -d '\r' | tail -n 1)"
    if [ -z "$localappdata_win" ]; then
        echo "Windows Agent dev setup: could not read %LOCALAPPDATA% from Windows."
        echo "Manual setup: copy vrcx-server/dist/vrcx-log-agent.exe to %LOCALAPPDATA%\\VRCX-Mobile\\dev-agent, then run setup and run."
        agent_warn "could not read %LOCALAPPDATA% from Windows"
        return 0
    fi

    local localappdata_wsl
    if ! localappdata_wsl="$(wslpath -u "$localappdata_win" 2>/dev/null)"; then
        echo "Windows Agent dev setup: could not convert %LOCALAPPDATA% to a WSL path."
        echo "Manual setup: copy vrcx-server/dist/vrcx-log-agent.exe to %LOCALAPPDATA%\\VRCX-Mobile\\dev-agent, then run setup and run."
        agent_warn "wslpath -u failed for %LOCALAPPDATA%"
        return 0
    fi

    local agent_dir_wsl="$localappdata_wsl/VRCX-Mobile/dev-agent"
    if ! mkdir -p "$agent_dir_wsl"; then
        agent_warn "failed to create agent directory $agent_dir_wsl"
        return 0
    fi
    if ! cp -f vrcx-server/dist/vrcx-log-agent.exe "$agent_dir_wsl/vrcx-log-agent.exe"; then
        agent_warn "failed to copy vrcx-log-agent.exe to $agent_dir_wsl"
        return 0
    fi

    echo "Creating Windows Dev Agent token..."
    local token
    if ! token="$(cd vrcx-server && DATABASE_URL="$db_url" go run ./cmd/admin agent-token create "$user_id" "Windows Dev Agent")"; then
        echo "Windows Agent dev setup: failed to create agent token."
        agent_warn "failed to create agent token (check DB connection)"
        return 0
    fi
    token="$(printf '%s\n' "$token" | tail -n 1 | tr -d '\r')"

    local agent_dir_win agent_exe_win
    if ! agent_dir_win="$(wslpath -w "$agent_dir_wsl" 2>/dev/null)"; then
        agent_warn "wslpath -w failed for $agent_dir_wsl"
        return 0
    fi
    if ! agent_exe_win="$(wslpath -w "$agent_dir_wsl/vrcx-log-agent.exe" 2>/dev/null)"; then
        agent_warn "wslpath -w failed for vrcx-log-agent.exe path"
        return 0
    fi

    local ps_script
    ps_script="\$ErrorActionPreference='Stop'; "
    ps_script+="Set-Location -LiteralPath $(ps_quote "$agent_dir_win"); "
    ps_script+="\$target = $(ps_quote "$agent_exe_win"); "
    ps_script+="Get-CimInstance Win32_Process | Where-Object { \$_.ExecutablePath -eq \$target -and \$_.CommandLine -like '* run*' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force }; "
    ps_script+="& \$target setup --server $(ps_quote "$server_url") --token $(ps_quote "$token"); "
    ps_script+="Start-Process -FilePath \$target -ArgumentList 'run' -WorkingDirectory $(ps_quote "$agent_dir_win")"

    echo "Starting Windows log agent dev process..."
    if ! timeout 60s powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "$ps_script"; then
        echo "Windows Agent dev setup: setup or Start-Process failed."
        echo "Manual setup:"
        echo "  cd \"$agent_dir_win\""
        echo "  .\\vrcx-log-agent.exe setup --server \"$server_url\" --token \"<agent-token>\""
        echo "  .\\vrcx-log-agent.exe run"
        agent_warn "PowerShell setup/Start-Process failed (check server URL and token)"
        return 0
    fi

    # Verify the agent process is alive after launch (Start-Process returns immediately
    # even if the process exits right away, so we poll for up to ~3 s).
    echo "Verifying Windows log agent process..."
    local verify_script verify_result
    verify_script="\$t=$(ps_quote "$agent_exe_win"); "
    verify_script+="\$ok=\$false; "
    verify_script+="for(\$i=0;\$i-lt5;\$i++){ "
    verify_script+="Start-Sleep -Milliseconds 600; "
    verify_script+="if(Get-CimInstance Win32_Process | Where-Object{ \$_.ExecutablePath-eq\$t -and \$_.CommandLine-like '* run*' }){ \$ok=\$true; break } "
    verify_script+="}; "
    verify_script+="if(\$ok){ 'RUNNING' }else{ 'NOT_RUNNING' }"
    verify_result="$(timeout 15s powershell.exe -NoProfile -Command "$verify_script" 2>/dev/null | tr -d '\r' | grep -E '^(RUNNING|NOT_RUNNING)$' | tail -1)"
    if [ "$verify_result" != "RUNNING" ]; then
        echo "Windows Agent dev setup: agent process exited immediately after launch."
        echo "  Check log: powershell.exe -Command \"Get-Content '\$env:LOCALAPPDATA\\VRCX-Mobile\\log-agent\\agent.log' -Tail 20\""
        agent_warn "agent process exited immediately after launch (run the status/log commands above to diagnose)"
        return 0
    fi

    AGENT_STATUS="running"
    AGENT_SERVER_URL="$server_url"
    AGENT_EXE_WIN="$agent_exe_win"
}

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
setup_windows_agent_dev "$USER_ID" "$DB_URL"

echo ""
echo "Backend ready:"
echo "  proxy:    http://localhost:8080"
echo "  postgres: localhost:5432"
echo "Allowlisted user: $USER_ID"
echo ""

cd mobile
[ -d node_modules ] || pnpm install
if ! MOBILE_PORT="$(find_mobile_port)"; then
    echo "No free mobile dev port found in range 5174-5199." >&2
    exit 1
fi
if [ "$MOBILE_PORT" != "5174" ]; then
    echo "Mobile dev port 5174 is already in use; using ${MOBILE_PORT} instead."
fi

echo ""
if [ "${AGENT_STATUS:-}" = "running" ]; then
    echo "GameLog agent:  ✓ RUNNING"
    echo "  server: $AGENT_SERVER_URL"
    echo "  exe:    $AGENT_EXE_WIN"
    echo "  → VRChat を起動してプレイすると GameLog が届きます (PWA login as: $USER_ID)"
    echo "  status: powershell.exe -Command \"\$env:LOCALAPPDATA + '\\VRCX-Mobile\\dev-agent\\vrcx-log-agent.exe' + ' status'\""
elif [ "${AGENT_STATUS:-}" = "failed" ]; then
    echo "⚠️  GameLog agent: NOT RUNNING — ${AGENT_FAIL_REASON}"
    echo "  Manual setup:"
    echo "    1. vrcx-server/scripts/build-log-agent.sh"
    echo "    2. Copy vrcx-server/dist/vrcx-log-agent.exe to a Windows folder"
    echo "    3. .\\vrcx-log-agent.exe setup --server \"http://localhost:8080\" --token \"<token>\""
    echo "    4. .\\vrcx-log-agent.exe run"
fi
echo ""
exec pnpm dev -- --port "$MOBILE_PORT"
