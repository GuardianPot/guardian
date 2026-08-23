package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRestartRestoresStateRetryHealthIdentityAndQueue(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.SetIdentity(ctx, IdentityRecord{
		CertificateSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		NotBefore:         now.Add(-time.Hour),
		NotAfter:          now.Add(time.Hour),
		EnrollmentStatus:  "enrolled",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRevision(ctx, RevisionRecord{
		ObjectKind: "edge-config", DesiredRevision: 9, ObservedRevision: 8, LastGoodRevision: 8, ConditionCode: "reconcile-pending",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRetry(ctx, RetryRecord{
		OperationID: "device-channel", Attempts: 3, NextAttempt: now.Add(time.Minute), LastErrorCode: "control-unavailable",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetHealth(ctx, HealthCondition{Name: "local-db", Status: "healthy", ReasonCode: "ready"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enqueue(ctx, "pending-event", []byte("pending")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Identity == nil || snapshot.Identity.EnrollmentStatus != "enrolled" ||
		len(snapshot.Revisions) != 1 || snapshot.Revisions[0].LastGoodRevision != 8 ||
		len(snapshot.Retries) != 1 || snapshot.Retries[0].Attempts != 3 ||
		len(snapshot.Health) != 1 || snapshot.Queue.Pending != 1 {
		t.Fatalf("restart snapshot = %+v", snapshot)
	}
}

func TestSameCertificateCannotClearPersistedRevocationOnRestart(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	record := IdentityRecord{
		CertificateSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		NotBefore:         now.Add(-time.Hour), NotAfter: now.Add(time.Hour), EnrollmentStatus: "revoked",
	}
	if err := store.SetIdentity(ctx, record); err != nil {
		t.Fatal(err)
	}
	record.EnrollmentStatus = "enrolled"
	if err := store.SetIdentity(ctx, record); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Identity == nil || snapshot.Identity.EnrollmentStatus != "revoked" {
		t.Fatalf("same certificate cleared revocation: %+v", snapshot.Identity)
	}

	record.CertificateSHA256 = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	if err := store.SetIdentity(ctx, record); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Identity == nil || snapshot.Identity.EnrollmentStatus != "enrolled" {
		t.Fatalf("new certificate did not establish new identity: %+v", snapshot.Identity)
	}
}

func TestLockContentionReturnsNamedBusyCondition(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{
		DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool"), BusyTimeout: 10 * time.Millisecond,
	}
	first, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.db.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer first.db.ExecContext(context.Background(), "ROLLBACK")
	err = second.SetHealth(ctx, HealthCondition{Name: "local-db", Status: "degraded", ReasonCode: "busy"})
	if !errors.Is(err, ErrDatabaseBusy) {
		t.Fatalf("lock contention error = %v", err)
	}
}

func TestDiskFullSimulationReturnsNamedCondition(t *testing.T) {
	store, _ := openTestStore(t)
	store.beforeWrite = func() error { return syscall.ENOSPC }
	err := store.SetHealth(context.Background(), HealthCondition{Name: "local-db", Status: "failed", ReasonCode: "disk-full"})
	if !errors.Is(err, ErrStorageFull) {
		t.Fatalf("disk-full simulation error = %v", err)
	}
	if _, err := store.Enqueue(context.Background(), "event-full", []byte("payload")); !errors.Is(err, ErrStorageFull) {
		t.Fatalf("disk-full enqueue error = %v", err)
	}
}

func TestCorruptionIsNamedAndNeverSilentlyReset(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "edge.db")
	original := []byte("this is not sqlite and must remain intact")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	options := Options{DatabasePath: path, SpoolDirectory: filepath.Join(root, "spool")}
	if _, err := Open(ctx, options); !errors.Is(err, ErrCorruptDatabase) {
		t.Fatalf("corrupt Open() error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("corrupt database was silently changed: %q", after)
	}
}

func TestExplicitRecoveryQuarantinesOnlyCorruptDevelopmentDatabase(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "edge.db")
	options := Options{DatabasePath: path, SpoolDirectory: filepath.Join(root, "spool")}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 23, 12, 0, 0, 123, time.UTC)
	if _, err := RecoverDevelopmentDatabase(ctx, options, false, now); !errors.Is(err, ErrRecoveryConfirmationRequired) {
		t.Fatalf("unconfirmed recovery error = %v", err)
	}
	report, err := RecoverDevelopmentDatabase(ctx, options, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.QuarantinedPaths) != 1 {
		t.Fatalf("recovery report = %+v", report)
	}
	if _, err := os.Stat(report.QuarantinedPaths[0]); err != nil {
		t.Fatal(err)
	}
	store, err := OpenReadOnly(ctx, options)
	if err != nil {
		t.Fatalf("recovered database open: %v", err)
	}
	store.Close()
	if _, err := RecoverDevelopmentDatabase(ctx, options, true, now.Add(time.Second)); !errors.Is(err, ErrRecoveryNotRequired) {
		t.Fatalf("healthy recovery error = %v", err)
	}
}

func TestUnversionedLegacySchemaRequiresExplicitRecovery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "edge.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE legacy_state (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	options := Options{DatabasePath: path, SpoolDirectory: filepath.Join(root, "spool")}
	if _, err := Open(ctx, options); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("legacy Open() error = %v", err)
	}
}

func TestVersionedSchemaMustExactlyMatchCurrentShape(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DROP INDEX retry_metadata_due_idx"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, options); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("incomplete versioned schema error = %v", err)
	}
}

func TestDatabaseAndDirectoriesUseRestrictivePermissions(t *testing.T) {
	store, options := openTestStore(t)
	if _, err := store.Enqueue(context.Background(), "permission-event", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		options.DatabasePath:               0o600,
		filepath.Dir(options.DatabasePath): 0o700,
		options.SpoolDirectory:             0o700,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s permissions = %o, want %o", path, info.Mode().Perm(), want)
		}
	}
}

func TestOpenRejectsSymlinkedStatePath(t *testing.T) {
	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedDirectory := filepath.Join(root, "linked")
	if err := os.Symlink(realDirectory, linkedDirectory); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), Options{
		DatabasePath:   filepath.Join(linkedDirectory, "edge.db"),
		SpoolDirectory: filepath.Join(root, "spool"),
	})
	if err == nil {
		t.Fatal("symlinked state path unexpectedly opened")
	}
}

func TestSnapshotIsBoundedAndRejectsRawErrorText(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	if err := store.SetRetry(ctx, RetryRecord{
		OperationID: "channel", Attempts: 1, NextAttempt: time.Now(), LastErrorCode: "token secret value",
	}); err == nil {
		t.Fatal("raw retry error text unexpectedly accepted")
	}
	for index := 0; index < maxSnapshotRows+10; index++ {
		if err := store.SetHealth(ctx, HealthCondition{
			Name: fmt.Sprintf("condition-%02d", index), Status: "unknown", ReasonCode: "not-started",
		}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Health) != maxSnapshotRows {
		t.Fatalf("health rows = %d, want %d", len(snapshot.Health), maxSnapshotRows)
	}
}
