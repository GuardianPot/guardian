package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestVersionIsTheOnlyUnprivilegedOperation(t *testing.T) {
	previous := version
	version = "test-version"
	defer func() { version = previous }()
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"--version"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "test-version\n" {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestParseInvocationAcceptsOnlyTypedRootPolicy(t *testing.T) {
	parsed, err := parseInvocation([]string{
		"--allow-interface", "guardian0",
		"--allow-namespace", "guardian-decoy-a",
		"--allow-workload", "guardian-workload-a",
		"--allow-address-range", "192.0.2.0/24",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.interfaces) != 1 || len(parsed.namespaces) != 1 || len(parsed.workloads) != 1 || len(parsed.addressRanges) != 1 {
		t.Fatalf("parsed policy = %+v", parsed)
	}
	for _, args := range [][]string{
		{"shell", "id"},
		{"--version", "--allow-interface", "guardian0"},
		{"--unknown"},
	} {
		if _, err := parseInvocation(args); err == nil {
			t.Fatalf("unsafe/unknown invocation accepted: %s", strings.Join(args, " "))
		}
	}
}
