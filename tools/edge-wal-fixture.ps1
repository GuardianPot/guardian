[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$fixtureRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('guardian-w8-' + [guid]::NewGuid().ToString('N'))
$databasePath = Join-Path $fixtureRoot 'edge.db'
$binaryPath = Join-Path $fixtureRoot 'edge-agent-fixture.exe'

New-Item -ItemType Directory -Path $fixtureRoot -Force | Out-Null
try {
    Push-Location $repoRoot
    & go build -o $binaryPath ./apps/edge-agent/cmd/edge-agent
    if ($LASTEXITCODE -ne 0) {
        throw "W8 fixture build failed with exit code $LASTEXITCODE."
    }
    & $binaryPath --w8-fixture crash $databasePath
    $crashExitCode = $LASTEXITCODE
    if ($crashExitCode -ne 42) {
        throw "W8 crash phase exit code was $crashExitCode, expected 42."
    }

    Start-Sleep -Milliseconds 250
    & $binaryPath --w8-fixture recover $databasePath
    if ($LASTEXITCODE -ne 0) {
        throw "W8 recovery phase failed with exit code $LASTEXITCODE."
    }
    Write-Output 'W8 crash/restart fixture passed.'
}
finally {
    Pop-Location
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
