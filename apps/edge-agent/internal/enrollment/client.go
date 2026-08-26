// Package enrollment owns the bounded TLS bootstrap and certificate-rotation
// client. Enrollment tokens and private keys never enter configuration or logs.
package enrollment

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
)

const (
	maximumResponseBytes = 64 << 10
	maximumTokenBytes    = 64
)

var (
	ErrInvalidEndpoint = errors.New("enrollment endpoint is invalid")
	ErrInvalidToken    = errors.New("enrollment token is invalid")
	ErrDenied          = errors.New("enrollment was denied")
	ErrInvalidResponse = errors.New("enrollment response is invalid")
)

type Client struct {
	HTTP *http.Client
}

type Result struct {
	DeviceID      string
	EnvironmentID string
	Serial        string
	NotAfter      time.Time
}

type responseEnvelope struct {
	ContractVersion   string    `json:"contract_version"`
	DeviceID          string    `json:"device_id"`
	EnvironmentID     string    `json:"environment_id"`
	CertificateSerial string    `json:"certificate_serial"`
	CertificatePEM    string    `json:"certificate_pem"`
	CACertificatePEM  string    `json:"ca_certificate_pem"`
	NotBefore         time.Time `json:"not_before"`
	NotAfter          time.Time `json:"not_after"`
}

// Enroll generates the Edge key locally, exchanges a one-time token and CSR,
// validates the returned chain, and installs the identity recoverably.
func (c *Client) Enroll(
	ctx context.Context,
	controlPlaneEndpoint string,
	token []byte,
	certPath, keyPath string,
) (Result, error) {
	if len(token) == 0 || len(token) > maximumTokenBytes || strings.TrimSpace(string(token)) != string(token) {
		return Result{}, ErrInvalidToken
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(token))
	if err != nil || len(decoded) != 32 {
		clear(decoded)
		return Result{}, ErrInvalidToken
	}
	clear(decoded)
	key, csrPEM, privateKeyPEM, err := newIdentityRequest()
	if err != nil {
		return Result{}, err
	}
	defer clear(privateKeyPEM)
	response, err := c.exchange(ctx, controlPlaneEndpoint, "/v1/enrollments", token, csrPEM, nil)
	if err != nil {
		return Result{}, err
	}
	if err := validateAndInstall(response, key, privateKeyPEM, certPath, keyPath, time.Now().UTC()); err != nil {
		return Result{}, err
	}
	return Result{DeviceID: response.DeviceID, EnvironmentID: response.EnvironmentID, Serial: response.CertificateSerial, NotAfter: response.NotAfter}, nil
}

// Rotate uses the active mTLS identity, generates a replacement key, and
// installs the returned pair through the same crash-recovery boundary.
func (c *Client) Rotate(
	ctx context.Context,
	controlPlaneEndpoint, certPath, keyPath string,
) (Result, error) {
	current, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil || len(current.Certificate) == 0 {
		return Result{}, identity.ErrInvalid
	}
	key, csrPEM, privateKeyPEM, err := newIdentityRequest()
	if err != nil {
		return Result{}, err
	}
	defer clear(privateKeyPEM)
	response, err := c.exchange(ctx, controlPlaneEndpoint, "/v1/device-certificates:rotate", nil, csrPEM, &current)
	if err != nil {
		return Result{}, err
	}
	if err := validateAndInstall(response, key, privateKeyPEM, certPath, keyPath, time.Now().UTC()); err != nil {
		return Result{}, err
	}
	return Result{DeviceID: response.DeviceID, EnvironmentID: response.EnvironmentID, Serial: response.CertificateSerial, NotAfter: response.NotAfter}, nil
}

func (c *Client) exchange(
	ctx context.Context,
	endpoint, path string,
	token, csrPEM []byte,
	clientIdentity *tls.Certificate,
) (responseEnvelope, error) {
	requestURL, err := enrollmentURL(endpoint, path)
	if err != nil {
		return responseEnvelope{}, err
	}
	payload, err := json.Marshal(map[string]string{"csr_pem": string(csrPEM)})
	if err != nil {
		return responseEnvelope{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return responseEnvelope{}, ErrInvalidEndpoint
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	if len(token) > 0 {
		request.Header.Set("Authorization", "Bearer "+string(token))
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	clone := *client
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
	}
	transport = transport.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.MinVersion = tls.VersionTLS13
	if clientIdentity != nil {
		transport.TLSClientConfig.Certificates = []tls.Certificate{*clientIdentity}
	}
	clone.Transport = transport
	client = &clone
	httpResponse, err := client.Do(request)
	if err != nil {
		return responseEnvelope{}, fmt.Errorf("perform enrollment exchange: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusCreated {
		_, _ = io.Copy(io.Discard, io.LimitReader(httpResponse.Body, maximumResponseBytes))
		return responseEnvelope{}, ErrDenied
	}
	limited := io.LimitReader(httpResponse.Body, maximumResponseBytes+1)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var response responseEnvelope
	if err := decoder.Decode(&response); err != nil {
		return responseEnvelope{}, ErrInvalidResponse
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return responseEnvelope{}, ErrInvalidResponse
	}
	return response, nil
}

func enrollmentURL(endpoint, path string) (string, error) {
	if strings.Contains(endpoint, "://") {
		return "", ErrInvalidEndpoint
	}
	parsed, err := url.Parse("https://" + endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return "", ErrInvalidEndpoint
	}
	parsed.Path = path
	return parsed.String(), nil
}

func newIdentityRequest() (*ecdsa.PrivateKey, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate Edge identity key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create Edge identity CSR: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal Edge identity key: %w", err)
	}
	defer clear(privateDER)
	return key,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), nil
}

func validateAndInstall(
	response responseEnvelope,
	key *ecdsa.PrivateKey,
	privateKeyPEM []byte,
	certPath, keyPath string,
	now time.Time,
) error {
	if response.ContractVersion != "guardian.device.v1" || !uuidV7Pattern.MatchString(response.DeviceID) ||
		!uuidPattern.MatchString(response.EnvironmentID) || response.CertificateSerial == "" || response.NotAfter.IsZero() ||
		len(response.CertificatePEM) == 0 || len(response.CACertificatePEM) == 0 {
		return ErrInvalidResponse
	}
	block, rest := pem.Decode([]byte(response.CertificatePEM))
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return ErrInvalidResponse
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil || certificate.SerialNumber.Text(16) != response.CertificateSerial {
		return ErrInvalidResponse
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok || !publicKey.Equal(&key.PublicKey) {
		return ErrInvalidResponse
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(response.CACertificatePEM)) {
		return ErrInvalidResponse
	}
	if _, err := certificate.Verify(x509.VerifyOptions{
		Roots: roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return ErrInvalidResponse
	}
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != "urn:guardian:device:"+response.DeviceID {
		return ErrInvalidResponse
	}
	certificateBundle := append([]byte(response.CertificatePEM), []byte(response.CACertificatePEM)...)
	if err := identity.Install(certPath, keyPath, certificateBundle, privateKeyPEM); err != nil {
		return err
	}
	return nil
}

var (
	uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)
