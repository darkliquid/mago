package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darkliquid/mago/internal/buildlib"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		exit(err)
	}

	version := flag.String("version", "", "miniaudio version to download before building (defaults to the vendored version in zz_generated.bindings.go)")
	flag.Parse()

	outPath := ""
	args := flag.Args()
	if len(args) > 1 {
		exit(fmt.Errorf("usage: buildlib [-version x.y.z] [output-path]"))
	}
	if len(args) == 1 {
		outPath = args[0]
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(root, outPath)
		}
	}

	if err := buildlib.Build(root, outPath, *version); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
