package api

import (
	"context"
	"errors"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
)

// errUnexpectedForTest stands in for a failure the API layer has no case for.
// The contract's answer to one of these is `internal.unexpected` and nothing
// else, so the text here must never reach a response.
var errUnexpectedForTest = errors.New("postgres: relation \"decoys\" does not exist")

// errUnreachableRepository fails any storage call the CP-0004 contract cases
// reach. Every case in that suite is rejected by domain validation first, so a
// response carrying this error means validation let something through.
var errUnreachableRepository = errors.New("storage was reached by an input validation should have refused")

func environmentTestServer(service EnvironmentService) *Server {
	authorizer := EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
		return "owner-1", nil
	})
	return NewServer("127.0.0.1:0", nil, discardLogger(),
		WithEnvironmentService(service), WithEnvironmentAuthorizer(authorizer))
}

type unreachableDecoyRepository struct{}

func (unreachableDecoyRepository) ListDecoys(context.Context, string, int32) ([]deception.View, error) {
	return nil, errUnreachableRepository
}

func (unreachableDecoyRepository) Decoy(context.Context, string, string) (deception.View, error) {
	return deception.View{}, errUnreachableRepository
}

func (unreachableDecoyRepository) CreateDecoy(
	context.Context, string, deception.Write, deception.Mutation,
) (deception.Decoy, error) {
	return deception.Decoy{}, errUnreachableRepository
}

func (unreachableDecoyRepository) UpdateDecoy(
	context.Context, string, string, deception.Write, int64, deception.Mutation,
) (deception.Decoy, error) {
	return deception.Decoy{}, errUnreachableRepository
}

func (unreachableDecoyRepository) SetDecoyDesiredState(
	context.Context, string, string, deception.DesiredState, int64, deception.Mutation,
) (deception.Decoy, error) {
	return deception.Decoy{}, errUnreachableRepository
}

func (unreachableDecoyRepository) RemoveDecoy(
	context.Context, string, string, int64, deception.Mutation,
) (deception.Decoy, error) {
	return deception.Decoy{}, errUnreachableRepository
}

func (unreachableDecoyRepository) RecordObservations(context.Context, deception.Report) error {
	return errUnreachableRepository
}

type unreachableEnvironmentRepository struct{}

func (unreachableEnvironmentRepository) Organization(context.Context) (environment.Organization, error) {
	return environment.Organization{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) ListEnvironments(context.Context, int32) ([]environment.Environment, error) {
	return nil, errUnreachableRepository
}

func (unreachableEnvironmentRepository) Environment(context.Context, string) (environment.Environment, error) {
	return environment.Environment{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) CreateEnvironment(
	context.Context, environment.NormalizedName, environment.Mutation,
) (environment.Environment, error) {
	return environment.Environment{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) UpdateEnvironment(
	context.Context, string, environment.NormalizedName, int64, environment.Mutation,
) (environment.Environment, error) {
	return environment.Environment{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) ListZones(context.Context, string, int32) ([]environment.Zone, error) {
	return nil, errUnreachableRepository
}

func (unreachableEnvironmentRepository) Zone(context.Context, string, string) (environment.Zone, error) {
	return environment.Zone{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) CreateZone(
	context.Context, string, environment.NormalizedName, string, environment.Mutation,
) (environment.Zone, error) {
	return environment.Zone{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) UpdateZone(
	context.Context, string, string, environment.NormalizedName, string, int64, environment.Mutation,
) (environment.Zone, error) {
	return environment.Zone{}, errUnreachableRepository
}

func (unreachableEnvironmentRepository) RemoveZone(
	context.Context, string, string, int64, environment.Mutation,
) error {
	return errUnreachableRepository
}
