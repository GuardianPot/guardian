# Guardian Edge Agent service profile

This directory is the Debian 13 reference service profile for P1-W7. It is not
a complete production packaging matrix.

## Identity and privilege boundary

- `guardian-edge.sysusers` creates a stable `guardian-edge` user/group with no
  login shell and `/nonexistent` as its declared home.
- The main daemon has an empty capability bounding set. It does not run as root,
  open the container runtime socket, alter networking, or execute shell strings.
- Later privileged work is limited to the typed Unix-domain-socket helper
  approved for P1-W8; this unit does not grant access to such a helper.

## Reference installation

1. Install the statically linked binary as `/usr/bin/guardian-edge`, owned by
   `root:root` and mode `0755`.
2. Install `guardian-edge.sysusers` under `/usr/lib/sysusers.d/` and run
   `systemd-sysusers`.
3. Install `guardian-edge.tmpfiles` under `/usr/lib/tmpfiles.d/` and run
   `systemd-tmpfiles --create`.
4. Copy `config.example.json` to `/etc/guardian-edge/config.json`, set owner
   `root:guardian-edge`, and mode `0640`.
5. Provision the device certificate at mode `0644` or stricter and its matching
   private key as `guardian-edge:guardian-edge` mode `0600`.
6. Install `guardian-edge.service`, run `systemctl daemon-reload`, and validate
   with `systemd-analyze verify guardian-edge.service` before enabling it.

The configuration accepts no enrollment token or private-key body. A missing,
expired, mismatched, symlinked, or broadly readable key makes startup fail. No
plaintext or unauthenticated channel fallback exists.

See `docs/runbooks/edge-agent/development.md` for diagnostics and explicit
development database recovery.
