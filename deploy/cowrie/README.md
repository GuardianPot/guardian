# P0-W7 Cowrie adapter fixture

This directory records the Phase 0 Cowrie adapter spike. It validates the
official Cowrie OCI image as a disposable medium-interaction SSH fixture; it
does not expose a product decoy or grant Cowrie arbitrary egress.

The fixture is pinned to:

```text
cowrie/cowrie:3.0.12@sha256:3e4ce75576e4dffc3397ae3ad8dbb00afa00fe826b1531fea50d4fd9728326e1
upstream revision: ced855a5cda953eb4ad439d8ee8060afe4234fe4
```

The image is run on an internal Docker network with a read-only root,
bounded tmpfs state, dropped capabilities, `no-new-privileges`, CPU/memory/PID
limits, no bind mounts, and no runtime socket. The SSH client runs inside the
same fixture container so the host does not need a published decoy port.

Run from the repository root:

```powershell
./tools/cowrie-fixture.ps1
```

```bash
./tools/cowrie-fixture.sh
```

The fixture covers failed and successful authentication, a canonical
session/command event sample, hostile command data, malformed-event tolerance,
and a default-deny egress probe. Raw Cowrie events are normalized through
`tools/check-cowrie-events.mjs`; passwords are never copied into the canonical
event shape and attacker command content remains explicitly untrusted data.

The test credential is disposable fixture data only. It must never be reused
outside this local/CI proof harness.
