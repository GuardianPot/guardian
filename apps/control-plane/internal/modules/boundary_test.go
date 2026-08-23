// Package modules_test enforces the Control Plane module-to-database boundary.
package modules_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var forbiddenDataImports = []string{
	"database/sql",
	"github.com/GuardianPot/guardian/apps/control-plane/internal/database",
	"github.com/jackc/pgx",
}

func TestModulesCannotImportDataAccessPackages(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve module test location")
	}
	moduleRoot := filepath.Dir(currentFile)

	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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

	if t.Failed() {
		t.Log("modules must use public module APIs or domain events; database adapters belong outside module packages")
	}
}
