package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/config"
)

func emptyLookup(string) (string, bool) { return "", false }

func TestVersionCommand(t *testing.T) {
	previous := version
	version = "test-version"
	t.Cleanup(func() { version = previous })

	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"version"}, emptyLookup, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run(version) error = %v", err)
	}
	if stdout.String() != "test-version\n" {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestServeRequiresDatabaseConfiguration(t *testing.T) {
	err := run(context.Background(), []string{"serve"}, config.LookupEnv(emptyLookup), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "GUARDIAN_DATABASE_URL") {
		t.Fatalf("run(serve) error = %v, want missing database configuration", err)
	}
}

func TestMigrateErrorDoesNotLeakDatabasePassword(t *testing.T) {
	secret := "guardian-fake-secret"
	lookup := func(key string) (string, bool) {
		if key == "GUARDIAN_DATABASE_URL" {
			return "postgres://guardian:" + secret + "@%zz", true
		}
		return "", false
	}

	err := run(context.Background(), []string{"migrate"}, lookup, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run(migrate) unexpectedly accepted an invalid database URL")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("migration error leaked the database password")
	}
}
