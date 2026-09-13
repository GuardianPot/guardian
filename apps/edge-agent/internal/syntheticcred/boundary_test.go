package syntheticcred

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

/*
TestThePackageReachesNothing holds the package to being a pure function.

It is imported by the privileged helper and by the unprivileged Edge Agent, so
anything it imports both of them get. The allowlist is closed rather than a list
of forbidden packages: a recogniser that grew a file read, a network call, or a
logger would be a new way for a definition or an offered value to leave the
comparison, and it has to be added here by name to do it.
*/
func TestThePackageReachesNothing(t *testing.T) {
	allowed := map[string]struct{}{
		"crypto/sha256": {}, "crypto/subtle": {}, "encoding/base64": {}, "encoding/hex": {},
		"errors": {}, "fmt": {}, "strings": {}, "unicode": {}, "unicode/utf8": {},
	}
	// Named as well, so the reason survives a future edit to the allowlist.
	forbidden := map[string]string{
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity":   "recognition must never hold device identity",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/devicepki":  "recognition must never reach device PKI",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/privclient": "the Edge Agent's half must never hold the privileged-helper client",
		"github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged": "the helper imports this package, not the reverse",
	}
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", name, err)
			}
			for prefix, reason := range forbidden {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					t.Errorf("%s imports forbidden %s: %s", name, importPath, reason)
				}
			}
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			if _, ok := allowed[importPath]; !ok {
				t.Errorf("%s imports %s, which is outside the closed allowlist", name, importPath)
			}
		}
	}
}
