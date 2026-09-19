package cgoguard

import (
	"path/filepath"
	"testing"
)

func TestFileUsesCgo(t *testing.T) {
	cases := map[string]bool{
		"uses_import_c.go.txt": true,
		"uses_export.go.txt":   true,
		"uses_cgo_tag.go.txt":  true,
		"clean.go.txt":         false,
	}
	for name, want := range cases {
		got, err := FileUsesCgo(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestRepositoryHasNoCgo(t *testing.T) {
	found, err := Find(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("cgo usage detected in: %v", found)
	}
}
