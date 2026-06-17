#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p dist
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/vrcx-log-agent.exe ./cmd/log-agent
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum dist/vrcx-log-agent.exe
fi
