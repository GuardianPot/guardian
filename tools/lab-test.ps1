$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'deploy/lab/compose.yaml'
$composeArgs = @('-f', $composeFile, '-p', 'guardian-lab')

function Invoke-LabExec([string]$service, [string]$command) {
    & docker compose @composeArgs exec -T $service sh -ec $command
    if ($LASTEXITCODE -ne 0) {
        throw "lab command failed in $service`: $command"
    }
}

Push-Location $repoRoot
try {
    $forwardingOutput = & docker compose @composeArgs exec -T edge-agent sh -ec 'cat /proc/sys/net/ipv4/ip_forward'
    if ($LASTEXITCODE -ne 0) {
        throw 'edge-agent is not running'
    }
    $forwarding = ($forwardingOutput -join "`n").Trim()
    if ($forwarding -ne '1') {
        throw "edge-agent IPv4 forwarding is not enabled: $forwarding"
    }

    Invoke-LabExec 'edge-agent' 'if ip route show default | grep -q .; then exit 1; fi; ip route show; ping -c 1 -W 1 172.30.0.20 >/dev/null'
    Invoke-LabExec 'attacker' 'if ip route show default | grep -q .; then exit 1; fi; ip route show; ping -c 1 -W 1 172.30.20.10 >/dev/null; curl --fail --silent http://172.30.20.10:8080/ | grep -F guardian-lab-test-host >/dev/null'

    & docker compose @composeArgs exec -T attacker sh -ec 'ping -c 1 -W 1 172.30.0.20 >/dev/null 2>&1' | Out-Null
    if ($LASTEXITCODE -eq 0) {
        throw 'attacker unexpectedly reached the management-only control-plane address'
    }

    Write-Host 'P0-W4 lab test passed: forwarding, routed access, HTTP reachability, and management isolation.'
} finally {
    Pop-Location
}
