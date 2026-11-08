$ErrorActionPreference = 'Stop'
$server = 'ubuntu@120.53.18.44'
$repo = 'https://github.com/Padfoot1277/WebApp_PokerGame_Shengji.git'
$stage = Join-Path $env:TEMP ('shengji-deploy-' + [guid]::NewGuid().ToString('N'))
function Run([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed (exit $LASTEXITCODE)." }
}
Write-Host 'This command deploys GitHub HEAD and permanently clears ALL rooms, sessions and game history.'
Run 'ssh' @('-o','BatchMode=yes',$server,'sudo -n true')
Run 'git' @('clone','--depth','1',$repo,$stage)
Push-Location $stage
try {
    Run 'git' @('log','-1','--oneline')
    Push-Location 'frontend'
    try { Run 'npm.cmd' @('ci'); Run 'npm.cmd' @('run','build') } finally { Pop-Location }
    $previousOS = $env:GOOS; $previousArch = $env:GOARCH; $previousCGO = $env:CGO_ENABLED
    try {
        $env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
        Run 'go' @('build','-o','shengji-server','./cmd/server')
    } finally { $env:GOOS=$previousOS; $env:GOARCH=$previousArch; $env:CGO_ENABLED=$previousCGO }
    Run 'tar' @('-czf','deploy.tar.gz','shengji-server','-C','frontend','dist')
    Run 'scp' @('deploy.tar.gz',($server + ':/home/ubuntu/shengji-deploy.tar.gz'))
    $remote = @'
set -eu
base=/home/ubuntu/shengji
stage=$(mktemp -d /home/ubuntu/shengji-stage.XXXXXX)
trap 'rm -rf "$stage"' EXIT
tar -xzf /home/ubuntu/shengji-deploy.tar.gz -C "$stage"
test -s "$stage/shengji-server"
test -s "$stage/dist/index.html"
test "$(readlink -f "$base/data")" = /home/ubuntu/shengji/data
test ! -L "$base/data"
sudo -n systemctl stop shengji
trap 'sudo -n systemctl start shengji; rm -rf "$stage"' EXIT
install -m 755 "$stage/shengji-server" "$base/server.new"
mv "$base/server.new" "$base/server"
mkdir -p "$base/frontend/dist"
test "$(readlink -f "$base/frontend/dist")" = /home/ubuntu/shengji/frontend/dist
find /home/ubuntu/shengji/frontend/dist -mindepth 1 -delete
cp -a "$stage/dist/." "$base/frontend/dist/"
find /home/ubuntu/shengji/data -mindepth 1 -delete
sudo -n systemctl start shengji
sleep 2
systemctl is-active shengji
curl --fail --silent --show-error http://127.0.0.1:8080/healthz
rm -f /home/ubuntu/shengji-deploy.tar.gz
'@
    $remoteFile = Join-Path $stage 'deploy.sh'
    [IO.File]::WriteAllText($remoteFile, $remote.Replace("`r`n", "`n"), [Text.UTF8Encoding]::new($false))
    Run 'scp' @($remoteFile,($server + ':/home/ubuntu/shengji-deploy.sh'))
    Run 'ssh' @('-o','BatchMode=yes',$server,'bash /home/ubuntu/shengji-deploy.sh')
    Write-Host 'Deployment and reset complete; server internal health check passed.'
    try {
        $health = Invoke-RestMethod 'http://120.53.18.44:8080/healthz' -TimeoutSec 15
        if ($health.status -ne 'ok') { throw 'Unexpected health response.' }
        Write-Host 'Public health check passed. Open http://120.53.18.44:8080'
    } catch { Write-Warning ('Deployment succeeded, but public access could not be verified: ' + $_.Exception.Message) }
} finally { Pop-Location }
