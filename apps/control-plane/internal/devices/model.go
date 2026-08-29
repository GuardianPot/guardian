package devices

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
)

const (
	EnrollmentTokenLifetime = 15 * time.Minute
	enrollmentTokenBytes    = 32
	maxDeviceNameBytes      = 128
)

var (
	ErrInvalidInput          = errors.New("device enrollment input is invalid")
	ErrTokenInvalid          = errors.New("enrollment token is invalid")
	ErrTokenExpired          = errors.New("enrollment token is expired")
	ErrTokenConsumed         = errors.New("enrollment token is consumed")
	ErrTokenRevoked          = errors.New("enrollment token is revoked")
	ErrDeviceDisabled        = errors.New("device is disabled")
	ErrDeviceRevoked         = errors.New("device is revoked")
	ErrCertificateRevoked    = errors.New("device certificate is revoked")
	ErrCertificateStale      = errors.New("device certificate is not active")
	ErrEnrollmentRateLimited = errors.New("enrollment attempts are rate limited")
	ErrNotFound              = errors.New("device was not found")
)

type DeviceState string

const (
	DevicePending  DeviceState = "pending"
	DeviceActive   DeviceState = "active"
	DeviceDisabled DeviceState = "disabled"
	DeviceRevoked  DeviceState = "revoked"
)

// EnrollmentTokenRecord is the durable non-secret token metadata.
type EnrollmentTokenRecord struct {
	TokenID       string
	DeviceID      string
	EnvironmentID string
	DeviceName    string
	TokenHash     [sha256.Size]byte
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// EnrollmentToken is returned once to an authorized operator. Token is never
// persisted by the service or included in audit snapshots.
type EnrollmentToken struct {
	TokenID       string    `json:"token_id"`
	DeviceID      string    `json:"device_id"`
	EnvironmentID string    `json:"environment_id"`
	DeviceName    string    `json:"device_name"`
	Token         string    `json:"token"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type TokenSummary struct {
	TokenID       string     `json:"token_id"`
	DeviceID      string     `json:"device_id"`
	EnvironmentID string     `json:"environment_id"`
	DeviceName    string     `json:"device_name"`
	ExpiresAt     time.Time  `json:"expires_at"`
	ConsumedAt    *time.Time `json:"consumed_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// InventoryDevice is the deliberately narrow, non-secret operator view of a
// device. Enrollment material, certificate identities, health projections,
// and internal reconciliation state are kept behind their own boundaries.
type InventoryDevice struct {
	DeviceID                   string      `json:"device_id"`
	EnvironmentID              string      `json:"environment_id"`
	DisplayName                string      `json:"display_name"`
	State                      DeviceState `json:"state"`
	CreatedAt                  time.Time   `json:"created_at"`
	UpdatedAt                  time.Time   `json:"updated_at"`
	ActiveCertificateExpiresAt *time.Time  `json:"active_certificate_expires_at,omitempty"`
}

// InventoryRepository owns the bounded read-only device inventory projection.
type InventoryRepository interface {
	ListInventoryDevices(context.Context, string, int32) ([]InventoryDevice, error)
	InventoryDevice(context.Context, string, string) (InventoryDevice, error)
}

// InventoryService validates public identifiers and enforces the fixed list
// bound before delegating to storage.
type InventoryService struct {
	repository InventoryRepository
}

func NewInventoryService(repository InventoryRepository) (*InventoryService, error) {
	if repository == nil {
		return nil, errors.New("device inventory repository is required")
	}
	return &InventoryService{repository: repository}, nil
}

func (s *InventoryService) ListDevices(ctx context.Context, environmentID string) ([]InventoryDevice, error) {
	if !validUUID(environmentID) {
		return nil, ErrInvalidInput
	}
	return s.repository.ListInventoryDevices(ctx, environmentID, 200)
}

func (s *InventoryService) Device(ctx context.Context, environmentID, deviceID string) (InventoryDevice, error) {
	if !validUUID(environmentID) || !validUUIDv7(deviceID) {
		return InventoryDevice{}, ErrInvalidInput
	}
	return s.repository.InventoryDevice(ctx, environmentID, deviceID)
}

type Certificate struct {
	DeviceID      string
	EnvironmentID string
	Serial        string
	Fingerprint   string
	PEM           []byte
	NotBefore     time.Time
	NotAfter      time.Time
}

type EnrollmentResult struct {
	Certificate Certificate
	CAPEM       []byte
}

// Repository owns durable token/device/certificate transitions and their
// atomic audit append.
type Repository interface {
	CreateEnrollmentToken(context.Context, EnrollmentTokenRecord, string) error
	CreateReenrollmentToken(context.Context, EnrollmentTokenRecord, string) (string, error)
	ListEnrollmentTokens(context.Context, string) ([]TokenSummary, error)
	RevokeEnrollmentToken(context.Context, string, string, string, string) error
	Enroll(context.Context, [sha256.Size]byte, time.Time, func(string) (Certificate, error)) (Certificate, error)
	Rotate(context.Context, string, string, time.Time, func(string) (Certificate, error)) (Certificate, error)
	SetDeviceState(context.Context, string, string, DeviceState, string) error
	CertificateEligible(context.Context, string, string) error
	AllowEnrollmentAttempt(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error
	RecordEnrollmentFailure(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error
	ClearEnrollmentThrottle(context.Context, [sha256.Size]byte, [sha256.Size]byte) error
}

// Service coordinates bounded secrets and cryptography while Repository owns
// all durable concurrency and audit behavior.
type Service struct {
	repository Repository
	authority  *devicepki.ProductAuthority
	now        func() time.Time
}

func NewService(repository Repository, authority *devicepki.ProductAuthority) (*Service, error) {
	if repository == nil || authority == nil {
		return nil, errors.New("device repository and authority are required")
	}
	return &Service{repository: repository, authority: authority, now: time.Now}, nil
}

func (s *Service) CreateEnrollmentToken(ctx context.Context, environmentID, deviceName, actorID string) (EnrollmentToken, error) {
	if !validUUID(environmentID) || !validBoundedText(deviceName, maxDeviceNameBytes) || !validBoundedText(actorID, 256) {
		return EnrollmentToken{}, ErrInvalidInput
	}
	now := s.now().UTC()
	deviceID, err := newUUIDv7(now)
	if err != nil {
		return EnrollmentToken{}, err
	}
	result, record, err := newEnrollmentToken(now, environmentID, deviceID, deviceName)
	if err != nil {
		return EnrollmentToken{}, err
	}
	if err := s.repository.CreateEnrollmentToken(ctx, record, actorID); err != nil {
		return EnrollmentToken{}, err
	}
	return result, nil
}

// CreateReenrollmentToken is the explicit operator re-enable boundary. The
// repository permanently revokes old certificate/token material while keeping
// the stable device ID and moving the record back to pending.
func (s *Service) CreateReenrollmentToken(ctx context.Context, environmentID, deviceID, actorID string) (EnrollmentToken, error) {
	if !validUUID(environmentID) || !validUUIDv7(deviceID) || !validBoundedText(actorID, 256) {
		return EnrollmentToken{}, ErrInvalidInput
	}
	now := s.now().UTC()
	result, record, err := newEnrollmentToken(now, environmentID, deviceID, "pending-reenrollment")
	if err != nil {
		return EnrollmentToken{}, err
	}
	deviceName, err := s.repository.CreateReenrollmentToken(ctx, record, actorID)
	if err != nil {
		return EnrollmentToken{}, err
	}
	result.DeviceName = deviceName
	return result, nil
}

func (s *Service) ListEnrollmentTokens(ctx context.Context, environmentID string) ([]TokenSummary, error) {
	if !validUUID(environmentID) {
		return nil, ErrInvalidInput
	}
	return s.repository.ListEnrollmentTokens(ctx, environmentID)
}

func (s *Service) RevokeEnrollmentToken(ctx context.Context, environmentID, tokenID, actorID string) error {
	if !validUUID(environmentID) || !validUUIDv7(tokenID) || !validBoundedText(actorID, 256) {
		return ErrInvalidInput
	}
	return s.repository.RevokeEnrollmentToken(ctx, environmentID, tokenID, actorID, newCorrelationID())
}

func (s *Service) Enroll(ctx context.Context, source, encodedToken string, csrPEM []byte) (EnrollmentResult, error) {
	if !validBoundedText(source, 256) {
		return EnrollmentResult{}, ErrTokenInvalid
	}
	tokenScope := sha256.Sum256([]byte(encodedToken))
	sourceScope := sha256.Sum256([]byte(source))
	now := s.now().UTC()
	if err := s.repository.AllowEnrollmentAttempt(ctx, tokenScope, sourceScope, now); err != nil {
		return EnrollmentResult{}, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(encodedToken)
	if err != nil || len(raw) != enrollmentTokenBytes {
		clear(raw)
		if throttleErr := s.repository.RecordEnrollmentFailure(ctx, tokenScope, sourceScope, now); throttleErr != nil {
			return EnrollmentResult{}, errors.Join(ErrTokenInvalid, throttleErr)
		}
		return EnrollmentResult{}, ErrTokenInvalid
	}
	hash := sha256.Sum256(raw)
	clear(raw)
	certificate, err := s.repository.Enroll(ctx, hash, now, func(deviceID string) (Certificate, error) {
		issued, err := s.authority.Issue(deviceID, csrPEM, now)
		if err != nil {
			return Certificate{}, err
		}
		return certificateFromIssued(issued), nil
	})
	if err != nil {
		if throttleErr := s.repository.RecordEnrollmentFailure(ctx, tokenScope, sourceScope, now); throttleErr != nil {
			return EnrollmentResult{}, errors.Join(err, throttleErr)
		}
		return EnrollmentResult{}, err
	}
	if err := s.repository.ClearEnrollmentThrottle(ctx, tokenScope, sourceScope); err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{Certificate: certificate, CAPEM: s.authority.CertificatePEM()}, nil
}

func (s *Service) Rotate(ctx context.Context, deviceID, currentSerial string, csrPEM []byte) (EnrollmentResult, error) {
	if !validUUIDv7(deviceID) || !validSerial(currentSerial) {
		return EnrollmentResult{}, ErrInvalidInput
	}
	now := s.now().UTC()
	certificate, err := s.repository.Rotate(ctx, deviceID, currentSerial, now, func(stableID string) (Certificate, error) {
		issued, err := s.authority.Issue(stableID, csrPEM, now)
		if err != nil {
			return Certificate{}, err
		}
		return certificateFromIssued(issued), nil
	})
	if err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{Certificate: certificate, CAPEM: s.authority.CertificatePEM()}, nil
}

func (s *Service) SetDeviceState(ctx context.Context, environmentID, deviceID string, state DeviceState, actorID string) error {
	if !validUUID(environmentID) || !validUUIDv7(deviceID) || !validBoundedText(actorID, 256) || (state != DeviceDisabled && state != DeviceRevoked) {
		return ErrInvalidInput
	}
	return s.repository.SetDeviceState(ctx, environmentID, deviceID, state, actorID)
}

// VerifyCertificate combines chain/identity verification with durable active
// serial and device-state checks. P1-W5 can reuse this exact fail-closed seam.
func (s *Service) VerifyCertificate(ctx context.Context, certificate *x509.Certificate) (string, string, error) {
	deviceID, err := s.authority.Verify(certificate, s.now().UTC())
	if err != nil {
		return "", "", err
	}
	serial := strings.ToLower(certificate.SerialNumber.Text(16))
	if err := s.repository.CertificateEligible(ctx, deviceID, serial); err != nil {
		return "", "", err
	}
	return deviceID, serial, nil
}

func certificateFromIssued(issued devicepki.IssuedCertificate) Certificate {
	return Certificate{
		DeviceID:    issued.DeviceID,
		Serial:      issued.Serial,
		Fingerprint: issued.Fingerprint,
		PEM:         append([]byte(nil), issued.PEM...),
		NotBefore:   issued.NotBefore,
		NotAfter:    issued.NotAfter,
	}
}

func newUUIDv7(now time.Time) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[6:]); err != nil {
		return "", fmt.Errorf("generate UUIDv7 randomness: %w", err)
	}
	milliseconds := uint64(now.UTC().UnixMilli())
	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)
	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded), nil
}

func validUUID(value string) bool {
	if len(value) != 36 || strings.ToLower(value) != value || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	return err == nil && len(decoded) == 16 && decoded[8]&0xc0 == 0x80
}

func validUUIDv7(value string) bool {
	return validUUID(value) && value[14] == '7'
}

func validSerial(value string) bool {
	if len(value) < 1 || len(value) > 32 || strings.ToLower(value) != value {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validBoundedText(value string, max int) bool {
	return value != "" && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func newCorrelationID() string {
	value, err := newUUIDv7(time.Now())
	if err != nil {
		return "device-operation"
	}
	return value
}

func newEnrollmentToken(now time.Time, environmentID, deviceID, deviceName string) (EnrollmentToken, EnrollmentTokenRecord, error) {
	tokenID, err := newUUIDv7(now)
	if err != nil {
		return EnrollmentToken{}, EnrollmentTokenRecord{}, err
	}
	raw := make([]byte, enrollmentTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return EnrollmentToken{}, EnrollmentTokenRecord{}, fmt.Errorf("generate enrollment token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	clear(raw)
	record := EnrollmentTokenRecord{
		TokenID:       tokenID,
		DeviceID:      deviceID,
		EnvironmentID: environmentID,
		DeviceName:    deviceName,
		TokenHash:     hash,
		ExpiresAt:     now.Add(EnrollmentTokenLifetime),
		CreatedAt:     now,
	}
	return EnrollmentToken{
		TokenID:       tokenID,
		DeviceID:      deviceID,
		EnvironmentID: environmentID,
		DeviceName:    deviceName,
		Token:         token,
		ExpiresAt:     record.ExpiresAt,
	}, record, nil
}
