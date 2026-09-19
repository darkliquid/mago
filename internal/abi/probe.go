// Package abi validates that Go's mirrors of miniaudio structs match the
// vendored header on the current platform. It is test tooling only and is
// never part of the shipped bridge.
package abi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/darkliquid/mago/internal/buildlib"
)

// compilerOverride, when set, replaces "zig" with the given compiler. Tests use
// it to avoid requiring zig.
var compilerOverride string

// Parse reads probe output of the form "key value" per line.
func Parse(output string) (map[string]uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed probe line %q", line)
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed probe value in %q: %w", line, err)
		}
		values[fields[0]] = value
	}
	return values, nil
}

// Run compiles and executes the layout probe against the vendored miniaudio.h
// in root and returns its measurements.
func Run(root string) (map[string]uint64, error) {
	includeDir, cleanup, err := buildlib.PrepareIncludeDir(root, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tmp, err := os.MkdirTemp("", "mago-abi-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	source := filepath.Join(root, "internal", "abi", "layout_probe.c")
	probe := filepath.Join(tmp, "probe")
	if runtime.GOOS == "windows" {
		probe += ".exe"
	}

	compiler := "zig"
	if compilerOverride != "" {
		compiler = compilerOverride
	}
	build := exec.Command(compiler, "cc", "-std=c11", "-I", includeDir, "-o", probe, source) // #nosec G204
	if out, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("compile layout probe: %w\n%s", err, out)
	}

	out, err := exec.Command(probe).Output() // #nosec G204
	if err != nil {
		return nil, fmt.Errorf("run layout probe: %w", err)
	}
	return Parse(string(out))
}
