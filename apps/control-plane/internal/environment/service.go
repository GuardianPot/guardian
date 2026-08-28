package environment

import (
	"context"
	"errors"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("environment repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) Organization(ctx context.Context) (Organization, error) {
	return s.repository.Organization(ctx)
}

func (s *Service) ListEnvironments(ctx context.Context, limit int32) ([]Environment, error) {
	limit, err := NormalizeListLimit(limit)
	if err != nil {
		return nil, err
	}
	return s.repository.ListEnvironments(ctx, limit)
}

func (s *Service) Environment(ctx context.Context, environmentID string) (Environment, error) {
	if !validUUIDv7(environmentID) {
		return Environment{}, ErrNotFound
	}
	return s.repository.Environment(ctx, environmentID)
}

func (s *Service) CreateEnvironment(ctx context.Context, displayName string, mutation Mutation) (Environment, error) {
	name, err := NormalizeName(displayName)
	if err != nil {
		return Environment{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Environment{}, err
	}
	return s.repository.CreateEnvironment(ctx, name, mutation)
}

func (s *Service) UpdateEnvironment(
	ctx context.Context,
	environmentID, displayName string,
	expectedRevision int64,
	mutation Mutation,
) (Environment, error) {
	if !validUUIDv7(environmentID) {
		return Environment{}, ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return Environment{}, err
	}
	name, err := NormalizeName(displayName)
	if err != nil {
		return Environment{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Environment{}, err
	}
	return s.repository.UpdateEnvironment(ctx, environmentID, name, expectedRevision, mutation)
}

func (s *Service) ListZones(ctx context.Context, environmentID string, limit int32) ([]Zone, error) {
	if !validUUIDv7(environmentID) {
		return nil, ErrNotFound
	}
	limit, err := NormalizeListLimit(limit)
	if err != nil {
		return nil, err
	}
	return s.repository.ListZones(ctx, environmentID, limit)
}

func (s *Service) Zone(ctx context.Context, environmentID, zoneID string) (Zone, error) {
	if !validUUIDv7(environmentID) || !validUUIDv7(zoneID) {
		return Zone{}, ErrNotFound
	}
	return s.repository.Zone(ctx, environmentID, zoneID)
}

func (s *Service) CreateZone(
	ctx context.Context,
	environmentID, displayName, cidr string,
	mutation Mutation,
) (Zone, error) {
	if !validUUIDv7(environmentID) {
		return Zone{}, ErrNotFound
	}
	name, err := NormalizeName(displayName)
	if err != nil {
		return Zone{}, err
	}
	cidr, err = NormalizePrivateIPv4Prefix(cidr)
	if err != nil {
		return Zone{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Zone{}, err
	}
	return s.repository.CreateZone(ctx, environmentID, name, cidr, mutation)
}

func (s *Service) UpdateZone(
	ctx context.Context,
	environmentID, zoneID, displayName, cidr string,
	expectedRevision int64,
	mutation Mutation,
) (Zone, error) {
	if !validUUIDv7(environmentID) || !validUUIDv7(zoneID) {
		return Zone{}, ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return Zone{}, err
	}
	name, err := NormalizeName(displayName)
	if err != nil {
		return Zone{}, err
	}
	cidr, err = NormalizePrivateIPv4Prefix(cidr)
	if err != nil {
		return Zone{}, err
	}
	mutation, err = normalizeMutation(mutation)
	if err != nil {
		return Zone{}, err
	}
	return s.repository.UpdateZone(ctx, environmentID, zoneID, name, cidr, expectedRevision, mutation)
}

func (s *Service) RemoveZone(
	ctx context.Context,
	environmentID, zoneID string,
	expectedRevision int64,
	mutation Mutation,
) error {
	if !validUUIDv7(environmentID) || !validUUIDv7(zoneID) {
		return ErrNotFound
	}
	if err := ValidateRevision(expectedRevision); err != nil {
		return err
	}
	mutation, err := normalizeMutation(mutation)
	if err != nil {
		return err
	}
	return s.repository.RemoveZone(ctx, environmentID, zoneID, expectedRevision, mutation)
}

func validUUIDv7(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '7' {
		return false
	}
	switch value[19] {
	case '8', '9', 'a', 'b', 'A', 'B':
	default:
		return false
	}
	for i, c := range []byte(value) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
