// Package cgoguard detects accidental cgo usage. mago is a zero-CGO project:
// the native library is a prebuilt runtime artifact loaded with purego, and no
// Go source in this repository may use cgo.
package cgoguard

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	importCRe = regexp.MustCompile(`(?m)^\s*(?:import\s+)?"C"\s*$`)
	exportRe  = regexp.MustCompile(`(?m)^//export\s+\w+`)
	cgoTagRe  = regexp.MustCompile(`(?m)^//go:build[^\n]*\bcgo\b`)
)

// FileUsesCgo reports whether the Go file at path uses cgo, by C import,
// //export directive, or cgo build constraint.
func FileUsesCgo(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	if f, err := parser.ParseFile(fset, path, data, parser.ImportsOnly); err == nil {
		for _, imp := range f.Imports {
			if imp.Path.Value == `"C"` {
				return true, nil
			}
		}
	}

	return importCRe.Match(data) || exportRe.Match(data) || cgoTagRe.Match(data), nil
}

// Find walks root and returns the paths of every Go file that uses cgo.
func Find(root string) ([]string, error) {
	var found []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".beads", ".dolt", ".claude", ".cursor", ".agents",
				".codex", "testdata", "node_modules":
				return fs.SkipDir
			}
			if strings.HasSuffix(filepath.ToSlash(path), "internal/lib") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		uses, err := FileUsesCgo(path)
		if err != nil {
			return err
		}
		if uses {
			found = append(found, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return found, nil
}
