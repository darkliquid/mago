package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/mago/internal/buildlib"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		exit(err)
	}

	version := flag.String("version", "", "miniaudio version to download before building (defaults to the vendored version in zz_generated.bindings.go)")
	all := flag.Bool("all", false, "build all supported target platforms")
	targetFlag := flag.String("target", "", "build specific target (e.g. linux/amd64)")
	downloadOnly := flag.String("download-only", "", "download miniaudio.h to target destination path and exit")
	flag.Parse()

	modesSet := 0
	if *all {
		modesSet++
	}
	if *targetFlag != "" {
		modesSet++
	}
	if *downloadOnly != "" {
		modesSet++
	}
	if modesSet > 1 {
		exit(fmt.Errorf("cannot specify more than one of -all, -target, and -download-only"))
	}

	args := flag.Args()
	if len(args) > 1 {
		exit(fmt.Errorf("usage: buildlib [-version x.y.z] [-all | -target os/arch | -download-only path] [output-path]"))
	}

	outPath := ""
	if len(args) == 1 {
		outPath = args[0]
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(root, outPath)
		}
	}

	if *downloadOnly != "" {
		if outPath != "" {
			exit(fmt.Errorf("positional output path cannot be used with -download-only"))
		}
		ver, err := buildlib.ResolveMiniaudioVersion(root, *version)
		if err != nil {
			exit(err)
		}
		dst := *downloadOnly
		if !filepath.IsAbs(dst) {
			dst = filepath.Join(root, dst)
		}
		if err := buildlib.DownloadMiniaudioHeader(ver, dst); err != nil {
			exit(err)
		}
		fmt.Printf("Downloaded miniaudio.h (version %s) to %s\n", ver, dst)
		return
	}

	if *all {
		if outPath != "" {
			exit(fmt.Errorf("positional output path cannot be used with -all"))
		}
		if err := buildlib.BuildAll(root, *version); err != nil {
			exit(err)
		}
		return
	}

	if *targetFlag != "" {
		parts := strings.Split(*targetFlag, "/")
		if len(parts) != 2 {
			exit(fmt.Errorf("invalid target format %q, expected os/arch (e.g. linux/amd64)", *targetFlag))
		}
		target, err := buildlib.FindTarget(parts[0], parts[1])
		if err != nil {
			exit(err)
		}
		if outPath != "" {
			if err := buildlib.BuildTargetWithOutput(root, target, outPath, *version); err != nil {
				exit(err)
			}
		} else {
			if err := buildlib.BuildTarget(root, target, *version); err != nil {
				exit(err)
			}
		}
		return
	}

	if err := buildlib.Build(root, outPath, *version); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
