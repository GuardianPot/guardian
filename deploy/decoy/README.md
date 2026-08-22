# P0-W6 decoy runtime fixture

The lifecycle fixture exercises Docker Desktop's containerd-managed `runc`
runtime as the local OCI validation path. It is a disposable runtime test, not
an attacker-facing decoy implementation.

The fixture uses the exact Debian 13 slim digest already approved for the lab,
no network, a read-only root filesystem, a small writable `/tmp`, bounded
memory/CPU/PIDs, all Linux capabilities dropped, and `no-new-privileges`.
There are no bind mounts and no containerd or Docker socket mounts.

Run from the repository root:

```powershell
./tools/decoy-lifecycle.ps1
```

The Bash runner supports both a native `docker` CLI and Docker Desktop's
`docker.exe` bridge from WSL:

```bash
./tools/decoy-lifecycle.sh
```

The runner pulls the digest, executes two create/start/inspect/kill/remove
cycles, verifies the runtime and resource policy, checks socket/read-only/
tmpfs isolation from inside the decoy, injects a kill failure, and proves that
cleanup leaves no fixture container behind.
