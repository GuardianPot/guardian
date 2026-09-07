# Decoy packs

Decoy implementations and adapters are introduced by scoped Phase 0/Phase 2
work packages. No attacker-facing product behavior is implemented here: this
directory holds no image, adapter, or runtime artifact, and `P2-W5` through
`P2-W8` build the packs themselves.

## `index.json`

`index.json` is the server-side pack index that `P2-W15` reads. A decoy record
names a pack by `(pack, version)`; the Control Plane resolves the family,
interaction level, and digest here.

The index exists so that no API field ever carries a container image reference,
command, mount, or capability. An operator cannot write to it, so a committed
change to this file is the only way a runtime detail can enter desired state.

`digest` is `null` for every entry. `P2-W4` defines what a manifest contains
and nothing exists yet to hash; a placeholder digest would assert an artifact
identity nothing has verified, which is the same failure as reporting an
unconfirmed decoy as healthy. `DC-12`'s version and digest fields are in place
and transported, and `P2-W4` fills the digest.

This file is the canonical index. The Control Plane's read model is the Go
table in `apps/control-plane/internal/deception/packindex.go`, which cannot
embed this file because `decoys/` is outside that Go module;
`TestPackIndexMatchesCanonicalFile` is what keeps the two from drifting.
