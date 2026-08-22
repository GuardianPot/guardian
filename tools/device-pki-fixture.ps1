[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Push-Location $repoRoot
try {
    & go test ./apps/control-plane/internal/devicepki -run TestDevicePKI -count=1 -v
    if ($LASTEXITCODE -ne 0) {
        throw "W9 device PKI fixture failed with exit code $LASTEXITCODE."
    }
    Write-Output 'W9 device PKI fixture passed.'
}
finally {
    Pop-Location
}
