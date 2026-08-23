package components

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEdgePrivilegeAndStorageBoundaries(t *testing.T) {
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	forbiddenEverywhere := map[string]string{
		"os/exec":                          "the main daemon must not execute shell or child commands",
		"github.com/containerd/containerd": "container runtime authority belongs behind P1-W8",
		"github.com/docker/docker":         "the main daemon must not receive Docker authority",
		"github.com/vishvananda/netlink":   "network mutation belongs behind P1-W8",
	}
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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
		relative, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			t.Errorf("relativize %s: %v", path, err)
			return nil
		}
		inStorage := relative == "internal/storage" || strings.HasPrefix(relative, "internal"+string(filepath.Separator)+"storage"+string(filepath.Separator))
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("unquote import in %s: %v", relative, err)
				continue
			}
			for prefix, reason := range forbiddenEverywhere {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					t.Errorf("%s imports forbidden %s: %s", relative, importPath, reason)
				}
			}
			if !inStorage && (importPath == "database/sql" || importPath == "modernc.org/sqlite" || strings.HasPrefix(importPath, "modernc.org/sqlite/")) {
				t.Errorf("%s bypasses the internal storage boundary with %s", relative, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
