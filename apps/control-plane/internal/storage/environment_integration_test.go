//go:build integration

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/api"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/auth"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnvironmentPostgreSQLContract(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 12)
	if err != nil {
		t.Fatal(err)
	}
	service, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	organization, err := service.Organization(ctx)
	if err != nil || organization.OrganizationID == "" || organization.OrganizationID[14] != '7' {
		t.Fatalf("organization = (%+v, %v), want singleton UUIDv7", organization, err)
	}
	var organizationCount int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_environment.organizations`).Scan(&organizationCount); err != nil || organizationCount != 1 {
		t.Fatalf("organization count = (%d, %v)", organizationCount, err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE guardian_environment.organizations SET created_at = clock_timestamp()`); err == nil {
		t.Fatal("immutable organization accepted an update")
	}

	primary := createIntegrationEnvironment(t, ctx, service, " Production ")
	secondary := createIntegrationEnvironment(t, ctx, service, "Disaster recovery")
	if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_environment.zones (environment_id, display_name, name_key, network)
VALUES ($1, 'Database bypass', 'database bypass', '8.8.8.0/24')
`, primary.EnvironmentID); !postgresCode(err, "23514") {
		t.Fatalf("database public-CIDR error = %v, want check violation", err)
	}
	if _, err := service.CreateEnvironment(ctx, "production", environment.Mutation{ActorID: "owner-1"}); !errors.Is(err, environment.ErrNameConflict) {
		t.Fatalf("case-folded duplicate environment error = %v", err)
	}

	zoneA, err := service.CreateZone(ctx, primary.EnvironmentID, "Application", "10.20.0.0/24", environment.Mutation{ActorID: "owner-1", RequestID: "zone-a"})
	if err != nil {
		t.Fatal(err)
	}
	zoneB, err := service.CreateZone(ctx, primary.EnvironmentID, "Database", "172.20.0.0/24", environment.Mutation{ActorID: "owner-1", RequestID: "zone-b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateZone(ctx, primary.EnvironmentID, "Nested", "10.20.0.0/25", environment.Mutation{ActorID: "owner-1"}); !errors.Is(err, environment.ErrCIDRConflict) {
		t.Fatalf("nested zone error = %v", err)
	}
	if _, err := service.CreateZone(ctx, secondary.EnvironmentID, "Cross-environment", "10.20.0.0/24", environment.Mutation{ActorID: "owner-1"}); err != nil {
		t.Fatalf("cross-environment overlap was rejected: %v", err)
	}
	for _, cidr := range []string{"10.0.0.1/24", "8.8.8.0/24", "127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "0.0.0.0/0", "2001:db8::/32", "invalid"} {
		if _, err := service.CreateZone(ctx, primary.EnvironmentID, "Rejected "+cidr, cidr, environment.Mutation{ActorID: "owner-1"}); !errors.Is(err, environment.ErrInvalidInput) {
			t.Errorf("CIDR %q error = %v", cidr, err)
		}
	}

	projected, err := service.Environment(ctx, primary.EnvironmentID)
	if err != nil || projected.Status != environment.StatusZonesDefined || projected.ZoneCount != 2 {
		t.Fatalf("environment projection = (%+v, %v)", projected, err)
	}
	updated, err := service.UpdateEnvironment(ctx, primary.EnvironmentID, "Production primary", projected.Revision, environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateEnvironment(ctx, primary.EnvironmentID, "Stale write", projected.Revision, environment.Mutation{ActorID: "owner-1"}); !errors.Is(err, environment.ErrPreconditionFailed) {
		t.Fatalf("stale environment update error = %v", err)
	}
	if updated.Revision != projected.Revision+1 {
		t.Fatalf("updated revision = %d, want %d", updated.Revision, projected.Revision+1)
	}

	zoneA, err = service.UpdateZone(ctx, primary.EnvironmentID, zoneA.ZoneID, "Application services", "192.168.20.0/24", zoneA.Revision, environment.Mutation{ActorID: "owner-1"})
	if err != nil || zoneA.Revision != 2 {
		t.Fatalf("zone update = (%+v, %v)", zoneA, err)
	}
	if err := service.RemoveZone(ctx, primary.EnvironmentID, zoneB.ZoneID, zoneB.Revision-1, environment.Mutation{ActorID: "owner-1"}); !errors.Is(err, environment.ErrInvalidInput) {
		t.Fatalf("invalid zone revision error = %v", err)
	}
	if err := service.RemoveZone(ctx, primary.EnvironmentID, zoneB.ZoneID, zoneB.Revision, environment.Mutation{ActorID: "owner-1"}); err != nil {
		t.Fatal(err)
	}

	testConcurrentZoneWinner(t, ctx, service, primary.EnvironmentID)
	testEnvironmentAuditRollback(t, ctx, store, service)
	testEnvironmentDeviceForeignKey(t, ctx, store, service)

	var mutationCount int
	if err := store.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM guardian_audit.records
WHERE action IN ('environment.created', 'environment.updated', 'zone.created', 'zone.updated', 'zone.removed')
`).Scan(&mutationCount); err != nil || mutationCount < 8 {
		t.Fatalf("environment audit count = (%d, %v)", mutationCount, err)
	}
	var secretShapeCount int
	if err := store.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM guardian_audit.records
WHERE COALESCE(before_snapshot::text, '') || COALESCE(after_snapshot::text, '')
      ~* '(password|token|secret|authorization|cookie|private[_ -]?key)'
`).Scan(&secretShapeCount); err != nil || secretShapeCount != 0 {
		t.Fatalf("secret-shaped environment audit snapshots = (%d, %v)", secretShapeCount, err)
	}

	store.Close()
	restarted, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restartedService, err := environment.NewService(restarted)
	if err != nil {
		t.Fatal(err)
	}
	restartedEnvironment, err := restartedService.Environment(ctx, primary.EnvironmentID)
	if err != nil || restartedEnvironment.DisplayName != "Production primary" || restartedEnvironment.Status != environment.StatusZonesDefined {
		t.Fatalf("restart projection = (%+v, %v)", restartedEnvironment, err)
	}
}

func TestEnvironmentHTTPSWithRealOwnerSession(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x53}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	const origin = "https://guardian.example.test"
	authService, err := auth.NewService(store, secrets, auth.DefaultArgon2Params, origin)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapToken, _, err := authService.CreateBootstrapToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const password = "Environment-Owner-Password-2026!"
	bootstrap, err := authService.Bootstrap(ctx, bootstrapToken, "environment-owner", password)
	if err != nil {
		t.Fatal(err)
	}
	seed := provisioningSeed(t, bootstrap.ProvisioningURI)
	defer clear(seed)
	code, err := auth.TOTPCode(seed, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := authService.LoginTOTP(ctx, "environment-owner", password, code, "192.0.2.30")
	if err != nil {
		t.Fatal(err)
	}
	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := api.EnvironmentAuthorizerFunc(func(
		ctx context.Context, session, csrf, requestOrigin string, mutation bool,
	) (string, error) {
		if mutation {
			resolved, err := authService.AuthorizeMutation(ctx, session, csrf, requestOrigin)
			return resolved.UserID, err
		}
		resolved, err := authService.AuthorizeRead(ctx, session)
		return resolved.UserID, err
	})
	server := api.NewServer("127.0.0.1:0", store, nil,
		api.WithEnvironmentService(environmentService), api.WithEnvironmentAuthorizer(authorizer))
	tlsServer := httptest.NewTLSServer(server.Handler())
	defer tlsServer.Close()

	request := func(method, path, body string, mutation bool, headers http.Header) *http.Response {
		t.Helper()
		if headers == nil {
			headers = make(http.Header)
		}
		headers.Set("Cookie", (&http.Cookie{Name: "__Host-guardian_session", Value: credentials.SessionToken}).String())
		if mutation {
			headers.Set("X-CSRF-Token", credentials.CSRFToken)
			headers.Set("Origin", origin)
		}
		return authTLSRequest(t, tlsServer.Client(), method, tlsServer.URL+path, body, headers)
	}

	organizationResponse := request(http.MethodGet, "/v1/organization", "", false, nil)
	if organizationResponse.StatusCode != http.StatusOK || organizationResponse.TLS == nil {
		t.Fatalf("organization HTTPS response = %d TLS=%+v", organizationResponse.StatusCode, organizationResponse.TLS)
	}
	organizationResponse.Body.Close()

	deniedHeaders := http.Header{
		"Origin":       {"https://evil.example"},
		"X-CSRF-Token": {credentials.CSRFToken},
	}
	denied := request(http.MethodPost, "/v1/environments", `{"display_name":"Denied"}`, false, deniedHeaders)
	denied.Body.Close()
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-origin mutation status = %d", denied.StatusCode)
	}

	createdResponse := request(http.MethodPost, "/v1/environments", `{"display_name":"Production"}`, true,
		http.Header{"X-Request-ID": {"https-environment-create"}})
	defer createdResponse.Body.Close()
	if createdResponse.StatusCode != http.StatusCreated || createdResponse.Header.Get("ETag") != `"1"` {
		body, _ := io.ReadAll(createdResponse.Body)
		t.Fatalf("create environment response = %d ETag=%q body=%s", createdResponse.StatusCode, createdResponse.Header.Get("ETag"), body)
	}
	var created struct {
		Environment environment.Environment `json:"environment"`
	}
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Environment.EnvironmentID == "" {
		t.Fatal("create environment omitted identity")
	}

	for _, zone := range []string{
		`{"display_name":"Application","cidr":"10.30.0.0/24"}`,
		`{"display_name":"Database","cidr":"192.168.30.0/24"}`,
	} {
		response := request(http.MethodPost, "/v1/environments/"+created.Environment.EnvironmentID+"/zones", zone, true, nil)
		response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create zone response = %d", response.StatusCode)
		}
	}

	getResponse := request(http.MethodGet, "/v1/environments/"+created.Environment.EnvironmentID, "", false, nil)
	var projected struct {
		Environment environment.Environment `json:"environment"`
	}
	if err := json.NewDecoder(getResponse.Body).Decode(&projected); err != nil {
		getResponse.Body.Close()
		t.Fatal(err)
	}
	getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusOK || projected.Environment.ZoneCount != 2 ||
		projected.Environment.Status != environment.StatusZonesDefined {
		t.Fatalf("projected HTTPS environment = status %d value %+v", getResponse.StatusCode, projected.Environment)
	}

	missing := request(http.MethodPatch, "/v1/environments/"+created.Environment.EnvironmentID, `{"display_name":"Missing precondition"}`, true, nil)
	missing.Body.Close()
	if missing.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status = %d", missing.StatusCode)
	}
	stale := request(http.MethodPatch, "/v1/environments/"+created.Environment.EnvironmentID, `{"display_name":"Stale"}`, true,
		http.Header{"If-Match": {`"1"`}})
	stale.Body.Close()
	if stale.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match status = %d", stale.StatusCode)
	}
	valid := request(http.MethodPatch, "/v1/environments/"+created.Environment.EnvironmentID, `{"display_name":"Production primary"}`, true,
		http.Header{"If-Match": {getResponse.Header.Get("ETag")}})
	valid.Body.Close()
	if valid.StatusCode != http.StatusOK {
		t.Fatalf("valid If-Match status = %d", valid.StatusCode)
	}

	oversized := request(http.MethodPost, "/v1/environments", `{"display_name":"`+strings.Repeat("x", 16<<10)+`"}`, true, nil)
	oversized.Body.Close()
	if oversized.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized request status = %d", oversized.StatusCode)
	}
	forbidden := request(http.MethodDelete, "/v1/environments/"+created.Environment.EnvironmentID, "", true, nil)
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("environment DELETE status = %d", forbidden.StatusCode)
	}
}

func createIntegrationEnvironment(t *testing.T, ctx context.Context, service *environment.Service, name string) environment.Environment {
	t.Helper()
	item, err := service.CreateEnvironment(ctx, name, environment.Mutation{ActorID: "owner-1", RequestID: "environment-create"})
	if err != nil {
		t.Fatal(err)
	}
	if item.EnvironmentID == "" || item.EnvironmentID[14] != '7' || item.DisplayName != strings.TrimSpace(name) {
		t.Fatalf("created environment = %+v", item)
	}
	return item
}

func testConcurrentZoneWinner(t *testing.T, ctx context.Context, service *environment.Service, environmentID string) {
	t.Helper()
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	for index, cidr := range []string{"10.90.0.0/16", "10.90.1.0/24"} {
		wait.Add(1)
		go func(name, cidr string) {
			defer wait.Done()
			<-start
			_, err := service.CreateZone(ctx, environmentID, name, cidr, environment.Mutation{ActorID: "owner-1"})
			errorsFound <- err
		}(string(rune('A'+index)), cidr)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	successes, conflicts := 0, 0
	for err := range errorsFound {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, environment.ErrCIDRConflict):
			conflicts++
		default:
			t.Fatalf("concurrent zone error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent zone outcomes = %d success, %d conflict", successes, conflicts)
	}
}

func testEnvironmentAuditRollback(t *testing.T, ctx context.Context, store *Store, service *environment.Service) {
	t.Helper()
	if _, err := store.pool.Exec(ctx, `
CREATE FUNCTION guardian_audit.reject_environment_test_insert()
RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced audit failure'; END; $$;
CREATE TRIGGER reject_environment_test_insert
BEFORE INSERT ON guardian_audit.records
FOR EACH ROW EXECUTE FUNCTION guardian_audit.reject_environment_test_insert();
`); err != nil {
		t.Fatal(err)
	}
	_, createErr := service.CreateEnvironment(ctx, "Must roll back", environment.Mutation{ActorID: "owner-1"})
	if createErr == nil {
		t.Fatal("environment committed despite forced audit failure")
	}
	if _, err := store.pool.Exec(ctx, `
DROP TRIGGER reject_environment_test_insert ON guardian_audit.records;
DROP FUNCTION guardian_audit.reject_environment_test_insert();
`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_environment.environments WHERE display_name = 'Must roll back'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled-back environment count = (%d, %v)", count, err)
	}
}

func testEnvironmentDeviceForeignKey(t *testing.T, ctx context.Context, store *Store, service *environment.Service) {
	t.Helper()
	now := time.Now().UTC()
	_, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES ('0198dc8c-c600-7000-8000-000000000051', '0198dc8c-c600-7000-8000-000000000052', 'orphan', 'pending', $1, $1)
`, now)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("orphan device error = %v, want foreign-key violation", err)
	}
	owned := createIntegrationEnvironment(t, ctx, service, "Device-owned environment")
	if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES ('0198dc8c-c600-7000-8000-000000000053', $1, 'owned', 'pending', $2, $2)
`, owned.EnvironmentID, now); err != nil {
		t.Fatal(err)
	}
	_, err = store.pool.Exec(ctx, `DELETE FROM guardian_environment.environments WHERE environment_id = $1`, owned.EnvironmentID)
	if !errors.As(err, &pgErr) || pgErr.Code != "23001" {
		t.Fatalf("referenced environment delete error = %v, want foreign-key violation", err)
	}
}

func postgresCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
