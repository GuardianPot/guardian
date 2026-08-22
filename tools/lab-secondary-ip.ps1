$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'deploy/lab/compose.yaml'
$composeArgs = @('-f', $composeFile, '-p', 'guardian-lab')
$secondaryAddress = '172.30.20.99/32'
$secondaryIp = '172.30.20.99'

function Get-LabOutput([string]$service, [string]$command) {
    $output = & docker compose @composeArgs exec -T $service sh -ec $command
    if ($LASTEXITCODE -ne 0) {
        throw "lab command failed in $service`: $command"
    }
    return @($output)
}

function Invoke-Lab([string]$service, [string]$command) {
    $null = Get-LabOutput $service $command
}

function Get-InterfaceForRoute([string]$service, [string]$network) {
    $route = Get-LabOutput $service "ip -4 route show $network"
    $routeLine = $route | Select-Object -First 1
    $parts = $routeLine -split '\s+'
    if ($parts.Length -lt 3 -or $parts[1] -ne 'dev') {
        throw "could not determine interface for $network in $service"
    }
    return $parts[2]
}

Push-Location $repoRoot
try {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'tools/lab-reset.ps1') | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'lab reset failed' }

    $edgeZoneB = Get-InterfaceForRoute 'edge-agent' '172.30.20.0/24'
    $testHostZoneB = Get-InterfaceForRoute 'test-host' '172.30.20.0/24'

    Invoke-Lab 'edge-agent' "ip addr del $secondaryAddress dev $edgeZoneB 2>/dev/null || true"
    Invoke-Lab 'edge-agent' "arping -D -I $edgeZoneB -c 2 -w 2 $secondaryIp >/dev/null 2>&1"
    Invoke-Lab 'edge-agent' "ip addr add $secondaryAddress dev $edgeZoneB"

    $placed = Get-LabOutput 'edge-agent' "ip -o -4 addr show dev $edgeZoneB"
    $placedCount = @($placed | Select-String -SimpleMatch $secondaryAddress).Count
    if ($placedCount -ne 1) { throw "secondary address placement count was $placedCount, expected 1" }

    Invoke-Lab 'attacker' "ping -c 1 -W 1 $secondaryIp >/dev/null"

    Invoke-Lab 'edge-agent' "ip addr replace $secondaryAddress dev $edgeZoneB"
    Invoke-Lab 'edge-agent' "ip addr replace $secondaryAddress dev $edgeZoneB"
    $reconciled = Get-LabOutput 'edge-agent' "ip -o -4 addr show dev $edgeZoneB"
    $reconciledCount = @($reconciled | Select-String -SimpleMatch $secondaryAddress).Count
    if ($reconciledCount -ne 1) { throw "secondary address reconcile count was $reconciledCount, expected 1" }

    & docker compose @composeArgs exec -T test-host sh -ec "arping -D -I $testHostZoneB -c 2 -w 2 $secondaryIp >/dev/null 2>&1" | Out-Null
    if ($LASTEXITCODE -eq 0) { throw 'duplicate-address probe did not detect the placed secondary address' }

    Invoke-Lab 'edge-agent' "ip addr del $secondaryAddress dev $edgeZoneB"
    $cleaned = Get-LabOutput 'edge-agent' "ip -o -4 addr show dev $edgeZoneB"
    $cleanedCount = @($cleaned | Select-String -SimpleMatch $secondaryAddress).Count
    if ($cleanedCount -ne 0) { throw 'secondary address cleanup did not remove the address' }

    & docker compose @composeArgs exec -T test-host sh -ec "arping -D -I $testHostZoneB -c 2 -w 2 $secondaryIp >/dev/null 2>&1" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'duplicate-address probe remained active after cleanup' }

    Invoke-Lab 'edge-agent' 'if ip route show default | grep -q .; then exit 1; fi; nft list chain inet guardian_lab output | grep -F "policy drop" >/dev/null; if curl --connect-timeout 1 --silent http://198.51.100.1 >/dev/null 2>&1; then exit 1; fi'

    Write-Host 'P0-W5 secondary-IP spike passed: placement, routed reachability, reconcile, conflict detection, cleanup, and egress denial.'
} finally {
    & docker compose @composeArgs down --volumes --remove-orphans | Out-Null
}
