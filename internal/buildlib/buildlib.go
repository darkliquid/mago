package buildlib

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Target struct {
	GOOS     string
	GOARCH   string
	Filename string
	Triple   string
}

func (t Target) Key() string {
	return t.GOOS + "-" + t.GOARCH
}

func (t Target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

func AllTargets() []Target {
	return []Target{
		{GOOS: "linux", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-linux-gnu"},
		{GOOS: "linux", GOARCH: "arm64", Filename: "libminiaudio.so", Triple: "aarch64-linux-gnu"},
		{GOOS: "windows", GOARCH: "amd64", Filename: "miniaudio.dll", Triple: "x86_64-windows-gnu"},
		{GOOS: "freebsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-freebsd"},
		{GOOS: "netbsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-netbsd"},
		{GOOS: "darwin", GOARCH: "amd64", Filename: "libminiaudio.dylib", Triple: "x86_64-apple-darwin"},
		{GOOS: "darwin", GOARCH: "arm64", Filename: "libminiaudio.dylib", Triple: "arm64-apple-darwin"},
	}
}

func FindTarget(goos, goarch string) (Target, error) {
	for _, t := range AllTargets() {
		if t.GOOS == goos && t.GOARCH == goarch {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unsupported platform %s/%s", goos, goarch)
}

func CurrentTarget() (Target, error) {
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return FindTarget(goos, goarch)
}

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
	target, err := CurrentTarget()
	if err != nil {
		goos := os.Getenv("GOOS")
		if goos == "" {
			goos = runtime.GOOS
		}
		goarch := os.Getenv("GOARCH")
		if goarch == "" {
			goarch = runtime.GOARCH
		}
		return filepath.Join(root, "internal", "lib", goos+"-"+goarch, DefaultLibraryFilename(goos))
	}
	return filepath.Join(root, "internal", "lib", target.Key(), target.Filename)
}

func ResolveMiniaudioVersion(root, version string) (string, error) {
	if strings.TrimSpace(version) != "" {
		return strings.TrimSpace(version), nil
	}
	return defaultMiniaudioVersion(root)
}

var (
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
	miniaudioHeaderURL = func(version string) string {
		return "https://raw.githubusercontent.com/mackron/miniaudio/" + url.PathEscape(version) + "/miniaudio.h"
	}
	versionConstPattern = regexp.MustCompile(`ExpectedMiniaudioVersion(Major|Minor|Revision)\s+uint32\s*=\s*(\d+)`)
	compilerOverride    string // used in tests to mock compilation without executing zig/osxcross
)

func DownloadMiniaudioHeader(version, dstPath string) (err error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("miniaudio version must not be empty")
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory for miniaudio.h: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, miniaudioHeaderURL(version), nil)
	if err != nil {
		return fmt.Errorf("create miniaudio header request: %w", err)
	}
	req.Header.Set("User-Agent", "mago-buildlib")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download miniaudio.h for version %s: %w", version, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close miniaudio.h response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download miniaudio.h for version %s: unexpected status %s", version, resp.Status)
	}

	tmpPath := dstPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temporary miniaudio header file: %w", err)
	}

	copyErr := error(nil)
	if _, err := io.Copy(file, resp.Body); err != nil {
		copyErr = fmt.Errorf("write miniaudio.h for version %s: %w", version, err)
	}
	if err := file.Close(); err != nil && copyErr == nil {
		copyErr = fmt.Errorf("close temporary miniaudio header file: %w", err)
	}
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install miniaudio.h for version %s: %w", version, err)
	}

	return nil
}

func defaultMiniaudioVersion(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "zz_generated.bindings.go"))
	if err != nil {
		return "", fmt.Errorf("read generated bindings version constants: %w", err)
	}

	parts := map[string]string{}
	for _, match := range versionConstPattern.FindAllStringSubmatch(string(content), -1) {
		parts[match[1]] = match[2]
	}

	major, ok1 := parts["Major"]
	minor, ok2 := parts["Minor"]
	revision, ok3 := parts["Revision"]
	if !ok1 || !ok2 || !ok3 {
		return "", fmt.Errorf("find ExpectedMiniaudioVersion in generated bindings")
	}

	return major + "." + minor + "." + revision, nil
}

func prepareIncludeDir(root, version string) (includeDir string, cleanup func(), err error) {
	rootHeader := filepath.Join(root, "miniaudio.h")
	if info, statErr := os.Stat(rootHeader); statErr == nil && !info.IsDir() {
		// Use existing miniaudio.h from root without re-downloading
		dir, err := os.MkdirTemp("", "mago-buildlib-*")
		if err != nil {
			return "", nil, fmt.Errorf("create temporary include directory: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(dir) }

		src, err := os.Open(rootHeader)
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("open root miniaudio.h: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(filepath.Join(dir, "miniaudio.h"))
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("copy miniaudio.h to include directory: %w", err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			cleanup()
			return "", nil, fmt.Errorf("copy miniaudio.h content: %w", err)
		}
		if err := dst.Close(); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("close copied miniaudio.h: %w", err)
		}
		return dir, cleanup, nil
	}

	resolvedVersion, err := ResolveMiniaudioVersion(root, version)
	if err != nil {
		return "", nil, err
	}

	dir, err := os.MkdirTemp("", "mago-buildlib-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary include directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	headerPath := filepath.Join(dir, "miniaudio.h")
	if err := DownloadMiniaudioHeader(resolvedVersion, headerPath); err != nil {
		cleanup()
		return "", nil, err
	}

	return dir, cleanup, nil
}

func zigCompilerArgs(target Target, outPath, source, includeDir, root string) []string {
	commonFlags := []string{
		"-target", target.Triple,
		"-std=c11", "-O2",
		"-fvisibility=hidden",
		"-fno-asynchronous-unwind-tables",
		"-fno-ident",
		"-ffile-prefix-map=" + root + "=.",
		"-ffile-prefix-map=" + includeDir + "=.",
		"-I", includeDir,
		"-s",
	}

	switch target.GOOS {
	case "linux":
		return append(commonFlags,
			"-fPIC", "-shared",
			"-Wl,-soname," + filepath.Base(outPath),
			"-o", outPath, source,
			"-ldl", "-lm", "-lpthread",
		)
	case "windows":
		return append(commonFlags,
			"-shared",
			"-o", outPath, source,
			"-lwinmm", "-lole32", "-luuid",
		)
	case "freebsd", "netbsd":
		return append(commonFlags,
			"-fPIC", "-shared",
			"-o", outPath, source,
			"-lm", "-lpthread",
		)
	default:
		return nil
	}
}

func darwinCompilerArgs(target Target, root, outPath, source, includeDir string) []string {
	arch := target.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}

	return []string{
		"-arch", arch,
		"-std=c11", "-O2", "-fPIC", "-dynamiclib",
		"-fvisibility=hidden",
		"-ffile-prefix-map=" + root + "=.",
		"-ffile-prefix-map=" + includeDir + "=.",
		"-I", includeDir,
		"-install_name", "@rpath/" + filepath.Base(outPath),
		"-Wl,-x",
		"-o", outPath, source,
		"-framework", "CoreAudio",
		"-framework", "AudioToolbox",
		"-framework", "AudioUnit",
		"-framework", "Foundation",
		"-framework", "CoreFoundation",
		"-framework", "CoreServices",
		"-lm",
	}
}

func BuildTarget(root string, target Target, version string) error {
	outPath := filepath.Join(root, "internal", "lib", target.Key(), target.Filename)
	return BuildTargetWithOutput(root, target, outPath, version)
}

func BuildTargetWithOutput(root string, target Target, outPath, version string) error {
	if _, err := FindTarget(target.GOOS, target.GOARCH); err != nil {
		return fmt.Errorf("unsupported target %s: %w", target, err)
	}

	includeDir, cleanup, err := prepareIncludeDir(root, version)
	if err != nil {
		return err
	}
	defer cleanup()

	return buildTargetWithInclude(root, target, outPath, includeDir)
}

func buildTargetWithInclude(root string, target Target, outPath, includeDir string) error {
	if _, err := FindTarget(target.GOOS, target.GOARCH); err != nil {
		return fmt.Errorf("unsupported target %s: %w", target, err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	source := filepath.Join(root, "native", "miniaudio_bridge.c")

	if compilerOverride != "" {
		args := []string{"-std=c11", "-O2", "-fvisibility=hidden", "-I", includeDir, "-o", outPath, source}
		cmd := exec.Command(compilerOverride, args...) // #nosec G204
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compile %s with %s: %w", target, cmd.Path, err)
		}
		return nil
	}

	if target.GOOS == "darwin" {
		return buildDarwinTarget(target, root, outPath, source, includeDir)
	}

	zigArgs := zigCompilerArgs(target, outPath, source, includeDir, root)
	if len(zigArgs) == 0 {
		return fmt.Errorf("unsupported target %s for zig compilation", target)
	}

	args := append([]string{"cc"}, zigArgs...)
	cmd := exec.Command("zig", args...) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile %s with %s: %w", target, cmd.Path, err)
	}
	return nil
}

func buildDarwinTarget(target Target, root, outPath, source, includeDir string) error {
	if runtime.GOOS == "darwin" {
		// Native macOS compilation
		args := darwinCompilerArgs(target, root, outPath, source, includeDir)
		cmd := exec.Command("clang", args...) // #nosec G204
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Cross-compilation on Linux via osxcross container
	image := os.Getenv("OSXCROSS_IMAGE")
	if image == "" {
		image = "dockercross/osxcross"
	}

	compiler := "o64-clang"
	if target.GOARCH == "arm64" {
		compiler = "oa64-clang"
	}

	args := []string{
		"run", "--rm",
		"-v", root + ":/workspace:ro",
		"-v", includeDir + ":/include:ro",
		"-v", filepath.Dir(outPath) + ":/out",
		image,
		compiler,
		"-std=c11", "-O2", "-fPIC", "-dynamiclib",
		"-fvisibility=hidden",
		"-ffile-prefix-map=/workspace=.",
		"-ffile-prefix-map=/include=.",
		"-I", "/include",
		"-install_name", "@rpath/" + filepath.Base(outPath),
		"-Wl,-x",
		"-o", "/out/" + filepath.Base(outPath),
		"/workspace/native/miniaudio_bridge.c",
		"-framework", "CoreAudio",
		"-framework", "AudioToolbox",
		"-framework", "AudioUnit",
		"-framework", "Foundation",
		"-framework", "CoreFoundation",
		"-framework", "CoreServices",
		"-lm",
	}

	cmd := exec.Command("docker", args...) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cross-compile darwin %s via docker %s: %w", target.GOARCH, image, err)
	}
	return nil
}

func BuildAll(root, version string) error {
	includeDir, cleanup, err := prepareIncludeDir(root, version)
	if err != nil {
		return err
	}
	defer cleanup()

	targets := AllTargets()
	for _, target := range targets {
		fmt.Printf("Building %s (%s)...\n", target, target.Filename)
		outPath := filepath.Join(root, "internal", "lib", target.Key(), target.Filename)
		if err := buildTargetWithInclude(root, target, outPath, includeDir); err != nil {
			return fmt.Errorf("build target %s: %w", target, err)
		}
	}
	return nil
}

func Build(root, outPath, version string) error {
	target, err := CurrentTarget()
	if err != nil {
		return err
	}
	if outPath == "" {
		outPath = DefaultOutputPath(root)
	}
	return BuildTargetWithOutput(root, target, outPath, version)
}
