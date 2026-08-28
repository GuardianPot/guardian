package environment

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	DefaultListLimit  int32 = 50
	MaxListLimit      int32 = 200
	MaxNameRunes            = 128
	MaxNameBytes            = 512
	MaxRequestIDBytes       = 128
)

type Status string

const (
	StatusNeedsZones   Status = "needs_zones"
	StatusZonesDefined Status = "zones_defined"
)

var (
	ErrInvalidInput       = errors.New("environment input is invalid")
	ErrNotFound           = errors.New("environment resource was not found")
	ErrNameConflict       = errors.New("environment display name conflicts")
	ErrCIDRConflict       = errors.New("environment CIDR conflicts")
	ErrPreconditionFailed = errors.New("environment revision precondition failed")
)

type Organization struct {
	OrganizationID string    `json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type Environment struct {
	EnvironmentID  string    `json:"environment_id"`
	OrganizationID string    `json:"organization_id"`
	DisplayName    string    `json:"display_name"`
	Revision       int64     `json:"revision"`
	ZoneCount      int64     `json:"zone_count"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Zone struct {
	ZoneID        string    `json:"zone_id"`
	EnvironmentID string    `json:"environment_id"`
	DisplayName   string    `json:"display_name"`
	CIDR          string    `json:"cidr"`
	Revision      int64     `json:"revision"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type NormalizedName struct {
	DisplayName string
	NameKey     string
}

type Mutation struct {
	ActorID    string
	RequestID  string
	OccurredAt time.Time
}

type Repository interface {
	Organization(context.Context) (Organization, error)
	ListEnvironments(context.Context, int32) ([]Environment, error)
	Environment(context.Context, string) (Environment, error)
	CreateEnvironment(context.Context, NormalizedName, Mutation) (Environment, error)
	UpdateEnvironment(context.Context, string, NormalizedName, int64, Mutation) (Environment, error)
	ListZones(context.Context, string, int32) ([]Zone, error)
	Zone(context.Context, string, string) (Zone, error)
	CreateZone(context.Context, string, NormalizedName, string, Mutation) (Zone, error)
	UpdateZone(context.Context, string, string, NormalizedName, string, int64, Mutation) (Zone, error)
	RemoveZone(context.Context, string, string, int64, Mutation) error
}

func NormalizeName(value string) (NormalizedName, error) {
	if !utf8.ValidString(value) {
		return NormalizedName{}, fmt.Errorf("%w: display name must be valid UTF-8", ErrInvalidInput)
	}
	display := norm.NFC.String(strings.TrimSpace(value))
	if display == "" || utf8.RuneCountInString(display) > MaxNameRunes || len(display) > MaxNameBytes {
		return NormalizedName{}, fmt.Errorf(
			"%w: display name must contain 1..%d code points and at most %d bytes",
			ErrInvalidInput,
			MaxNameRunes,
			MaxNameBytes,
		)
	}
	for _, r := range display {
		if unicode.IsControl(r) {
			return NormalizedName{}, fmt.Errorf("%w: display name contains a control character", ErrInvalidInput)
		}
	}
	return NormalizedName{DisplayName: display, NameKey: norm.NFC.String(cases.Fold().String(display))}, nil
}

func NormalizePrivateIPv4Prefix(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("%w: CIDR must not be empty or padded", ErrInvalidInput)
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() <= 0 {
		return "", fmt.Errorf("%w: CIDR must be a canonical private IPv4 prefix", ErrInvalidInput)
	}
	if prefix != prefix.Masked() || prefix.String() != value {
		return "", fmt.Errorf("%w: CIDR contains host bits or is not canonical", ErrInvalidInput)
	}
	privateRoots := [...]netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	for _, root := range privateRoots {
		if prefix.Bits() >= root.Bits() && root.Contains(prefix.Addr()) {
			return prefix.String(), nil
		}
	}
	return "", fmt.Errorf("%w: CIDR must be wholly contained by an RFC1918 range", ErrInvalidInput)
}

func NormalizeListLimit(limit int32) (int32, error) {
	if limit == 0 {
		return DefaultListLimit, nil
	}
	if limit < 1 || limit > MaxListLimit {
		return 0, fmt.Errorf("%w: list limit must be between 1 and %d", ErrInvalidInput, MaxListLimit)
	}
	return limit, nil
}

func ValidateRevision(revision int64) error {
	if revision < 1 {
		return fmt.Errorf("%w: revision must be positive", ErrInvalidInput)
	}
	return nil
}

func normalizeMutation(mutation Mutation) (Mutation, error) {
	if mutation.ActorID == "" || strings.TrimSpace(mutation.ActorID) != mutation.ActorID || len(mutation.ActorID) > 256 {
		return Mutation{}, fmt.Errorf("%w: actor identity is invalid", ErrInvalidInput)
	}
	if mutation.RequestID != "" && (strings.TrimSpace(mutation.RequestID) != mutation.RequestID ||
		len(mutation.RequestID) > MaxRequestIDBytes) {
		return Mutation{}, fmt.Errorf("%w: request identity is invalid", ErrInvalidInput)
	}
	for _, value := range []string{mutation.ActorID, mutation.RequestID} {
		for _, r := range value {
			if unicode.IsControl(r) {
				return Mutation{}, fmt.Errorf("%w: mutation identity contains a control character", ErrInvalidInput)
			}
		}
	}
	if mutation.OccurredAt.IsZero() {
		mutation.OccurredAt = time.Now().UTC()
	} else {
		mutation.OccurredAt = mutation.OccurredAt.UTC()
	}
	return mutation, nil
}
