$ErrorActionPreference = 'Stop'
$server = 'ubuntu@120.53.18.44'
$repo = 'https://github.com/Padfoot1277/WebApp_PokerGame_Shengji.git'
$remoteTemplate = Join-Path $PSScriptRoot 'deploy-preserve.sh'
$stage = Join-Path $env:TEMP ('shengji-deploy-' + [guid]::NewGuid().ToString('N'))
function Run([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed (exit $LASTEXITCODE)." }
}
Write-Host 'This command deploys GitHub HEAD, retaining rooms and sessions. Players will briefly reconnect.'
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
    Run 'scp' @('deploy.tar.gz',($server + ':/home/ubuntu/shengji-preserve.tar.gz'))
    $remote = Get-Content -Raw $remoteTemplate
    $remoteFile = Join-Path $stage 'deploy.sh'
    [IO.File]::WriteAllText($remoteFile, $remote.Replace("`r`n", "`n"), [Text.UTF8Encoding]::new($false))
    Run 'scp' @($remoteFile,($server + ':/home/ubuntu/shengji-preserve.sh'))
    Run 'ssh' @('-o','BatchMode=yes',$server,'bash /home/ubuntu/shengji-preserve.sh /home/ubuntu/shengji-preserve.tar.gz')
    Write-Host 'Deployment complete; all game data retained; server internal health check passed.'
    try {
        $health = Invoke-RestMethod 'http://120.53.18.44:8080/healthz' -TimeoutSec 15
        if ($health.status -ne 'ok') { throw 'Unexpected health response.' }
        Write-Host 'Public health check passed. Open http://120.53.18.44:8080'
    } catch { Write-Warning ('Deployment succeeded, but public access could not be verified: ' + $_.Exception.Message) }
} finally { Pop-Location }
