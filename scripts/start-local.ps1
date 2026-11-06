$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$lanIp = Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -and $_.NetAdapter.Status -eq 'Up' } | ForEach-Object { $_.IPv4Address.IPAddress } | Where-Object { $_ -notmatch '^(127\.|169\.254\.)' } | Select-Object -First 1
if (-not $lanIp) { throw 'Connect this computer to Wi-Fi first.' }
Write-Host "Game server: ws://${lanIp}:8080/ws" -ForegroundColor Green
Write-Host 'Keep this window open while playing. Press Ctrl+C to stop.'
Set-Location -LiteralPath $projectDir
& go run ./cmd/server
