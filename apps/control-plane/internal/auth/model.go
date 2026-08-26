package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"
)

type UserStatus string

const (
	UserPendingMFA UserStatus = "pending_mfa"
	UserActive     UserStatus = "active"
	RoleOwner                 = "owner"
)

var (
	ErrAuthenticationDenied = errors.New("authentication denied")
	ErrRateLimited          = errors.New("authentication rate limited")
	ErrBootstrapUnavailable = errors.New("bootstrap is unavailable")
	ErrSessionInvalid       = errors.New("session is invalid")
	ErrCSRFInvalid          = errors.New("CSRF validation failed")
	ErrOriginInvalid        = errors.New("origin validation failed")
	ErrInvalidInput         = errors.New("authentication input is invalid")
)

type BootstrapTokenRecord struct {
	TokenID   string
	TokenHash [sha256.Size]byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

type BootstrapOwnerRecord struct {
	TokenHash        [sha256.Size]byte
	UserID           string
	Username         string
	PasswordPHC      string
	TOTPSeedEnvelope []byte
	RecoveryHashes   [][sha256.Size]byte
	CreatedAt        time.Time
}

type Credential struct {
	UserID           string
	Username         string
	Status           UserStatus
	PasswordPHC      string
	TOTPSeedEnvelope []byte
	LastTOTPCounter  int64
}

type SessionRecord struct {
	SessionID  string
	UserID     string
	TokenHash  [sha256.Size]byte
	CSRFHash   [sha256.Size]byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type Session struct {
	SessionID  string            `json:"session_id"`
	UserID     string            `json:"user_id"`
	Username   string            `json:"username"`
	Role       string            `json:"role"`
	CSRFHash   [sha256.Size]byte `json:"-"`
	CreatedAt  time.Time         `json:"created_at"`
	LastSeenAt time.Time         `json:"last_seen_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
	RevokedAt  *time.Time        `json:"revoked_at,omitempty"`
	Current    bool              `json:"current"`
}

type LoginCompletion struct {
	UserID       string
	TOTPCounter  int64
	Session      SessionRecord
	AccountScope [sha256.Size]byte
	SourceScope  [sha256.Size]byte
	OccurredAt   time.Time
}

type RecoveryCompletion struct {
	UserID       string
	RecoveryHash [sha256.Size]byte
	Session      SessionRecord
	AccountScope [sha256.Size]byte
	SourceScope  [sha256.Size]byte
	OccurredAt   time.Time
}

type PasswordChange struct {
	UserID      string
	PasswordPHC string
	Session     SessionRecord
	OccurredAt  time.Time
}

type Repository interface {
	CreateBootstrapToken(context.Context, BootstrapTokenRecord) error
	BootstrapOwner(context.Context, BootstrapOwnerRecord) error
	AllowAuthentication(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error
	CredentialByUsername(context.Context, string) (Credential, error)
	CredentialByUserID(context.Context, string) (Credential, error)
	RecordAuthenticationFailure(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error
	CompleteTOTPLogin(context.Context, LoginCompletion) error
	CompleteRecoveryLogin(context.Context, RecoveryCompletion) error
	AuthenticateSession(context.Context, [sha256.Size]byte, time.Time, time.Duration) (Session, error)
	RevokeSession(context.Context, string, string, time.Time, string) error
	ListSessions(context.Context, string, string, time.Time) ([]Session, error)
	ChangePassword(context.Context, PasswordChange) error
}

type BootstrapResult struct {
	UserID          string   `json:"user_id"`
	Username        string   `json:"username"`
	ProvisioningURI string   `json:"provisioning_uri"`
	RecoveryCodes   []string `json:"recovery_codes"`
}

type SessionCredentials struct {
	SessionToken string
	CSRFToken    string
	Session      Session
}
