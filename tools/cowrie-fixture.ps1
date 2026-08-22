$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$image = 'cowrie/cowrie:3.0.12@sha256:3e4ce75576e4dffc3397ae3ad8dbb00afa00fe826b1531fea50d4fd9728326e1'
$imageRevision = 'ced855a5cda953eb4ad439d8ee8060afe4234fe4'
$name = 'guardian-p0-w7-cowrie-fixture'
$network = 'guardian-p0-w7-cowrie-network'
$clientPath = Join-Path $env:TEMP 'guardian-p0-w7-cowrie-client'
$normalizer = Join-Path $PSScriptRoot 'check-cowrie-events.mjs'
$fixturePassword = 'letmein'
$hostileMarker = '__GUARDIAN_HOSTILE__'

function Remove-Fixture {
    try { & docker rm --force --volumes $name 2>&1 | Out-Null } catch { }
    try { & docker network rm $network 2>&1 | Out-Null } catch { }
    Remove-Item -Force $clientPath -ErrorAction SilentlyContinue
}

function Get-FixtureInspect {
    $json = & docker inspect $name
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture inspect failed' }
    return (($json -join "`n") | ConvertFrom-Json)[0]
}

function Build-Client {
    Push-Location (Join-Path $PSScriptRoot 'cowrie-client')
    $oldGoWork = $env:GOWORK
    $oldGoOs = $env:GOOS
    $oldGoArch = $env:GOARCH
    try {
        $env:GOWORK = 'off'
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        & go mod download
        if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture client dependency download failed' }
        & go build -o $clientPath .
        if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture client build failed' }
    } finally {
        if ($null -eq $oldGoWork) { Remove-Item Env:GOWORK -ErrorAction SilentlyContinue } else { $env:GOWORK = $oldGoWork }
        if ($null -eq $oldGoOs) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $oldGoOs }
        if ($null -eq $oldGoArch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $oldGoArch }
        Pop-Location
    }
}

function Start-Fixture {
    & docker network create --internal --label guardian.work-package=P0-W7 $network | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture network creation failed' }

    $createArgs = @(
        'create',
        '--name', $name,
        '--network', $network,
        '--read-only',
        '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=16m,uid=999,gid=999',
        '--tmpfs', '/cowrie/cowrie-git/var/log/cowrie:rw,nosuid,nodev,size=32m,uid=999,gid=999',
        '--tmpfs', '/cowrie/cowrie-git/var/lib/cowrie:rw,nosuid,nodev,size=32m,uid=999,gid=999',
        '--tmpfs', '/cowrie/cowrie-git/var/lib/cowrie/tty:rw,nosuid,nodev,size=32m,uid=999,gid=999',
        '--tmpfs', '/cowrie/cowrie-git/var/run:rw,nosuid,nodev,size=8m,uid=999,gid=999',
        '--memory', '256m',
        '--memory-swap', '256m',
        '--cpus', '0.50',
        '--pids-limit', '128',
        '--cap-drop', 'ALL',
        '--security-opt', 'no-new-privileges:true',
        '--label', 'guardian.work-package=P0-W7',
        '--label', 'guardian.decoy.socket-mount=false',
        '--label', "guardian.cowrie.upstream=v3.0.12@$imageRevision",
        $image
    )

    & docker @createArgs | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture create failed' }
    & docker start $name | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture start failed' }

    for ($attempt = 0; $attempt -lt 30; $attempt += 1) {
        $logs = (& docker logs $name 2>&1 | Out-String)
        if ($logs.Contains('Ready to accept SSH connections')) { return }
        Start-Sleep -Milliseconds 500
    }
    throw 'Cowrie fixture did not become SSH-ready'
}

function Assert-Security {
    $inspect = Get-FixtureInspect
    if ($inspect.HostConfig.Privileged) { throw 'Cowrie fixture is privileged' }
    if (-not $inspect.HostConfig.ReadonlyRootfs) { throw 'Cowrie fixture rootfs is writable' }
    if ([int64]$inspect.HostConfig.Memory -ne 268435456) { throw 'Cowrie memory limit mismatch' }
    if ([int64]$inspect.HostConfig.NanoCpus -ne 500000000) { throw 'Cowrie CPU limit mismatch' }
    if ([int64]$inspect.HostConfig.PidsLimit -ne 128) { throw 'Cowrie PID limit mismatch' }
    if (@($inspect.HostConfig.CapDrop) -notcontains 'ALL') { throw 'Cowrie capabilities were not dropped' }
    if (@($inspect.HostConfig.SecurityOpt) -notcontains 'no-new-privileges:true') { throw 'Cowrie no-new-privileges is missing' }
    if ($inspect.HostConfig.NetworkMode -ne $network) { throw 'Cowrie fixture network mismatch' }
    if ($null -ne $inspect.HostConfig.Binds -and @($inspect.HostConfig.Binds).Count -ne 0) { throw 'Cowrie fixture has bind mounts' }
    if (@($inspect.Mounts | Where-Object Type -eq 'bind').Count -ne 0) { throw 'Cowrie fixture has an implicit bind mount' }

    $networkInfo = ((& docker network inspect $network | ConvertFrom-Json)[0])
    if (-not $networkInfo.Internal) { throw 'Cowrie fixture network is not internal' }

    $imageInfo = ((& docker image inspect $image | ConvertFrom-Json)[0])
    $revision = $imageInfo.Config.Labels.PSObject.Properties['org.opencontainers.image.revision'].Value
    if ($revision -ne $imageRevision) { throw "Cowrie image revision mismatch: $revision" }
}

function Get-RawEvents {
    $events = & docker exec $name /cowrie/cowrie-env/bin/python3 -c "from pathlib import Path; print(Path('var/log/cowrie/cowrie.json').read_text())"
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie JSON event extraction failed' }
    return ($events -join "`n")
}

try {
    Remove-Fixture
    & docker pull $image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie digest-pinned image pull failed' }
    Build-Client
    Start-Fixture
    Assert-Security

    & docker cp $clientPath "$name`:/cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client"
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie fixture client copy failed' }

    $oldErrorAction = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & docker exec $name /cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client -user root -password root -command 'id' 2>$null
    $badPasswordExitCode = $LASTEXITCODE
    $ErrorActionPreference = $oldErrorAction
    if ($badPasswordExitCode -eq 0) { throw 'Cowrie accepted an invalid fixture password' }

    $command = "id; uname -a; printf `"<script>alert(1)</script>$hostileMarker`""
    & docker exec $name /cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client -user root -password $fixturePassword -command $command | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cowrie successful SSH session fixture failed' }

    $rawEvents = Get-RawEvents
    $normalizerInput = "$rawEvents`n{malformed-cowrie-event"
    $warningPath = Join-Path $env:TEMP 'guardian-p0-w7-cowrie-warning.txt'
    $oldErrorAction = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $normalized = ($normalizerInput | & node $normalizer --validate --forbidden $fixturePassword --hostile $hostileMarker 2> $warningPath)
    $normalizerExitCode = $LASTEXITCODE
    $ErrorActionPreference = $oldErrorAction
    if ($normalizerExitCode -ne 0) { throw 'Cowrie canonical event validation failed' }
    if (-not (($normalized -join "`n").Contains('guardian.telemetry.v1'))) { throw 'canonical telemetry version missing' }
    if (-not ((Get-Content $warningPath -Raw).Contains('malformed line(s) tolerated'))) { throw 'malformed-event resilience evidence missing' }

    $oldErrorAction = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & docker exec $name /cowrie/cowrie-env/bin/python3 -c "import socket; socket.create_connection(('1.1.1.1', 443), 1); raise SystemExit('egress unexpectedly allowed')" 2>$null
    $egressExitCode = $LASTEXITCODE
    $ErrorActionPreference = $oldErrorAction
    if ($egressExitCode -eq 0) { throw 'Cowrie egress probe unexpectedly succeeded' }

    Write-Host 'P0-W7 Cowrie adapter passed: pinned provenance, SSH auth/session/command evidence, hostile-input boundary, malformed-event tolerance, internal network, and egress denial.'
} finally {
    Remove-Fixture
}
