$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
  & npm.cmd --prefix minigame run check
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript check failed' }
  & npm.cmd --prefix minigame run build
  if ($LASTEXITCODE -ne 0) { throw 'Minigame build failed' }
  & npm.cmd --prefix minigame test
  if ($LASTEXITCODE -ne 0) { throw 'Minigame tests failed' }
  & npm.cmd --prefix frontend run build
  if ($LASTEXITCODE -ne 0) { throw 'Web build failed' }
  $releaseRoot = Join-Path $projectRoot 'release'
  $stage = Join-Path $releaseRoot ('friends-test-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
  New-Item -ItemType Directory -Path $stage -Force | Out-Null
  & go build -trimpath -o (Join-Path $stage 'shengji-server.exe') ./cmd/server
  if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
  New-Item -ItemType Directory -Path (Join-Path $stage 'frontend'),(Join-Path $stage 'minigame') -Force | Out-Null
  Copy-Item -LiteralPath 'frontend/dist' -Destination (Join-Path $stage 'frontend') -Recurse
  Copy-Item -LiteralPath 'minigame/dist','minigame/project.config.json' -Destination (Join-Path $stage 'minigame') -Recurse
  Copy-Item -LiteralPath 'minigame/TESTING.md' -Destination (Join-Path $stage 'TESTING.md')
  @'
@echo off
cd /d "%~dp0"
echo Computer IPv4 addresses:
ipconfig | findstr /c:"IPv4"
echo.
echo Keep this window open. Minigame URL: ws://COMPUTER_IP:8080/ws
set WEB_DIR=frontend/dist
set DATA_DIR=data
shengji-server.exe
pause
'@ | Set-Content -LiteralPath (Join-Path $stage 'START-SERVER.cmd') -Encoding Ascii
  $archive = Join-Path $releaseRoot 'shengji-friends-test.zip'
  $items = @(Get-ChildItem -LiteralPath $stage | ForEach-Object FullName)
  Compress-Archive -LiteralPath $items -DestinationPath $archive -Force
  Write-Host "Test package: $archive"
} finally { Pop-Location }
