// Package app_test enforces Control Plane domain-to-storage boundaries.
package app_test

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var domainPackageRoots = []string{
	"api",
	"audit",
	"auth",
	"deception",
	"devices",
	"environment",
	"health",
	"jobs",
}

var forbiddenDataImports = []string{
	"database/sql",
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage",
	"github.com/jackc/pgx",
}

func TestDomainPackagesCannotImportDataAccessPackages(t *testing.T) {
	internalRoot := controlPlaneInternalRoot(t)
	for _, packageRoot := range domainPackageRoots {
		packageRoot := packageRoot
		t.Run(packageRoot, func(t *testing.T) {
			root := filepath.Join(internalRoot, packageRoot)
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
					return nil
				}
				parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}
				for _, imported := range parsed.Imports {
					importPath, err := strconv.Unquote(imported.Path.Value)
					if err != nil {
						return err
					}
					for _, forbidden := range forbiddenDataImports {
						if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
							t.Errorf("%s directly imports forbidden data-access package %q", path, importPath)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	if t.Failed() {
		t.Log("domain packages must use public module APIs; PostgreSQL adapters belong under internal/storage")
	}
}

func TestLegacyPackageRootsAreAbsent(t *testing.T) {
	internalRoot := controlPlaneInternalRoot(t)
	for _, legacy := range []string{"database", "httpapi", "modules"} {
		_, err := os.Stat(filepath.Join(internalRoot, legacy))
		if err == nil {
			t.Errorf("legacy package root %q still exists", legacy)
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect legacy package root %q: %v", legacy, err)
		}
	}
}

func controlPlaneInternalRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve boundary test location")
	}
	return filepath.Dir(filepath.Dir(currentFile))
}
