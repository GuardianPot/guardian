package api

import (
	"bufio"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
)

// The CP-0004 evidence suite. The change proposal names five things that must
// be demonstrated before its approval is discharged, and each has a test here:
//
//   - the code vocabulary is closed and reviewed for disclosure
//     (TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract);
//   - no submitted value appears in any error response
//     (TestNoSubmittedValueReachesAnyErrorResponse);
//   - request_id carries no derivable meaning
//     (TestRequestIDCarriesNoDerivableMeaning);
//   - an unmigrated endpoint still behaves correctly
//     (TestUnmigratedEndpointStillReturnsThePhase1Body);
//   - no handler emits operator-facing prose
//     (TestNoHandlerEmitsOperatorFacingProse).

// proseKeys are the JSON keys that would carry operator-facing text if one ever
// appeared. The contract has none of them at any level, deliberately: a free
// sentence here would bypass the WCX-08 catalogue and the wording review it
// exists to enable.
var proseKeys = []string{"message", "detail", "details", "title", "description", "error", "reason", "hint"}

var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// observedErrorBody is the decoded response, kept separately from errorBody so
// a test failure reflects what was actually on the wire rather than what the
// server-side type would accept.
type observedErrorBody struct {
	Status      string `json:"status"`
	Code        string `json:"code"`
	FieldErrors []struct {
		Field      string `json:"field"`
		Code       string `json:"code"`
		MessageKey string `json:"message_key"`
	} `json:"field_errors"`
	RetryAfter *int   `json:"retry_after"`
	RequestID  string `json:"request_id"`
}

// TestUnmigratedEndpointStillReturnsThePhase1Body is the no-flag-day proof.
// Migration is per endpoint; an endpoint nobody has touched must be byte
// identical to what it returned before CP-0004, so the WCX-02 taxonomy keeps
// working against a partially migrated backend.
func TestUnmigratedEndpointStillReturnsThePhase1Body(t *testing.T) {
	unready := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error {
		return context.DeadlineExceeded
	}), discardLogger())

	for _, test := range []struct {
		name       string
		server     *Server
		request    *http.Request
		statusCode int
		want       string
	}{
		{
			name:       "readiness failure on an unmigrated endpoint",
			server:     unready,
			request:    httptest.NewRequest(http.MethodGet, "/readyz", nil),
			statusCode: http.StatusServiceUnavailable,
			want:       `{"status":"not_ready"}`,
		},
		{
			name:       "liveness on an unmigrated endpoint",
			server:     unready,
			request:    httptest.NewRequest(http.MethodGet, "/livez", nil),
			statusCode: http.StatusOK,
			want:       `{"status":"live"}`,
		},
		{
			// Authentication is not part of this migration. A rejected session
			// on an otherwise migrated route still writes the Phase 1 body, so
			// an unauthenticated caller gets no correlation identifier and no
			// vocabulary entry out of the server.
			name:   "unauthenticated request to a migrated route",
			server: NewServer("127.0.0.1:0", nil, discardLogger(), WithEnvironmentService(environmentServiceStub{})),
			request: func() *http.Request {
				return authenticatedEnvironmentRequest(t, http.MethodGet,
					"/v1/environments/"+testEnvironmentID, "", false)
			}(),
			statusCode: http.StatusUnauthorized,
			want:       `{"status":"unauthorized"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, test.request)
			if response.Code != test.statusCode {
				t.Fatalf("status = %d, want %d", response.Code, test.statusCode)
			}
			if got := strings.TrimSpace(response.Body.String()); got != test.want {
				t.Fatalf("body = %s, want exactly %s", got, test.want)
			}
			// Not just "no values": no keys at all beyond status.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			if len(raw) != 1 {
				t.Fatalf("unmigrated body carries %d keys, want only status", len(raw))
			}
		})
	}
}

// TestMigratedErrorsCarryTheClosedVocabulary walks every migrated failure on
// the environment, zone, and decoy surfaces and checks the whole contract on
// each: the Phase 1 status is unchanged, the code and every field error come
// from the closed vocabularies, and no prose key exists at any level.
func TestMigratedErrorsCarryTheClosedVocabulary(t *testing.T) {
	for _, test := range contractCases(t) {
		t.Run(test.name, func(t *testing.T) {
			response := test.run(t)
			if response.Code != test.statusCode {
				t.Fatalf("status = %d body = %s, want %d", response.Code, response.Body, test.statusCode)
			}
			body := decodeErrorBody(t, response)
			// Constraint 1: status keeps its Phase 1 value and meaning.
			if body.Status != test.status {
				t.Fatalf("status slug = %q, want %q", body.Status, test.status)
			}
			if body.Code != test.code {
				t.Fatalf("code = %q, want %q", body.Code, test.code)
			}
			if !errorCode(body.Code).valid() {
				t.Fatalf("code %q is outside the closed vocabulary", body.Code)
			}
			if !requestIDPattern.MatchString(body.RequestID) {
				t.Fatalf("request_id = %q, want 32 lowercase hex characters", body.RequestID)
			}
			if len(body.FieldErrors) != len(test.fields) {
				t.Fatalf("field_errors = %+v, want %+v", body.FieldErrors, test.fields)
			}
			for index, field := range body.FieldErrors {
				if field.Field != test.fields[index][0] || field.Code != test.fields[index][1] {
					t.Fatalf("field_errors[%d] = %q/%q, want %q/%q",
						index, field.Field, field.Code, test.fields[index][0], test.fields[index][1])
				}
				if !fieldPath(field.Field).valid() {
					t.Fatalf("field %q is outside the closed field vocabulary", field.Field)
				}
				if !fieldReason(field.Code).valid() {
					t.Fatalf("field code %q is outside the closed reason vocabulary", field.Code)
				}
				// Constraint 3: no endpoint emits a catalogue key today.
				if field.MessageKey != "" {
					t.Fatalf("field_errors[%d] carries message_key %q", index, field.MessageKey)
				}
			}
			assertNoProse(t, response.Body.Bytes())
		})
	}
}

// TestNoSubmittedValueReachesAnyErrorResponse is constraint 4's evidence.
//
// Every string an operator can put in a decoy or zone write is replaced with a
// distinctive marker, every marker is driven through every rejection path the
// endpoint has, and no marker may appear anywhere in the response — not in a
// value, not in a key, not in a code. A rejected persona or display name that
// came back would be a leakage path, and the decoy form is where the most of
// them are.
func TestNoSubmittedValueReachesAnyErrorResponse(t *testing.T) {
	const marker = "zzGuardianSubmittedSecretzz"
	markers := []string{
		marker,
		marker + "-password",
		"10.99.99." + "1",
		strings.Repeat(marker, 40),
	}

	server := decoyTestServer(decoyServiceStub{
		createDecoy: func(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error) {
			return deception.Decoy{}, deception.ErrNameConflict
		},
		updateDecoy: func(context.Context, string, string, deception.Input, int64, deception.Mutation) (deception.Decoy, error) {
			return deception.Decoy{}, deception.ErrAddressConflict
		},
	})
	environmentServer := environmentTestServer(environmentServiceStub{
		createZone: func(context.Context, string, string, string, environment.Mutation) (environment.Zone, error) {
			return environment.Zone{}, environment.ErrCIDRConflict
		},
		createEnvironment: func(context.Context, string, environment.Mutation) (environment.Environment, error) {
			return environment.Environment{}, environment.ErrNameConflict
		},
	})

	decoyBase := "/v1/environments/" + testEnvironmentID + "/decoys"
	zoneBase := "/v1/environments/" + testEnvironmentID + "/zones"

	for _, value := range markers {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		quoted := string(encoded)
		// One marker per field in turn, so a leak from any single field is
		// caught rather than masked by a neighbour.
		decoyBodies := []string{
			`{"zone_id":` + quoted + `,"display_name":"n","family":"smb","persona":"windows_file_service_host",` +
				`"address":"10.20.0.40","pack":"smb-fileshare","pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":` + quoted + `,"family":"smb",` +
				`"persona":"windows_file_service_host","address":"10.20.0.40","pack":"smb-fileshare","pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":` + quoted + `,` +
				`"persona":"windows_file_service_host","address":"10.20.0.40","pack":"smb-fileshare","pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":"smb","persona":` + quoted + `,` +
				`"address":"10.20.0.40","pack":"smb-fileshare","pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":"smb",` +
				`"persona":"windows_file_service_host","address":` + quoted + `,"pack":"smb-fileshare","pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":"smb",` +
				`"persona":"windows_file_service_host","address":"10.20.0.40","pack":` + quoted + `,"pack_version":"0.1.0"}`,
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":"smb",` +
				`"persona":"windows_file_service_host","address":"10.20.0.40","pack":"smb-fileshare","pack_version":` + quoted + `}`,
			// An unknown key, whose name is also operator-chosen.
			`{"zone_id":"` + testZoneID + `","display_name":"n","family":"smb",` +
				`"persona":"windows_file_service_host","address":"10.20.0.40","pack":"smb-fileshare",` +
				`"pack_version":"0.1.0",` + quoted + `:"x"}`,
		}
		for _, body := range decoyBodies {
			for _, target := range []struct{ method, path, ifMatch string }{
				{http.MethodPost, decoyBase, ""},
				{http.MethodPatch, decoyBase + "/" + testDecoyID, `"3"`},
			} {
				assertNoMarker(t, server, target.method, target.path, body, target.ifMatch, value)
			}
		}
		for _, body := range []string{
			`{"display_name":` + quoted + `,"cidr":"10.20.0.0/24"}`,
			`{"display_name":"Zone","cidr":` + quoted + `}`,
		} {
			assertNoMarker(t, environmentServer, http.MethodPost, zoneBase, body, "", value)
		}
		assertNoMarker(t, environmentServer, http.MethodPost, "/v1/environments",
			`{"display_name":`+quoted+`}`, "", value)
	}

	// The caller-supplied mutation header is operator input too, and it is never
	// echoed: the CP-0004 request_id is server-generated and unrelated to it.
	request := authenticatedEnvironmentRequest(t, http.MethodPost, decoyBase,
		`{"zone_id":"`+testZoneID+`","display_name":"n","family":"smb",`+
			`"persona":"windows_file_service_host","address":"10.20.0.40",`+
			`"pack":"smb-fileshare","pack_version":"0.1.0"}`, true)
	request.Header.Set("X-Request-ID", marker)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), marker) {
		t.Fatalf("X-Request-ID was echoed into the error body: %s", response.Body)
	}
}

func assertNoMarker(t *testing.T, server *Server, method, path, body, ifMatch, marker string) {
	t.Helper()
	request := authenticatedEnvironmentRequest(t, method, path, body, true)
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code < 400 {
		t.Fatalf("%s %s = %d; the case must fail for the assertion to mean anything", method, path, response.Code)
	}
	if strings.Contains(response.Body.String(), marker) {
		t.Fatalf("%s %s echoed a submitted value.\nbody sent: %s\nresponse: %s", method, path, body, response.Body)
	}
	assertNoProse(t, response.Body.Bytes())
}

// TestRequestIDCarriesNoDerivableMeaning is constraint 5's evidence.
//
// The identifier is safe to display and safe to place in the WCX-15 diagnostic
// report only if nothing can be read out of it. These assertions cover the
// three ways a correlation identifier usually leaks something: it encodes a
// clock, it encodes a counter, or it encodes something about the request.
func TestRequestIDCarriesNoDerivableMeaning(t *testing.T) {
	const samples = 512
	identifiers := make([]string, 0, samples)
	seen := make(map[string]struct{}, samples)
	for range samples {
		id := newRequestID()
		if !requestIDPattern.MatchString(id) {
			t.Fatalf("request_id = %q, want 32 lowercase hex characters", id)
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("request_id %q was issued twice", id)
		}
		seen[id] = struct{}{}
		identifiers = append(identifiers, id)
	}

	// No timestamp and no counter: a monotonic sequence would mean the value
	// encodes creation order, which is exactly what a ULID does and what
	// constraint 5 rules out.
	ascending, descending := 0, 0
	for index := 1; index < len(identifiers); index++ {
		switch {
		case identifiers[index] > identifiers[index-1]:
			ascending++
		case identifiers[index] < identifiers[index-1]:
			descending++
		}
	}
	if ascending == len(identifiers)-1 || descending == len(identifiers)-1 {
		t.Fatal("request_id is monotonic across issuance, so it encodes creation order")
	}

	// No fixed segment: every character position must vary, so no part of the
	// identifier is a constant, a prefix, or a derived tag.
	for position := range 32 {
		distinct := make(map[byte]struct{}, 16)
		for _, id := range identifiers {
			distinct[id[position]] = struct{}{}
		}
		if len(distinct) < 8 {
			t.Fatalf("position %d took only %d distinct values across %d identifiers",
				position, len(distinct), samples)
		}
	}

	// Identifiers issued a measurable interval apart are not ordered by that
	// interval, so wall-clock time is not recoverable from one.
	before := newRequestID()
	time.Sleep(2 * time.Millisecond)
	after := newRequestID()
	if before == after {
		t.Fatal("two identifiers issued at different times were identical")
	}

	// The same request repeated yields different identifiers, so the value is
	// not a function of the request: nothing about the caller, the route, the
	// body, or the failure is recoverable from it.
	server := decoyTestServer(decoyServiceStub{
		createDecoy: func(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error) {
			return deception.Decoy{}, deception.ErrNameConflict
		},
	})
	body := `{"zone_id":"` + testZoneID + `","display_name":"Identical","family":"smb",` +
		`"persona":"windows_file_service_host","address":"10.20.0.40",` +
		`"pack":"smb-fileshare","pack_version":"0.1.0"}`
	issued := make(map[string]struct{}, 8)
	for range 8 {
		request := authenticatedEnvironmentRequest(t, http.MethodPost,
			"/v1/environments/"+testEnvironmentID+"/decoys", body, true)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		issued[decodeErrorBody(t, response).RequestID] = struct{}{}
	}
	if len(issued) != 8 {
		t.Fatalf("8 identical requests produced %d distinct identifiers; the value is derived from the request", len(issued))
	}
}

// TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract is the disclosure
// review's machine-checkable half.
//
// Every entry must be a lowercase machine token; a code that named an internal
// table, a host, a driver, or a file would be a disclosure, and one that read
// as a sentence would be prose. The vocabularies must also match
// openapi/guardian.yaml exactly, so a code added in Go without a contract
// change fails the lane.
func TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract(t *testing.T) {
	tokenPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	// Substrings that would signal an implementation detail escaping into a
	// contract surface.
	forbidden := []string{
		"postgres", "pgx", "sql", "database", "db", "table", "column", "goose",
		"panic", "nil", "exception", "stack", "trace", "internal_error",
		"localhost", "127.0.0.1", "/", "\\", "@", ":",
	}

	codes := make([]string, 0, len(errorCodes()))
	for _, code := range errorCodes() {
		codes = append(codes, string(code))
	}
	paths := make([]string, 0, len(fieldPaths()))
	for _, path := range fieldPaths() {
		paths = append(paths, string(path))
	}
	reasons := make([]string, 0, len(fieldReasons()))
	for _, reason := range fieldReasons() {
		reasons = append(reasons, string(reason))
	}

	for _, group := range [][]string{codes, paths, reasons} {
		seen := make(map[string]struct{}, len(group))
		for _, entry := range group {
			if !tokenPattern.MatchString(entry) {
				t.Errorf("%q is not a lowercase machine token", entry)
			}
			if strings.Contains(entry, " ") {
				t.Errorf("%q reads as prose rather than an identifier", entry)
			}
			if _, duplicate := seen[entry]; duplicate {
				t.Errorf("%q appears twice in its vocabulary", entry)
			}
			seen[entry] = struct{}{}
			for _, needle := range forbidden {
				// internal.unexpected is the deliberate exception: it is the
				// only entry that may accompany a 500, and it says nothing.
				if entry == string(codeInternalUnexpected) {
					continue
				}
				if strings.Contains(entry, needle) {
					t.Errorf("%q contains %q, which discloses implementation detail", entry, needle)
				}
			}
		}
	}

	// Drift: the Go vocabulary and the published contract must agree, entry for
	// entry, the way the audit action vocabulary agrees with its schema.
	for _, test := range []struct {
		schema string
		want   []string
	}{
		{"ErrorCode", codes},
		{"FieldPath", paths},
		{"FieldErrorCode", reasons},
	} {
		published := openAPIEnum(t, test.schema)
		if len(published) != len(test.want) {
			t.Fatalf("%s: contract has %d entries, Go has %d", test.schema, len(published), len(test.want))
		}
		contract := make(map[string]struct{}, len(published))
		for _, entry := range published {
			contract[entry] = struct{}{}
		}
		for _, entry := range test.want {
			if _, ok := contract[entry]; !ok {
				t.Errorf("%s: %q is in Go but not in openapi/guardian.yaml", test.schema, entry)
			}
		}
	}

	// Every field path a domain can attribute must exist in the wire
	// vocabulary. A domain field outside it would be silently dropped, which
	// would hide a rejection reason rather than surface it.
	for _, field := range deception.Fields() {
		if !fieldPath(field).valid() {
			t.Errorf("deception field %q has no wire vocabulary entry", field)
		}
	}
	for _, reason := range deception.Reasons() {
		if !fieldReason(reason).valid() {
			t.Errorf("deception reason %q has no wire vocabulary entry", reason)
		}
	}
	for _, field := range environment.Fields() {
		if !fieldPath(field).valid() {
			t.Errorf("environment field %q has no wire vocabulary entry", field)
		}
	}
	for _, reason := range environment.Reasons() {
		if !fieldReason(reason).valid() {
			t.Errorf("environment reason %q has no wire vocabulary entry", reason)
		}
	}
}

// TestNoHandlerEmitsOperatorFacingProse reads the package source and checks
// that every status slug and every assembled code is a literal machine token.
// A computed or formatted argument is how prose would first appear, so the
// check is on the call sites rather than on one response.
func TestNoHandlerEmitsOperatorFacingProse(t *testing.T) {
	tokenPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	suffixPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	fileSet := token.NewFileSet()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	inspected := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, source, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		{
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, argument := "", -1
				switch function := call.Fun.(type) {
				case *ast.Ident:
					name, argument = function.Name, 2
				case *ast.SelectorExpr:
					name, argument = function.Sel.Name, 3
				}
				switch name {
				case "writeStatus":
					// writeStatus(writer, code, status)
				case "writeAuthError":
					// writeAuthError(writer, err, denial)
					argument = 2
				case "writeError":
					// s.writeError(writer, request, code, status, detail)
				case "code":
					// resource.code(suffix)
					if len(call.Args) != 1 {
						return true
					}
					literal, ok := call.Args[0].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						t.Errorf("%s: resource code suffix is not a string literal",
							fileSet.Position(call.Pos()))
						return true
					}
					inspected++
					if value := strings.Trim(literal.Value, `"`); !suffixPattern.MatchString(value) {
						t.Errorf("%s: code suffix %q is not a machine token",
							fileSet.Position(call.Pos()), value)
					}
					return true
				default:
					return true
				}
				if len(call.Args) <= argument {
					return true
				}
				// One forwarding hop is allowed, and only one: writeAuthError
				// takes a denial slug and hands it to writeStatus unchanged.
				// Every writeAuthError call site is checked above, so the
				// literal behind this identifier is validated there and the
				// chain is closed.
				if identifier, ok := call.Args[argument].(*ast.Ident); ok && identifier.Name == "denial" {
					return true
				}
				literal, ok := call.Args[argument].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Errorf("%s: %s status argument is not a string literal, so it could carry prose",
						fileSet.Position(call.Pos()), name)
					return true
				}
				inspected++
				if value := strings.Trim(literal.Value, `"`); !tokenPattern.MatchString(value) {
					t.Errorf("%s: status %q is not a lowercase machine token",
						fileSet.Position(call.Pos()), value)
				}
				return true
			})
		}
	}
	if inspected < 20 {
		t.Fatalf("inspected only %d call sites; the scan is not reaching the handlers", inspected)
	}
}

// contractCase is one migrated failure and the exact contract it must produce.
type contractCase struct {
	name       string
	statusCode int
	status     string
	code       string
	fields     [][2]string
	run        func(*testing.T) *httptest.ResponseRecorder
}

func contractCases(t *testing.T) []contractCase {
	t.Helper()
	decoyBase := "/v1/environments/" + testEnvironmentID + "/decoys"
	zoneBase := "/v1/environments/" + testEnvironmentID + "/zones"
	validDecoy := `{"zone_id":"` + testZoneID + `","display_name":"Finance file server","family":"smb",` +
		`"persona":"windows_file_service_host","address":"10.20.0.40",` +
		`"pack":"smb-fileshare","pack_version":"0.1.0"}`

	// A service that fails one way, used where the failure is a storage
	// outcome rather than an input the domain can reject on its own.
	failing := func(err error) *Server {
		return decoyTestServer(decoyServiceStub{
			listDecoys: func(context.Context, string, int32) ([]deception.View, error) { return nil, err },
			getDecoy:   func(context.Context, string, string) (deception.View, error) { return deception.View{}, err },
			createDecoy: func(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error) {
				return deception.Decoy{}, err
			},
			updateDecoy: func(context.Context, string, string, deception.Input, int64, deception.Mutation) (deception.Decoy, error) {
				return deception.Decoy{}, err
			},
			enableDecoy: func(context.Context, string, string, int64, deception.Mutation) (deception.Decoy, error) {
				return deception.Decoy{}, err
			},
			removeDecoy: func(context.Context, string, string, int64, deception.Mutation) error { return err },
		})
	}
	// The real domain services, so the field attribution in each case is the one
	// the production validator produces rather than one a stub asserted.
	decoyService, err := deception.NewService(unreachableDecoyRepository{})
	if err != nil {
		t.Fatal(err)
	}
	realDecoy := decoyTestServer(decoyService)
	environmentService, err := environment.NewService(unreachableEnvironmentRepository{})
	if err != nil {
		t.Fatal(err)
	}
	realEnvironment := environmentTestServer(environmentService)

	environmentFailing := func(err error) *Server {
		return environmentTestServer(environmentServiceStub{
			createEnvironment: func(context.Context, string, environment.Mutation) (environment.Environment, error) {
				return environment.Environment{}, err
			},
			updateEnvironment: func(context.Context, string, string, int64, environment.Mutation) (environment.Environment, error) {
				return environment.Environment{}, err
			},
			createZone: func(context.Context, string, string, string, environment.Mutation) (environment.Zone, error) {
				return environment.Zone{}, err
			},
		})
	}

	post := func(server *Server, path, body, ifMatch string) func(*testing.T) *httptest.ResponseRecorder {
		return sendAs(server, http.MethodPost, path, body, ifMatch)
	}

	return []contractCase{
		{
			name: "decoy display name is out of range", statusCode: 400,
			status: "invalid_request", code: "decoy.request.invalid",
			fields: [][2]string{{"display_name", "out_of_range"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"`+testZoneID+`","display_name":"","family":"smb",`+
					`"persona":"windows_file_service_host","address":"10.20.0.40",`+
					`"pack":"smb-fileshare","pack_version":"0.1.0"}`, ""),
		},
		{
			name: "decoy family is outside the closed set", statusCode: 400,
			status: "invalid_request", code: "decoy.request.invalid",
			fields: [][2]string{{"family", "unsupported"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"`+testZoneID+`","display_name":"n","family":"telnet",`+
					`"persona":"windows_file_service_host","address":"10.20.0.40",`+
					`"pack":"smb-fileshare","pack_version":"0.1.0"}`, ""),
		},
		{
			name: "decoy persona is outside the closed set", statusCode: 400,
			status: "invalid_request", code: "decoy.request.invalid",
			fields: [][2]string{{"persona", "unsupported"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"`+testZoneID+`","display_name":"n","family":"smb",`+
					`"persona":"chief_executive","address":"10.20.0.40",`+
					`"pack":"smb-fileshare","pack_version":"0.1.0"}`, ""),
		},
		{
			name: "decoy address is not private", statusCode: 400,
			status: "invalid_request", code: "decoy.request.invalid",
			fields: [][2]string{{"address", "out_of_range"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"`+testZoneID+`","display_name":"n","family":"smb",`+
					`"persona":"windows_file_service_host","address":"8.8.8.8",`+
					`"pack":"smb-fileshare","pack_version":"0.1.0"}`, ""),
		},
		{
			name: "decoy pack version is not in the index", statusCode: 400,
			status: "unknown_pack", code: "decoy.pack.unknown",
			fields: [][2]string{{"pack_version", "unknown"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"`+testZoneID+`","display_name":"n","family":"smb",`+
					`"persona":"windows_file_service_host","address":"10.20.0.40",`+
					`"pack":"smb-fileshare","pack_version":"9.9.9"}`, ""),
		},
		{
			name: "decoy zone reference does not exist", statusCode: 404,
			status: "not_found", code: "decoy.zone.not_found",
			fields: [][2]string{{"zone_id", "unknown"}},
			run: post(realDecoy, decoyBase,
				`{"zone_id":"not-a-uuid","display_name":"n","family":"smb",`+
					`"persona":"windows_file_service_host","address":"10.20.0.40",`+
					`"pack":"smb-fileshare","pack_version":"0.1.0"}`, ""),
		},
		{
			name: "decoy address is outside its zone", statusCode: 400,
			status: "address_outside_zone", code: "decoy.address.outside_zone",
			fields: [][2]string{{"address", "outside_zone"}},
			run:    post(failing(deception.ErrAddressOutsideZone), decoyBase, validDecoy, ""),
		},
		{
			name: "decoy name conflicts", statusCode: 409,
			status: "name_conflict", code: "decoy.display_name.conflicting",
			fields: [][2]string{{"display_name", "conflicting"}},
			run:    post(failing(deception.ErrNameConflict), decoyBase, validDecoy, ""),
		},
		{
			name: "decoy address conflicts", statusCode: 409,
			status: "address_conflict", code: "decoy.address.conflicting",
			fields: [][2]string{{"address", "conflicting"}},
			run:    post(failing(deception.ErrAddressConflict), decoyBase, validDecoy, ""),
		},
		{
			// No field is wrong, so none is named.
			name: "decoy budget is exhausted", statusCode: 409,
			status: "decoy_budget_exhausted", code: "decoy.budget.exhausted",
			run: post(failing(deception.ErrDecoyBudgetExhausted), decoyBase, validDecoy, ""),
		},
		{
			name: "decoy does not exist", statusCode: 404,
			status: "not_found", code: "decoy.not_found",
			run: sendAs(failing(deception.ErrNotFound), http.MethodGet, decoyBase+"/"+testDecoyID, "", ""),
		},
		{
			name: "decoy revision is stale", statusCode: 412,
			status: "precondition_failed", code: "decoy.revision.stale",
			run: sendAs(failing(deception.ErrPreconditionFailed), http.MethodPost,
				decoyBase+"/"+testDecoyID+"/enable", "", `"3"`),
		},
		{
			name: "decoy revision is missing", statusCode: 428,
			status: "precondition_required", code: "decoy.revision.required",
			run: sendAs(failing(deception.ErrNotFound), http.MethodDelete, decoyBase+"/"+testDecoyID, "", ""),
		},
		{
			name: "decoy service is unavailable", statusCode: 503,
			status: "decoy_service_unavailable", code: "decoy.unavailable",
			run: sendAs(decoyTestServer(nil), http.MethodGet, decoyBase, "", ""),
		},
		{
			name: "decoy request carries an unknown key", statusCode: 400,
			status: "invalid_request", code: "decoy.request.invalid",
			run: post(realDecoy, decoyBase, `{"unexpected":"x"}`, ""),
		},
		{
			name: "zone cidr is malformed", statusCode: 400,
			status: "invalid_request", code: "zone.request.invalid",
			fields: [][2]string{{"cidr", "malformed"}},
			run:    post(realEnvironment, zoneBase, `{"display_name":"Servers","cidr":"10.20.0.0/33"}`, ""),
		},
		{
			name: "zone cidr is outside RFC1918", statusCode: 400,
			status: "invalid_request", code: "zone.request.invalid",
			fields: [][2]string{{"cidr", "out_of_range"}},
			run:    post(realEnvironment, zoneBase, `{"display_name":"Servers","cidr":"8.8.0.0/16"}`, ""),
		},
		{
			name: "zone name is empty", statusCode: 400,
			status: "invalid_request", code: "zone.request.invalid",
			fields: [][2]string{{"display_name", "out_of_range"}},
			run:    post(realEnvironment, zoneBase, `{"display_name":"  ","cidr":"10.20.0.0/24"}`, ""),
		},
		{
			name: "zone cidr overlaps an existing zone", statusCode: 409,
			status: "cidr_conflict", code: "zone.cidr.overlapping",
			fields: [][2]string{{"cidr", "conflicting"}},
			run: post(environmentFailing(environment.ErrCIDRConflict), zoneBase,
				`{"display_name":"Servers","cidr":"10.20.0.0/24"}`, ""),
		},
		{
			name: "zone name conflicts", statusCode: 409,
			status: "name_conflict", code: "zone.display_name.conflicting",
			fields: [][2]string{{"display_name", "conflicting"}},
			run: post(environmentFailing(environment.ErrNameConflict), zoneBase,
				`{"display_name":"Servers","cidr":"10.20.0.0/24"}`, ""),
		},
		{
			name: "zone revision is missing", statusCode: 428,
			status: "precondition_required", code: "zone.revision.required",
			run: sendAs(environmentFailing(environment.ErrNotFound), http.MethodDelete,
				zoneBase+"/"+testZoneID, "", ""),
		},
		{
			name: "environment name conflicts", statusCode: 409,
			status: "name_conflict", code: "environment.display_name.conflicting",
			fields: [][2]string{{"display_name", "conflicting"}},
			run: post(environmentFailing(environment.ErrNameConflict), "/v1/environments",
				`{"display_name":"Production"}`, ""),
		},
		{
			name: "environment name is out of range", statusCode: 400,
			status: "invalid_request", code: "environment.request.invalid",
			fields: [][2]string{{"display_name", "out_of_range"}},
			run:    post(realEnvironment, "/v1/environments", `{"display_name":""}`, ""),
		},
		{
			name: "environment revision is stale", statusCode: 412,
			status: "precondition_failed", code: "environment.revision.stale",
			run: sendAs(environmentFailing(environment.ErrPreconditionFailed), http.MethodPatch,
				"/v1/environments/"+testEnvironmentID, `{"display_name":"Production"}`, `"4"`),
		},
		{
			name: "environment revision is missing", statusCode: 428,
			status: "precondition_required", code: "environment.revision.required",
			run: sendAs(environmentFailing(environment.ErrNotFound), http.MethodPatch,
				"/v1/environments/"+testEnvironmentID, `{"display_name":"Production"}`, ""),
		},
		{
			name: "environment list limit is invalid", statusCode: 400,
			status: "invalid_request", code: "environment.request.invalid",
			run: sendAs(environmentFailing(environment.ErrNotFound), http.MethodGet,
				"/v1/environments?limit=201", "", ""),
		},
		{
			name: "environment service is unavailable", statusCode: 503,
			status: "environment_service_unavailable", code: "environment.unavailable",
			run: sendAs(environmentTestServer(nil), http.MethodGet, "/v1/environments", "", ""),
		},
		{
			// An unexpected failure says nothing about what failed.
			name: "unexpected failure", statusCode: 500,
			status: "internal_error", code: "internal.unexpected",
			run: sendAs(failing(errUnexpectedForTest), http.MethodGet, decoyBase, "", ""),
		},
	}
}

func sendAs(server *Server, method, path, body, ifMatch string) func(*testing.T) *httptest.ResponseRecorder {
	return func(t *testing.T) *httptest.ResponseRecorder {
		t.Helper()
		request := authenticatedEnvironmentRequest(t, method, path, body, method != http.MethodGet)
		if ifMatch != "" {
			request.Header.Set("If-Match", ifMatch)
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}
}

func decodeErrorBody(t *testing.T, response *httptest.ResponseRecorder) observedErrorBody {
	t.Helper()
	var body observedErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
	return body
}

// assertNoProse walks the decoded body and fails on any key that could hold
// operator-facing text, at any depth.
func assertNoProse(t *testing.T, payload []byte) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode %q: %v", payload, err)
	}
	var walk func(any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				for _, forbidden := range proseKeys {
					if strings.EqualFold(key, forbidden) {
						t.Errorf("error body carries a prose key %q", key)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(decoded)
}

// openAPIEnum reads one enum block out of the canonical contract. The contract
// sits outside this Go module, so it is read from disk exactly as the pack
// index is in TestPackIndexMatchesCanonicalFile.
func openAPIEnum(t *testing.T, schema string) []string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "openapi", "guardian.yaml")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("read canonical contract: %v", err)
	}
	defer file.Close()

	var values []string
	inSchema, inEnum := false, false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \r")
		switch {
		case line == "    "+schema+":":
			inSchema = true
		case inSchema && inEnum && strings.HasPrefix(line, "        - "):
			values = append(values, strings.TrimPrefix(line, "        - "))
		case inSchema && line == "      enum:":
			inEnum = true
		case inSchema && inEnum && !strings.HasPrefix(line, "        "):
			// The enum block ended, and with it this schema.
			inSchema, inEnum = false, false
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan canonical contract: %v", err)
	}
	if len(values) == 0 {
		t.Fatalf("schema %s has no enum in openapi/guardian.yaml", schema)
	}
	return values
}
