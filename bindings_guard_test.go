package mago

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bindingAllowlist names bindingSet fields that are registered but intentionally
// not referenced by Go code yet. Each entry must justify why it is allowed to
// be dead so the list cannot grow silently.
var bindingAllowlist = map[string]string{
	"maRBGetSubbufferSize":      "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maRBGetSubbufferStride":    "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maRBGetSubbufferOffset":    "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maRBGetSubbufferPtr":       "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maPCMRBGetSubbufferSize":   "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maPCMRBGetSubbufferStride": "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maPCMRBGetSubbufferOffset": "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maPCMRBGetSubbufferPtr":    "unused ring-buffer subbuffer accessor; see mago-8a3.15.3.3",
	"maNoiseSetType":            "decide Noise.SetType stub and binding; see mago-8a3.15.2.3",
}

var (
	bindingFieldRe = regexp.MustCompile(`(?m)^\t(\w+)\s+func`)
	bindingUseRe   = regexp.MustCompile(`\.bindings\.`)
)

func TestNoDeadBindings(t *testing.T) {
	fields, err := bindingFields()
	if err != nil {
		t.Fatalf("parse bindings: %v", err)
	}

	used := make(map[string]bool, len(fields))
	walkErr := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "internal" {
				if filepath.ToSlash(path) == "internal/lib" {
					return fs.SkipDir
				}
				return nil
			}
			switch d.Name() {
			case ".git", ".beads", ".dolt", ".claude", ".cursor", ".agents",
				".codex", "testdata", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasPrefix(path, "zz_generated.") ||
			path == "bindings_guard_test.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bindingUseRe.Match(data) {
			return nil
		}
		for _, f := range fields {
			if bindingFieldUseRe(f).Match(data) {
				used[f] = true
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}

	for _, f := range fields {
		if used[f] {
			continue
		}
		if reason, ok := bindingAllowlist[f]; ok {
			t.Logf("allowlisted dead binding %s: %s", f, reason)
			continue
		}
		t.Errorf("binding %s is registered but never used; use it or allowlist it with a reason", f)
	}
}

// bindingFieldUseRe returns a regexp that matches a reference to the binding
// field f as it appears at call sites, e.g. lib.bindings.maDeviceStart(...).
func bindingFieldUseRe(f string) *regexp.Regexp {
	return regexp.MustCompile(`\.bindings\.` + regexp.QuoteMeta(f) + `\b`)
}

func bindingFields() ([]string, error) {
	data, err := os.ReadFile("zz_generated.bindings.go")
	if err != nil {
		return nil, err
	}
	start := strings.Index(string(data), "type bindingSet struct {")
	if start < 0 {
		return nil, os.ErrNotExist
	}
	end := strings.Index(string(data[start:]), "\n}")
	if end < 0 {
		return nil, os.ErrNotExist
	}
	block := string(data[start : start+end])
	matches := bindingFieldRe.FindAllStringSubmatch(block, -1)
	fields := make([]string, 0, len(matches))
	for _, m := range matches {
		fields = append(fields, m[1])
	}
	return fields, nil
}
