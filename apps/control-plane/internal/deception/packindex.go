package deception

// PackEntry is one row of the server-side pack index.
//
// The index exists so that no API field ever carries a container image
// reference, command, mount, or capability. A create names a pack by
// (Pack, Version); everything a runtime would need is resolved here, in the
// Control Plane, from a table an operator cannot write to.
type PackEntry struct {
	Pack             string           `json:"pack"`
	Version          string           `json:"version"`
	Digest           string           `json:"digest"`
	Family           Family           `json:"family"`
	InteractionLevel InteractionLevel `json:"interaction_level"`
}

// packIndex mirrors decoys/index.json, which is the canonical file and is
// checked against this table by TestPackIndexMatchesCanonicalFile. The table
// is Go rather than an embedded file because decoys/ sits outside this Go
// module and cannot be embedded from here.
//
// Digest is empty for every entry: P2-W4 defines what a manifest contains and
// P2-W5 through P2-W8 build the packs, so there is nothing yet to hash.
// Recording an invented digest would assert an artifact identity nothing has
// verified, which is the same failure as reporting an unreported decoy as
// healthy. It is reported as null and stays that way until a pack exists.
var packIndex = [...]PackEntry{
	{Pack: "ssh-cowrie", Version: "0.1.0", Digest: "", Family: FamilySSH, InteractionLevel: InteractionMedium},
	{Pack: "http-admin", Version: "0.1.0", Digest: "", Family: FamilyHTTP, InteractionLevel: InteractionLow},
	{Pack: "postgres-service", Version: "0.1.0", Digest: "", Family: FamilyPostgres, InteractionLevel: InteractionLow},
	{Pack: "smb-fileshare", Version: "0.1.0", Digest: "", Family: FamilySMB, InteractionLevel: InteractionLow},
}

// Packs returns the index in declaration order as a defensive copy.
func Packs() []PackEntry {
	result := make([]PackEntry, len(packIndex))
	copy(result, packIndex[:])
	return result
}

// ResolvePack returns the index entry for one (pack, version) pair and rejects
// a family that disagrees with it. A decoy whose declared family does not match
// its pack would render as one thing in the console and behave as another.
func ResolvePack(pack, version string, family Family) (PackEntry, error) {
	if !family.Valid() {
		return PackEntry{}, violate(FieldFamily, ReasonUnsupported, ErrInvalidInput)
	}
	for _, entry := range packIndex {
		if entry.Pack != pack || entry.Version != version {
			continue
		}
		if entry.Family != family {
			return PackEntry{}, violate(FieldPack, ReasonUnsupported, ErrInvalidInput)
		}
		if err := InteractionLevelFor(entry.Family, entry.InteractionLevel); err != nil {
			return PackEntry{}, err
		}
		return entry, nil
	}
	// The pack name alone may be valid, so the pair is what is unknown. It is
	// attributed to the version because the console offers packs as a closed
	// list and the version is the half an operator can get wrong.
	return PackEntry{}, violate(FieldPackVersion, ReasonUnknown, ErrUnknownPack)
}
