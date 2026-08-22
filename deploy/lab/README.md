# P0-W4 isolated network lab

This disposable lab validates the first edge-routing boundary on the local
Docker Linux engine. It has no published ports and uses three dedicated
Docker bridges with masquerading disabled. Every lab service removes its
default route at startup; only the explicit management and zone routes remain.

## Baseline profile

| Service | Network | Address | Purpose |
| --- | --- | --- | --- |
| `control-plane` | `management` | `172.30.0.20` | management-side peer |
| `edge-agent` | `management` | `172.30.0.10` | routed edge |
| `edge-agent` | `zone-a` | `172.30.10.1` | attacker-side gateway |
| `edge-agent` | `zone-b` | `172.30.20.1` | test-host-side gateway |
| `attacker` | `zone-a` | `172.30.10.10` | untrusted test source |
| `test-host` | `zone-b` | `172.30.20.10` | isolated HTTP target |

The edge image is pinned to a Debian 13 slim digest. The Compose project is
explicitly named `guardian-lab` so reset commands cannot target another local
Compose project accidentally.

## Run and verify

From the repository root:

```powershell
./tools/lab-reset.ps1
./tools/lab-test.ps1
```

The reset removes only the named `guardian-lab` containers, networks, and
anonymous volumes, then rebuilds the pinned image and starts the four services.
The test proves IP forwarding, both routed zone paths, absence of default
routes, and that the attacker cannot reach the management-only control-plane
address.

The equivalent Bash entry points are:

```bash
./tools/lab-reset.sh
./tools/lab-test.sh
```

## Hyper-V second profile

The Docker/WSL2 profile is the repeatable CI-like baseline. A later Hyper-V
validation uses an explicitly owned Debian 13 VM with three host-only virtual
switches: `guardian-management`, `guardian-zone-a`, and `guardian-zone-b`.
Assign the same subnets and addresses from the table above, enable IPv4
forwarding only on the edge VM, and apply the same positive and negative tests.
Do not bridge any lab adapter to a production or home LAN. Keep the VM
firewall default-deny and destroy the profile after validation.

## Safety boundary

This lab contains no production credentials, no host port publishing, no
masquerading, and no default route from a lab service. Do not add `ports`,
`network_mode: host`, a host bind to sensitive paths, an external network, or a
default route without a new reviewed change proposal.
