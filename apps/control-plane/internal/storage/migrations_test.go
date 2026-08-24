package storage

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/migrations"
)

func TestEmbeddedMigrationsAreForwardOnly(t *testing.T) {
	paths, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no embedded migrations found")
	}
	if len(paths) != int(migrations.LatestVersion) {
		t.Fatalf("embedded migration count = %d, latest version = %d", len(paths), migrations.LatestVersion)
	}
	for _, path := range paths {
		content, err := fs.ReadFile(migrations.Files, path)
		if err != nil {
			t.Fatal(err)
		}
		lower := bytes.ToLower(content)
		if !bytes.Contains(lower, []byte("-- +goose up")) {
			t.Fatalf("migration %s has no explicit up section", path)
		}
		if bytes.Contains(lower, []byte("-- +goose down")) {
			t.Fatalf("migration %s contains a forbidden down migration", path)
		}
		versionText := strings.SplitN(filepath.Base(path), "_", 2)[0]
		version, err := strconv.ParseInt(versionText, 10, 64)
		if err != nil {
			t.Fatalf("migration %s has invalid version prefix: %v", path, err)
		}
		if version < 1 || version > migrations.LatestVersion {
			t.Fatalf("migration %s version %d is outside 1..%d", path, version, migrations.LatestVersion)
		}
	}
}
