//go:build integration

package storage

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/api"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/auth"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

func TestLocalAuthenticationPostgreSQLAndTLSContract(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 12)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x41}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	const origin = "https://guardian.example.test"
	service, err := auth.NewService(store, secrets, auth.DefaultArgon2Params, origin)
	if err != nil {
		t.Fatal(err)
	}

	const password = "Owner-Strong-Password-2026!"
	expiredToken, expiredHash, err := auth.GenerateOpaqueSecret()
	if err != nil {
		t.Fatal(err)
	}
	expiredNow := time.Now().UTC().Add(-20 * time.Minute)
	expiredID, err := auth.NewUUIDv7(expiredNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateBootstrapToken(ctx, auth.BootstrapTokenRecord{
		TokenID: expiredID, TokenHash: expiredHash, CreatedAt: expiredNow,
		ExpiresAt: expiredNow.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Bootstrap(ctx, expiredToken, "owner", password); !errors.Is(err, auth.ErrAuthenticationDenied) {
		t.Fatalf("expired bootstrap error = %v", err)
	}

	bootstrapToken, _, err := service.CreateBootstrapToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := exerciseConcurrentBootstrapTLS(t, store, service, bootstrapToken, password)
	if len(bootstrap.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("bootstrap recovery codes = %d", len(bootstrap.RecoveryCodes))
	}
	seed := provisioningSeed(t, bootstrap.ProvisioningURI)
	defer clear(seed)
	code, err := auth.TOTPCode(seed, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.LoginTOTP(ctx, "owner", password, code, "192.0.2.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LoginTOTP(ctx, "owner", password, code, "192.0.2.10"); !errors.Is(err, auth.ErrAuthenticationDenied) {
		t.Fatalf("TOTP replay error = %v", err)
	}
	if _, err := service.AuthorizeMutation(ctx, first.SessionToken, first.CSRFToken, "https://evil.example"); !errors.Is(err, auth.ErrOriginInvalid) {
		t.Fatalf("cross-origin mutation error = %v", err)
	}
	assertConcurrentSessionAuthorization(t, store, service, first.SessionToken, first.Session.SessionID)

	httpCredentials := exerciseAuthTLS(t, store, service, seed, password, origin)
	if _, err := service.AuthorizeRead(ctx, httpCredentials.SessionToken); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("logged-out HTTP session error = %v", err)
	}

	recovery, err := service.LoginRecovery(ctx, "owner", password, bootstrap.RecoveryCodes[0], "192.0.2.11")
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Session.SessionID == first.Session.SessionID {
		t.Fatal("recovery login reused a prior session identity")
	}
	if _, err := service.AuthorizeRead(ctx, first.SessionToken); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("recovery did not revoke prior session: %v", err)
	}
	if _, err := service.LoginRecovery(ctx, "owner", password, bootstrap.RecoveryCodes[0], "192.0.2.11"); !errors.Is(err, auth.ErrAuthenticationDenied) {
		t.Fatalf("recovery-code replay error = %v", err)
	}

	const newPassword = "Another-Strong-Password-2027!"
	changed, err := service.ChangePassword(ctx, recovery.SessionToken, recovery.CSRFToken, origin, password, newPassword)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Session.SessionID == recovery.Session.SessionID {
		t.Fatal("password change did not rotate the session identity")
	}
	if _, err := service.AuthorizeRead(ctx, recovery.SessionToken); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("password change did not revoke prior session: %v", err)
	}
	if err := service.Logout(ctx, changed.SessionToken, changed.CSRFToken, origin); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthorizeRead(ctx, changed.SessionToken); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("logout session error = %v", err)
	}

	assertSessionExpiry(t, store, bootstrap.UserID)
	store = assertDurableThrottle(t, ctx, databaseURL, service, store)
	assertAuthEvidenceRedacted(t, store, []string{
		bootstrapToken, expiredToken, password, newPassword, bootstrap.RecoveryCodes[0],
		first.SessionToken, first.CSRFToken, recovery.SessionToken, recovery.CSRFToken,
		provisioningSecret(t, bootstrap.ProvisioningURI),
	})
}

func exerciseConcurrentBootstrapTLS(
	t *testing.T,
	readiness api.Readiness,
	service *auth.Service,
	bootstrapToken, password string,
) auth.BootstrapResult {
	t.Helper()
	server := api.NewServer("127.0.0.1:0", readiness, nil, api.WithAuthService(service))
	tlsServer := httptest.NewTLSServer(server.Handler())
	defer tlsServer.Close()
	type bootstrapOutcome struct {
		result auth.BootstrapResult
		status int
		body   string
	}
	outcomes := make(chan bootstrapOutcome, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			headers := http.Header{"Authorization": {"Bearer " + bootstrapToken}}
			request, requestErr := http.NewRequest(http.MethodPost, tlsServer.URL+"/v1/auth/bootstrap",
				strings.NewReader(`{"username":"owner","password":"`+password+`"}`))
			if requestErr != nil {
				outcomes <- bootstrapOutcome{body: requestErr.Error()}
				return
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", headers.Get("Authorization"))
			response, requestErr := tlsServer.Client().Do(request)
			if requestErr != nil {
				outcomes <- bootstrapOutcome{body: requestErr.Error()}
				return
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			var result auth.BootstrapResult
			_ = json.Unmarshal(body, &result)
			outcomes <- bootstrapOutcome{result: result, status: response.StatusCode, body: string(body)}
		}()
	}
	start.Done()
	var bootstrap auth.BootstrapResult
	successes, replays := 0, 0
	for range 2 {
		outcome := <-outcomes
		switch {
		case outcome.status == http.StatusCreated:
			successes++
			bootstrap = outcome.result
		case outcome.status == http.StatusUnauthorized:
			replays++
		default:
			t.Fatalf("unexpected concurrent bootstrap result: status=%d body=%s", outcome.status, outcome.body)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("bootstrap successes=%d replays=%d", successes, replays)
	}
	return bootstrap
}

func exerciseAuthTLS(
	t *testing.T,
	readiness api.Readiness,
	service *auth.Service,
	seed []byte,
	password, origin string,
) auth.SessionCredentials {
	t.Helper()
	server := api.NewServer("127.0.0.1:0", readiness, nil, api.WithAuthService(service))
	tlsServer := httptest.NewTLSServer(server.Handler())
	defer tlsServer.Close()
	code, err := auth.TOTPCode(seed, time.Now().UTC().Add(auth.TOTPStep))
	if err != nil {
		t.Fatal(err)
	}
	response := authTLSRequest(t, tlsServer.Client(), http.MethodPost, tlsServer.URL+"/v1/auth/login",
		`{"username":"owner","password":"`+password+`","totp_code":"`+code+`"}`, nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("TLS login status=%d body=%s", response.StatusCode, payload)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("TLS login security headers = %v", response.Header)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("TLS login cookies = %v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != "__Host-guardian_session" || !cookie.Secure || !cookie.HttpOnly ||
		cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("session cookie contract = %+v", cookie)
	}
	var payload struct {
		CSRFToken string       `json:"csrf_token"`
		Session   auth.Session `json:"session"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	credentials := auth.SessionCredentials{SessionToken: cookie.Value, CSRFToken: payload.CSRFToken, Session: payload.Session}

	headers := http.Header{"X-CSRF-Token": {payload.CSRFToken}, "Origin": {"https://evil.example"}}
	headers.Add("Cookie", cookie.String())
	denied := authTLSRequest(t, tlsServer.Client(), http.MethodPost, tlsServer.URL+"/v1/auth/logout", "", headers)
	denied.Body.Close()
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-origin logout status = %d", denied.StatusCode)
	}
	headers.Set("Origin", origin)
	loggedOut := authTLSRequest(t, tlsServer.Client(), http.MethodPost, tlsServer.URL+"/v1/auth/logout", "", headers)
	loggedOut.Body.Close()
	if loggedOut.StatusCode != http.StatusNoContent || !strings.Contains(loggedOut.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout status=%d cookie=%q", loggedOut.StatusCode, loggedOut.Header.Get("Set-Cookie"))
	}
	return credentials
}

func authTLSRequest(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func provisioningSeed(t *testing.T, value string) []byte {
	t.Helper()
	secret := provisioningSecret(t, value)
	seed, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil || len(seed) != auth.TOTPSeedBytes {
		t.Fatalf("decode provisioning seed: %v", err)
	}
	return seed
}

func provisioningSecret(t *testing.T, value string) string {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "otpauth" {
		t.Fatalf("parse provisioning URI: %v", err)
	}
	return parsed.Query().Get("secret")
}

func assertSessionExpiry(t *testing.T, store *Store, userID string) {
	t.Helper()
	var now time.Time
	if err := store.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	for name, instants := range map[string][3]time.Time{
		"absolute": {now.Add(-9 * time.Hour), now.Add(-9 * time.Hour), now.Add(-time.Hour)},
		"idle":     {now.Add(-time.Hour), now.Add(-16 * time.Minute), now.Add(7 * time.Hour)},
	} {
		t.Run("session_expiry_"+name, func(t *testing.T) {
			createdAt, lastSeenAt, expiresAt := instants[0], instants[1], instants[2]
			token, tokenHash, err := auth.GenerateOpaqueSecret()
			if err != nil {
				t.Fatal(err)
			}
			_, csrfHash, err := auth.GenerateOpaqueSecret()
			if err != nil {
				t.Fatal(err)
			}
			sessionID, err := auth.NewUUIDv7(createdAt)
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.pool.Exec(context.Background(), `
INSERT INTO guardian_auth.sessions (
    session_id, user_id, token_hash, csrf_hash, created_at, last_seen_at, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)`, sessionID, userID, tokenHash[:], csrfHash[:], createdAt, lastSeenAt, expiresAt)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.AuthenticateSession(context.Background(), tokenHash, auth.SessionIdleExpiry); !errors.Is(err, auth.ErrSessionInvalid) {
				t.Fatalf("expired session token %s accepted: %v", token, err)
			}
		})
	}
}

func assertConcurrentSessionAuthorization(
	t *testing.T,
	store *Store,
	service *auth.Service,
	sessionToken, sessionID string,
) {
	t.Helper()
	const requestCount = 32
	start := make(chan struct{})
	outcomes := make(chan error, requestCount)
	var ready sync.WaitGroup
	ready.Add(requestCount)
	for range requestCount {
		go func() {
			ready.Done()
			<-start
			_, err := service.AuthorizeRead(context.Background(), sessionToken)
			outcomes <- err
		}()
	}
	ready.Wait()
	close(start)
	for range requestCount {
		if err := <-outcomes; err != nil {
			t.Fatalf("concurrent session authorization failed: %v", err)
		}
	}

	var lastSeenAt, databaseNow time.Time
	var revokedAt *time.Time
	if err := store.pool.QueryRow(context.Background(), `
SELECT last_seen_at, revoked_at, clock_timestamp()
FROM guardian_auth.sessions
WHERE session_id = $1`, sessionID).Scan(&lastSeenAt, &revokedAt, &databaseNow); err != nil {
		t.Fatal(err)
	}
	if revokedAt != nil {
		t.Fatalf("concurrent authorization revoked an active session at %s", revokedAt.UTC())
	}
	if lastSeenAt.After(databaseNow) {
		t.Fatalf("session last-seen time %s is ahead of database time %s", lastSeenAt.UTC(), databaseNow.UTC())
	}
}

func assertDurableThrottle(t *testing.T, ctx context.Context, databaseURL string, service *auth.Service, store *Store) *Store {
	t.Helper()
	var before int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM guardian_auth.authentication_throttles`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		username := "unknown-owner-" + string(rune('a'+time.Now().UnixNano()%20))
		_, _ = service.LoginTOTP(ctx, username, "Invalid-Password-Value", "000000", "198.51.100.8")
	}
	var after int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM guardian_auth.authentication_throttles`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after > before+2 {
		t.Fatalf("arbitrary unknown usernames grew throttle state: before=%d after=%d", before, after)
	}
	store.Close()
	reopened, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x41}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := auth.NewService(reopened, secrets, auth.DefaultArgon2Params, "https://guardian.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.LoginTOTP(ctx, "another-unknown-owner", "Invalid-Password-Value", "000000", "198.51.100.8"); !errors.Is(err, auth.ErrRateLimited) {
		t.Fatalf("durable throttle after reopen = %v", err)
	}
	return reopened
}

func assertAuthEvidenceRedacted(t *testing.T, store *Store, forbidden []string) {
	t.Helper()
	var auditText string
	if err := store.pool.QueryRow(context.Background(), `
SELECT COALESCE(string_agg(
    COALESCE(before_snapshot::text, '') || COALESCE(after_snapshot::text, ''), ' '
), '')
FROM guardian_audit.records`).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	for _, value := range forbidden {
		if value != "" && strings.Contains(auditText, value) {
			t.Fatalf("audit snapshots leaked forbidden authentication material")
		}
	}
	for _, action := range []audit.Action{
		audit.ActionBootstrapTokenCreated, audit.ActionBootstrapSucceeded, audit.ActionBootstrapFailed,
		audit.ActionLoginSucceeded, audit.ActionLoginFailed, audit.ActionMFAEnrolled,
		audit.ActionRecoveryCodeUsed, audit.ActionPasswordChanged, audit.ActionSessionRevoked, audit.ActionLogout,
	} {
		var count int
		if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM guardian_audit.records WHERE action = $1`, action).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count < 1 {
			t.Fatalf("missing authentication audit action %q", action)
		}
	}
	var invalidSecrets int
	if err := store.pool.QueryRow(context.Background(), `
SELECT
    (SELECT count(*) FROM guardian_auth.bootstrap_tokens WHERE octet_length(token_hash) <> 32) +
    (SELECT count(*) FROM guardian_auth.recovery_codes WHERE octet_length(code_hash) <> 32) +
    (SELECT count(*) FROM guardian_auth.sessions WHERE octet_length(token_hash) <> 32 OR octet_length(csrf_hash) <> 32) +
    (SELECT count(*) FROM guardian_auth.users WHERE position(convert_to('otpauth', 'UTF8') in totp_seed_envelope) <> 0)
`).Scan(&invalidSecrets); err != nil {
		t.Fatal(err)
	}
	if invalidSecrets != 0 {
		t.Fatalf("database secret-shape violations = %d", invalidSecrets)
	}
}
