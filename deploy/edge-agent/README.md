# Guardian Edge Agent service profile

This directory is the Debian 13 reference service profile for P1-W7 and P1-W8.
It is not a complete production packaging matrix.

## Identity and privilege boundary

- `guardian-edge.sysusers` creates a stable `guardian-edge` user/group with no
  login shell and `/nonexistent` as its declared home.
- The main daemon has an empty capability bounding set. It does not run as root,
  open the container runtime socket, alter networking, or execute shell strings.
- `guardian-edge-privd.service` runs the narrow P1-W8 helper as root with the
  `guardian-edge` primary group. It listens only at
  `/run/guardian-edge-privd/guardian-edge-privd.sock`, owned
  `root:guardian-edge` with mode `0660` under a `0750` root-owned directory.
- The helper validates kernel `SO_PEERCRED` PID/UID/GID, pins the PID with
  `pidfd_open`, and checks all `/proc/<pid>/status` credential slots before any
  RPC dispatch. The Edge unit only *wants* the helper; helper loss degrades the
  Edge health condition without terminating the main daemon.
- P1-W8 ships an unsupported production adapter and an empty capability
  bounding set. Typed address, nftables, container, and namespace adapters are
  activated only by later owner-reviewed packages.

## Reference installation

1. Install the statically linked daemon as `/usr/bin/guardian-edge` and helper
   as `/usr/libexec/guardian-edge/guardian-edge-privd`, both owned `root:root`
   with mode `0755`.
2. Install `guardian-edge.sysusers` under `/usr/lib/sysusers.d/` and run
   `systemd-sysusers`.
3. Install `guardian-edge.tmpfiles` under `/usr/lib/tmpfiles.d/` and run
   `systemd-tmpfiles --create`.
4. Copy `config.example.json` to `/etc/guardian-edge/config.json`, set owner
   `root:guardian-edge`, and mode `0640`.
5. Provision the device certificate at mode `0644` or stricter and its matching
   private key as `guardian-edge:guardian-edge` mode `0600`.
6. Install `guardian-edge-privd.tmpfiles` and
   `guardian-edge-privd.service` beside the existing Edge profiles.
7. Run `systemctl daemon-reload`; validate both units with
   `systemd-analyze verify guardian-edge-privd.service guardian-edge.service`.
8. Enable both units. The main daemon remains alive and reports a precise
   degraded reason if the helper is stopped or unavailable.

The configuration accepts no enrollment token or private-key body. A missing,
expired, mismatched, symlinked, or broadly readable key makes startup fail. No
plaintext or unauthenticated channel fallback exists.

See `docs/runbooks/edge-agent/development.md` for diagnostics and explicit
development database recovery. See
`docs/runbooks/privileged-helper/development.md` for P1-W8 operation and abuse
evidence.
