package main

import (
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

	outPath := ""
	if len(os.Args) > 1 {
		outPath = os.Args[1]
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(root, outPath)
		}
	}

	if err := buildlib.Build(root, outPath); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
