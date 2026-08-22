$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$image = 'debian:13-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258'
$name = 'guardian-p0-w6-decoy-fixture'
$memoryBytes = 64MB
$nanoCpus = 250000000
$pidsLimit = 64

function Remove-Fixture {
    & docker rm --force $name 2>$null | Out-Null
}

function Get-FixtureInspect {
    $json = & docker inspect $name
    if ($LASTEXITCODE -ne 0) { throw 'decoy fixture inspect failed' }
    return (($json -join "`n") | ConvertFrom-Json)[0]
}

function Invoke-DecoyCycle {
    Remove-Fixture

    $createArgs = @(
        'create',
        '--name', $name,
        '--runtime', 'runc',
        '--network', 'none',
        '--read-only',
        '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=16m',
        '--memory', '64m',
        '--memory-swap', '64m',
        '--cpus', '0.25',
        '--pids-limit', $pidsLimit,
        '--cap-drop', 'ALL',
        '--security-opt', 'no-new-privileges:true',
        '--user', '65532:65532',
        '--label', 'guardian.work-package=P0-W6',
        '--label', 'guardian.decoy.socket-mount=false',
        $image,
        'sleep', 'infinity'
    )

    & docker @createArgs | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'decoy fixture create failed' }
    & docker start $name | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'decoy fixture start failed' }

    try {
        $inspect = Get-FixtureInspect
        if ($inspect.HostConfig.Runtime -ne 'runc') { throw "runtime was $($inspect.HostConfig.Runtime)" }
        if ($inspect.HostConfig.Privileged) { throw 'decoy fixture is privileged' }
        if ([int64]$inspect.HostConfig.Memory -ne $memoryBytes) { throw 'memory limit mismatch' }
        if ([int64]$inspect.HostConfig.NanoCpus -ne $nanoCpus) { throw 'CPU limit mismatch' }
        if ([int64]$inspect.HostConfig.PidsLimit -ne $pidsLimit) { throw 'PID limit mismatch' }
        if (@($inspect.HostConfig.CapDrop) -notcontains 'ALL') { throw 'all capabilities were not dropped' }
        if (@($inspect.HostConfig.SecurityOpt) -notcontains 'no-new-privileges:true') { throw 'no-new-privileges is missing' }
        if ($inspect.HostConfig.NetworkMode -ne 'none') { throw 'decoy fixture has a network' }
        if ($null -ne $inspect.HostConfig.Binds -and @($inspect.HostConfig.Binds).Count -ne 0) { throw 'decoy fixture has bind mounts' }
        if (@($inspect.Mounts | Where-Object Type -eq 'bind').Count -ne 0) { throw 'decoy fixture has an implicit bind mount' }

        & docker exec $name sh -ec 'for socket in /run/containerd/containerd.sock /run/containerd/s/containerd.sock /var/run/docker.sock /run/docker.sock; do test ! -S "$socket"; done; if touch /etc/guardian-w6-write-test 2>/dev/null; then exit 1; fi; touch /tmp/guardian-w6-write-test; test -f /tmp/guardian-w6-write-test' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'decoy socket/read-only/tmpfs isolation test failed' }

        & docker kill $name | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'failure injection kill failed' }
        $running = (& docker inspect --format '{{.State.Running}}' $name).Trim()
        if ($LASTEXITCODE -ne 0 -or $running -ne 'false') { throw 'failure injection did not stop the decoy' }
    } finally {
        Remove-Fixture
    }

    $remaining = @(& docker ps --all --format '{{.Names}}')
    if ($remaining -contains $name) { throw 'decoy cleanup left a container behind' }
}

try {
    Remove-Fixture
    & docker pull $image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'digest-pinned decoy image pull failed' }

    $runtimeJson = (& docker info --format '{{json .Runtimes}}' | ConvertFrom-Json)
    if ($runtimeJson.PSObject.Properties.Name -notcontains 'runc') { throw 'runc runtime is not available through containerd' }

    Invoke-DecoyCycle
    Invoke-DecoyCycle
    Write-Host 'P0-W6 decoy lifecycle passed: digest pin, runc, limits, socket isolation, failure cleanup, and repeatability.'
} finally {
    Remove-Fixture
}
