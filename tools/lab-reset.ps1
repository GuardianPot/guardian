$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'deploy/lab/compose.yaml'
$composeArgs = @('-f', $composeFile, '-p', 'guardian-lab')

Push-Location $repoRoot
try {
    & docker compose @composeArgs down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { throw 'docker compose down failed' }

    & docker compose @composeArgs build --pull
    if ($LASTEXITCODE -ne 0) { throw 'docker compose build failed' }

    & docker compose @composeArgs up --detach
    if ($LASTEXITCODE -ne 0) { throw 'docker compose up failed' }
} finally {
    Pop-Location
}
