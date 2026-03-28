package buildlib

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
)

func DefaultLibraryFilename(goos string) string {
	switch goos {
	case "windows":
		return "miniaudio.dll"
	case "darwin":
		return "libminiaudio.dylib"
	default:
		return "libminiaudio.so"
	}
}

func DefaultOutputPath(root string) string {
	return filepath.Join(root, "native", DefaultLibraryFilename(runtime.GOOS))
}

func Build(root, outPath string) error {
	if outPath == "" {
		outPath = DefaultOutputPath(root)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	compiler := os.Getenv("CC")
	if compiler == "" {
		compiler = "cc"
	}
	switch path.Base(compiler) {
	case "cc":
		compiler = "cc"
	case "gcc":
		compiler = "gcc"
	case "clang":
		compiler = "clang"
	case "clang-cl":
		compiler = "clang-cl"
	default:
		return fmt.Errorf("unsupported C compiler %q", compiler)
	}

	source := filepath.Join(root, "native", "miniaudio_bridge.c")
	args, err := compilerArgs(outPath, source)
	if err != nil {
		return err
	}

	cmd := exec.Command(compiler, args...) // #nosec G204 -- compiler is restricted to an allowlist above.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func compilerArgs(outPath, source string) ([]string, error) {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-shared",
			"-Wl,-soname," + filepath.Base(outPath),
			"-o", outPath, source,
			"-ldl", "-lm", "-lpthread",
		}, nil
	case "darwin":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-dynamiclib",
			"-o", outPath, source,
			"-framework", "CoreAudio",
			"-framework", "AudioToolbox",
			"-framework", "AudioUnit",
			"-framework", "Foundation",
			"-framework", "CoreFoundation",
			"-framework", "CoreServices",
			"-lm",
		}, nil
	case "windows":
		return []string{
			"-std=c11", "-O2", "-shared",
			"-o", outPath, source,
			"-lwinmm", "-lole32", "-luuid",
		}, nil
	case "freebsd", "netbsd":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-shared",
			"-o", outPath, source,
			"-lm", "-lpthread",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported GOOS for shared library build: %s", runtime.GOOS)
	}
}
