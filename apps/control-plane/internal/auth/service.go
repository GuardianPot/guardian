package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

const (
	MaximumUsernameBytes = 64
	totpContextPrefix    = "guardian.auth.totp.v1:"
	defaultIssuer        = "Guardian"
)

type Service struct {
	repository   Repository
	secrets      secretstore.Store
	password     Argon2Params
	publicOrigin string
	issuer       string
	now          func() time.Time
	dummyPHC     string
	dummySeed    []byte
}

func NewService(repository Repository, secrets secretstore.Store, params Argon2Params, publicOrigin string) (*Service, error) {
	if repository == nil || secrets == nil || params.Validate() != nil || !validPublicOrigin(publicOrigin) {
		return nil, ErrInvalidInput
	}
	dummyPHC, err := HashPassword("guardian timing sentinel phrase", params)
	if err != nil {
		return nil, fmt.Errorf("build authentication timing sentinel: %w", err)
	}
	return &Service{
		repository: repository, secrets: secrets, password: params,
		publicOrigin: publicOrigin, issuer: defaultIssuer, now: func() time.Time { return time.Now().UTC() },
		dummyPHC: dummyPHC, dummySeed: make([]byte, TOTPSeedBytes),
	}, nil
}

func (s *Service) CreateBootstrapToken(ctx context.Context) (string, time.Time, error) {
	now := s.now().UTC()
	token, hash, err := GenerateOpaqueSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	tokenID, err := NewUUIDv7(now)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(BootstrapTokenExpiry)
	if err := s.repository.CreateBootstrapToken(ctx, BootstrapTokenRecord{
		TokenID: tokenID, TokenHash: hash, CreatedAt: now, ExpiresAt: expires,
	}); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) Bootstrap(ctx context.Context, encodedToken, username, password string) (BootstrapResult, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return BootstrapResult{}, err
	}
	normalizedPassword, err := NormalizeNewPassword(password, username)
	if err != nil {
		return BootstrapResult{}, err
	}
	tokenHash, err := HashOpaqueSecret(encodedToken)
	if err != nil {
		return BootstrapResult{}, ErrAuthenticationDenied
	}
	passwordPHC, err := HashPassword(normalizedPassword, s.password)
	if err != nil {
		return BootstrapResult{}, err
	}
	now := s.now().UTC()
	userID, err := NewUUIDv7(now)
	if err != nil {
		return BootstrapResult{}, err
	}
	seed, err := GenerateTOTPSeed()
	if err != nil {
		return BootstrapResult{}, err
	}
	defer clear(seed)
	envelope, err := s.secrets.Seal(seed, totpContext(userID))
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("seal owner TOTP seed: %w", err)
	}
	codes, hashes, err := GenerateRecoveryCodes()
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := s.repository.BootstrapOwner(ctx, BootstrapOwnerRecord{
		TokenHash: tokenHash, UserID: userID, Username: username, PasswordPHC: passwordPHC,
		TOTPSeedEnvelope: envelope, RecoveryHashes: hashes, CreatedAt: now,
	}); err != nil {
		return BootstrapResult{}, err
	}
	uri, err := ProvisioningURI(s.issuer, username, seed)
	if err != nil {
		return BootstrapResult{}, err
	}
	return BootstrapResult{UserID: userID, Username: username, ProvisioningURI: uri, RecoveryCodes: codes}, nil
}

func (s *Service) LoginTOTP(ctx context.Context, username, password, code, source string) (SessionCredentials, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	now := s.now().UTC()
	credential, credentialErr := s.repository.CredentialByUsername(ctx, username)
	if credentialErr != nil && !errors.Is(credentialErr, ErrAuthenticationDenied) {
		return SessionCredentials{}, credentialErr
	}
	accountIdentity := username
	if credentialErr != nil {
		accountIdentity = "unknown-account"
	}
	accountScope, sourceScope := authenticationScopes(accountIdentity, source)
	if err := s.repository.AllowAuthentication(ctx, accountScope, sourceScope, now); err != nil {
		return SessionCredentials{}, err
	}
	phc := s.dummyPHC
	seed := s.dummySeed
	lastCounter := int64(-1)
	if credentialErr == nil {
		phc = credential.PasswordPHC
		lastCounter = credential.LastTOTPCounter
		opened, openErr := s.secrets.Open(credential.TOTPSeedEnvelope, totpContext(credential.UserID))
		if openErr != nil || len(opened) != TOTPSeedBytes {
			clear(opened)
			credentialErr = ErrAuthenticationDenied
		} else {
			defer clear(opened)
			seed = opened
		}
	}
	passwordOK, verifyErr := VerifyPassword(password, phc)
	counter, totpOK := VerifyTOTP(seed, code, now, lastCounter)
	if credentialErr != nil || verifyErr != nil || !passwordOK || !totpOK {
		_ = s.repository.RecordAuthenticationFailure(ctx, accountScope, sourceScope, now)
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	credentials, record, err := newSessionCredentials(credential, now)
	if err != nil {
		return SessionCredentials{}, err
	}
	if err := s.repository.CompleteTOTPLogin(ctx, LoginCompletion{
		UserID: credential.UserID, TOTPCounter: counter, Session: record,
		AccountScope: accountScope, SourceScope: sourceScope, OccurredAt: now,
	}); err != nil {
		if errors.Is(err, ErrAuthenticationDenied) {
			_ = s.repository.RecordAuthenticationFailure(ctx, accountScope, sourceScope, now)
		}
		return SessionCredentials{}, err
	}
	return credentials, nil
}

func (s *Service) LoginRecovery(ctx context.Context, username, password, recoveryCode, source string) (SessionCredentials, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	now := s.now().UTC()
	credential, credentialErr := s.repository.CredentialByUsername(ctx, username)
	if credentialErr != nil && !errors.Is(credentialErr, ErrAuthenticationDenied) {
		return SessionCredentials{}, credentialErr
	}
	accountIdentity := username
	if credentialErr != nil {
		accountIdentity = "unknown-account"
	}
	accountScope, sourceScope := authenticationScopes(accountIdentity, source)
	if err := s.repository.AllowAuthentication(ctx, accountScope, sourceScope, now); err != nil {
		return SessionCredentials{}, err
	}
	phc := s.dummyPHC
	if credentialErr == nil {
		phc = credential.PasswordPHC
	}
	passwordOK, verifyErr := VerifyPassword(password, phc)
	recoveryHash, recoveryErr := HashRecoveryCode(recoveryCode)
	if credentialErr != nil || credential.Status != UserActive || verifyErr != nil || !passwordOK || recoveryErr != nil {
		_ = s.repository.RecordAuthenticationFailure(ctx, accountScope, sourceScope, now)
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	credentials, record, err := newSessionCredentials(credential, now)
	if err != nil {
		return SessionCredentials{}, err
	}
	if err := s.repository.CompleteRecoveryLogin(ctx, RecoveryCompletion{
		UserID: credential.UserID, RecoveryHash: recoveryHash, Session: record,
		AccountScope: accountScope, SourceScope: sourceScope, OccurredAt: now,
	}); err != nil {
		if errors.Is(err, ErrAuthenticationDenied) {
			_ = s.repository.RecordAuthenticationFailure(ctx, accountScope, sourceScope, now)
		}
		return SessionCredentials{}, err
	}
	return credentials, nil
}

func (s *Service) AuthorizeRead(ctx context.Context, sessionToken string) (Session, error) {
	hash, err := HashOpaqueSecret(sessionToken)
	if err != nil {
		return Session{}, ErrSessionInvalid
	}
	return s.repository.AuthenticateSession(ctx, hash, s.now().UTC(), SessionIdleExpiry)
}

func (s *Service) AuthorizeMutation(ctx context.Context, sessionToken, csrfToken, origin string) (Session, error) {
	if origin != s.publicOrigin {
		return Session{}, ErrOriginInvalid
	}
	session, err := s.AuthorizeRead(ctx, sessionToken)
	if err != nil {
		return Session{}, err
	}
	csrfHash, err := HashOpaqueSecret(csrfToken)
	if err != nil || subtle.ConstantTimeCompare(csrfHash[:], session.CSRFHash[:]) != 1 {
		return Session{}, ErrCSRFInvalid
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, sessionToken, csrfToken, origin string) error {
	session, err := s.AuthorizeMutation(ctx, sessionToken, csrfToken, origin)
	if err != nil {
		return err
	}
	return s.repository.RevokeSession(ctx, session.UserID, session.SessionID, s.now().UTC(), "logout")
}

func (s *Service) Sessions(ctx context.Context, sessionToken string) ([]Session, error) {
	current, err := s.AuthorizeRead(ctx, sessionToken)
	if err != nil {
		return nil, err
	}
	return s.repository.ListSessions(ctx, current.UserID, current.SessionID, s.now().UTC())
}

func (s *Service) RevokeSession(ctx context.Context, sessionToken, csrfToken, origin, targetSessionID string) error {
	current, err := s.AuthorizeMutation(ctx, sessionToken, csrfToken, origin)
	if err != nil {
		return err
	}
	if !validUUIDv7(targetSessionID) {
		return ErrInvalidInput
	}
	return s.repository.RevokeSession(ctx, current.UserID, targetSessionID, s.now().UTC(), "owner_revoked")
}

func (s *Service) ChangePassword(
	ctx context.Context,
	sessionToken, csrfToken, origin, currentPassword, newPassword string,
) (SessionCredentials, error) {
	current, err := s.AuthorizeMutation(ctx, sessionToken, csrfToken, origin)
	if err != nil {
		return SessionCredentials{}, err
	}
	credential, err := s.repository.CredentialByUserID(ctx, current.UserID)
	if err != nil {
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	valid, err := VerifyPassword(currentPassword, credential.PasswordPHC)
	if err != nil || !valid {
		return SessionCredentials{}, ErrAuthenticationDenied
	}
	normalized, err := NormalizeNewPassword(newPassword, credential.Username)
	if err != nil {
		return SessionCredentials{}, err
	}
	if same, verifyErr := VerifyPassword(normalized, credential.PasswordPHC); verifyErr == nil && same {
		return SessionCredentials{}, ErrInvalidPassword
	}
	phc, err := HashPassword(normalized, s.password)
	if err != nil {
		return SessionCredentials{}, err
	}
	now := s.now().UTC()
	credentials, record, err := newSessionCredentials(credential, now)
	if err != nil {
		return SessionCredentials{}, err
	}
	if err := s.repository.ChangePassword(ctx, PasswordChange{
		UserID: credential.UserID, PasswordPHC: phc, Session: record, OccurredAt: now,
	}); err != nil {
		return SessionCredentials{}, err
	}
	return credentials, nil
}

func newSessionCredentials(credential Credential, now time.Time) (SessionCredentials, SessionRecord, error) {
	sessionID, err := NewUUIDv7(now)
	if err != nil {
		return SessionCredentials{}, SessionRecord{}, err
	}
	sessionToken, tokenHash, err := GenerateOpaqueSecret()
	if err != nil {
		return SessionCredentials{}, SessionRecord{}, err
	}
	csrfToken, csrfHash, err := GenerateOpaqueSecret()
	if err != nil {
		return SessionCredentials{}, SessionRecord{}, err
	}
	record := SessionRecord{
		SessionID: sessionID, UserID: credential.UserID, TokenHash: tokenHash, CSRFHash: csrfHash,
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(SessionAbsoluteExpiry),
	}
	return SessionCredentials{
		SessionToken: sessionToken, CSRFToken: csrfToken,
		Session: Session{SessionID: sessionID, UserID: credential.UserID, Username: credential.Username,
			Role: RoleOwner, CreatedAt: now, LastSeenAt: now, ExpiresAt: record.ExpiresAt, Current: true},
	}, record, nil
}

func normalizeUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 || len(username) > MaximumUsernameBytes || !utf8.ValidString(username) {
		return "", ErrInvalidInput
	}
	for index := range username {
		value := username[index]
		if !((value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') || value == '.' || value == '_' || value == '-') {
			return "", ErrInvalidInput
		}
	}
	return username, nil
}

func authenticationScopes(username, source string) ([sha256.Size]byte, [sha256.Size]byte) {
	return sha256.Sum256([]byte("account:" + username)), sha256.Sum256([]byte("source:" + source))
}

func totpContext(userID string) []byte { return []byte(totpContextPrefix + userID) }

func validPublicOrigin(origin string) bool {
	return strings.HasPrefix(origin, "https://") && len(origin) <= 2048 &&
		!strings.ContainsAny(origin, "?#\r\n") && !strings.HasSuffix(origin, "/")
}

func validUUIDv7(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' ||
		value[14] != '7' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func IsDenied(err error) bool {
	return errors.Is(err, ErrAuthenticationDenied) || errors.Is(err, ErrSessionInvalid) ||
		errors.Is(err, ErrCSRFInvalid) || errors.Is(err, ErrOriginInvalid)
}
