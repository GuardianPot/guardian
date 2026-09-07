package deception

import (
	"context"
	"errors"
)

// Service is the P2-W15 application boundary. It validates every operator
// input against the closed vocabularies before storage sees it, and it has no
// method that writes observed state: that path belongs to the device channel
// alone.
type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("decoy repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) ListDecoys(ctx context.Context, environmentID string, limit int32) ([]View, error) {
	if !ValidUUIDv7(environmentID) {
		return nil, ErrNotFound
	}
	limit, err := NormalizeListLimit(limit)
	if err != nil {
		return nil, err
	}
	return s.repository.ListDecoys(ctx, environmentID, limit)
}

func (s *Service) Decoy(ctx context.Context, environmentID, decoyID string) (View, error) {
	if !ValidUUIDv7(environmentID) || !ValidUUIDv7(decoyID) {
		return View{}, ErrNotFound
	}
	return s.repository.Decoy(ctx, environmentID, decoyID)
}

// CreateDecoy places one decoy. The address is validated against its zone in
// storage, where the zone's prefix is known; everything the caller can decide
// on its own is decided here.
func (s *Service) CreateDecoy(
	ctx context.Context,
	environmentID string,
	input Input,
	mutation Mutation,
) (Decoy, error) {
	if !ValidUUIDv7(environmentID) {
		return Decoy{}, ErrNotFound
	}
	write, err := input.normalize()
	if err != nil {
		return Decoy{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Decoy{}, err
	}
	return s.repository.CreateDecoy(ctx, environmentID, write, mutation)
}

func (s *Service) UpdateDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	input Input,
	expectedRevision int64,
	mutation Mutation,
) (Decoy, error) {
	if !ValidUUIDv7(environmentID) || !ValidUUIDv7(decoyID) {
		return Decoy{}, ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return Decoy{}, err
	}
	write, err := input.normalize()
	if err != nil {
		return Decoy{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Decoy{}, err
	}
	return s.repository.UpdateDecoy(ctx, environmentID, decoyID, write, expectedRevision, mutation)
}

// EnableDecoy and DisableDecoy are separate operations rather than one PATCH
// field because WC-D16 assigns confirmation levels to operations.
func (s *Service) EnableDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	expectedRevision int64,
	mutation Mutation,
) (Decoy, error) {
	return s.setDesiredState(ctx, environmentID, decoyID, DesiredDeployed, expectedRevision, mutation)
}

func (s *Service) DisableDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	expectedRevision int64,
	mutation Mutation,
) (Decoy, error) {
	return s.setDesiredState(ctx, environmentID, decoyID, DesiredDisabled, expectedRevision, mutation)
}

// RemoveDecoy retires a decoy from the active list. It is not a deletion of
// history: the audit trail and every interaction already attributed to this
// decoy identity survive it.
func (s *Service) RemoveDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	expectedRevision int64,
	mutation Mutation,
) error {
	if !ValidUUIDv7(environmentID) || !ValidUUIDv7(decoyID) {
		return ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return err
	}
	mutation, err := normalizeMutation(mutation)
	if err != nil {
		return err
	}
	_, err = s.repository.RemoveDecoy(ctx, environmentID, decoyID, expectedRevision, mutation)
	return err
}

// RecordObservations ingests one Edge decoy report. It is reachable from the
// device channel and from nowhere else.
func (s *Service) RecordObservations(ctx context.Context, report Report) error {
	if err := ValidateReport(report); err != nil {
		return err
	}
	return s.repository.RecordObservations(ctx, report)
}

func (s *Service) setDesiredState(
	ctx context.Context,
	environmentID, decoyID string,
	state DesiredState,
	expectedRevision int64,
	mutation Mutation,
) (Decoy, error) {
	if !ValidUUIDv7(environmentID) || !ValidUUIDv7(decoyID) {
		return Decoy{}, ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return Decoy{}, err
	}
	mutation, err := normalizeMutation(mutation)
	if err != nil {
		return Decoy{}, err
	}
	return s.repository.SetDecoyDesiredState(ctx, environmentID, decoyID, state, expectedRevision, mutation)
}

// Input is the raw operator request. Nothing in it names a runtime artifact:
// pack and pack_version select a manifest from the server-side index, and the
// digest and interaction level are resolved there rather than supplied.
type Input struct {
	ZoneID      string
	DisplayName string
	Family      string
	Persona     string
	Address     string
	Pack        string
	PackVersion string
}

func (i Input) normalize() (Write, error) {
	if !ValidUUIDv7(i.ZoneID) {
		return Write{}, ErrNotFound
	}
	name, err := NormalizeName(i.DisplayName)
	if err != nil {
		return Write{}, err
	}
	family := Family(i.Family)
	persona := Persona(i.Persona)
	if !family.Valid() || !persona.Valid() {
		return Write{}, ErrInvalidInput
	}
	address, err := NormalizeAddress(i.Address)
	if err != nil {
		return Write{}, err
	}
	entry, err := ResolvePack(i.Pack, i.PackVersion, family)
	if err != nil {
		return Write{}, err
	}
	return Write{
		ZoneID: i.ZoneID, Name: name, Family: family, Persona: persona,
		InteractionLevel: entry.InteractionLevel, Address: address,
		Pack: entry.Pack, PackVersion: entry.Version, PackDigest: entry.Digest,
	}, nil
}
