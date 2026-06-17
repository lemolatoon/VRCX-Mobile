$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o "dist\vrcx-log-agent.exe" ".\cmd\log-agent"
Get-FileHash "dist\vrcx-log-agent.exe" -Algorithm SHA256
