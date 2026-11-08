$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$runner = Join-Path $PSScriptRoot 'live-bots.cjs'
$output = Join-Path $root 'release\bots'
New-Item -ItemType Directory -Force $output | Out-Null
# Build before stopping the current bots; compilation failure leaves them running.
Push-Location $root
$oldOS = $env:GOOS; $oldArch = $env:GOARCH
try {
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
    & go build -o (Join-Path $output 'bot-policy.new.exe') ./scripts/bot-policy
    if ($LASTEXITCODE -ne 0) { throw 'Bot policy build failed.' }
    $socketType = & node -p 'typeof WebSocket'
    if ($LASTEXITCODE -ne 0 -or $socketType -ne 'function') { throw 'Node.js 22+ is required.' }
} finally { $env:GOOS = $oldOS; $env:GOARCH = $oldArch; Pop-Location }
$existing = Get-CimInstance Win32_Process -Filter "name='node.exe'" | Where-Object {
    $_.CommandLine -like '*shengji-live-bots.cjs*' -or $_.CommandLine -like ('*' + $runner + '*')
}
foreach ($bot in $existing) { Stop-Process -Id $bot.ProcessId -ErrorAction Stop }
Start-Sleep -Milliseconds 300
Move-Item -Force (Join-Path $output 'bot-policy.new.exe') (Join-Path $output 'bot-policy.exe')
& node $runner
exit $LASTEXITCODE
