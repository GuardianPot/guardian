package decoy

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestDecoyPackageHoldsNoIdentityOrPrivilege is AC-SEC-002 in the direction
// this package can prove at build time. A decoy must never be able to reach
// device identity or a privileged operation, and a package that cannot import
// those cannot accidentally connect them.
//
// The runtime direction of AC-SEC-002 — that a running decoy container cannot
// reach the runtime socket — belongs to P2-W3, which owns the container
// lifecycle. This check is the half that is provable without one.
func TestDecoyPackageHoldsNoIdentityOrPrivilege(t *testing.T) {
	forbidden := map[string]string{
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity":   "a decoy must never hold device identity",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/devicepki":  "a decoy must never reach device PKI",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged": "a decoy must never reach a privileged operation",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/privclient": "a decoy must never hold the privileged-helper client",
		"os/exec":                        "a decoy manager must not execute a child process",
		"github.com/vishvananda/netlink": "network mutation belongs behind the privileged helper",
		"crypto/tls":                     "a decoy manager terminates no TLS and holds no key material",
	}
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("unquote import in %s: %v", path, err)
				continue
			}
			for prefix, reason := range forbidden {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					t.Errorf("%s imports forbidden %s: %s", path, importPath, reason)
				}
			}
			if strings.HasPrefix(importPath, "github.com/containerd/") {
				t.Errorf("%s imports containerd; the container lifecycle belongs to P2-W3", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
