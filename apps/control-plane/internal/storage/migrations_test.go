package storage

import (
	"bytes"
	"io/fs"
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
	}
}
