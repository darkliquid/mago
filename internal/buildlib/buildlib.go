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

var (
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
	miniaudioHeaderURL = func(version string) string {
		return "https://raw.githubusercontent.com/mackron/miniaudio/" + url.PathEscape(version) + "/miniaudio.h"
	}
	versionConstPattern = regexp.MustCompile(`ExpectedMiniaudioVersion(Major|Minor|Revision)\s+uint32\s*=\s*(\d+)`)
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

func Build(root, outPath, version string) (err error) {
	if strings.TrimSpace(version) == "" {
		resolvedVersion, err := defaultMiniaudioVersion(root)
		if err != nil {
			return err
		}
		version = resolvedVersion
	}
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
	switch filepath.Base(compiler) {
	case "cc":
	case "gcc":
	case "clang":
	case "clang-cl":
	default:
		return fmt.Errorf("unsupported C compiler %q", compiler)
	}

	includeDir, err := os.MkdirTemp("", "mago-buildlib-*")
	if err != nil {
		return fmt.Errorf("create temporary include directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(includeDir); cleanupErr != nil && err == nil {
			err = fmt.Errorf("remove temporary include directory: %w", cleanupErr)
		}
	}()

	headerPath := filepath.Join(includeDir, "miniaudio.h")
	if err := downloadMiniaudioHeader(version, headerPath); err != nil {
		return err
	}

	source := filepath.Join(root, "native", "miniaudio_bridge.c")
	args, err := compilerArgs(outPath, source, includeDir)
	if err != nil {
		return err
	}

	cmd := exec.Command(compiler, args...) // #nosec G204,G702 -- compiler is restricted to an allowlist above.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	major, ok := parts["Major"]
	if !ok {
		return "", fmt.Errorf("find ExpectedMiniaudioVersionMajor in generated bindings")
	}
	minor, ok := parts["Minor"]
	if !ok {
		return "", fmt.Errorf("find ExpectedMiniaudioVersionMinor in generated bindings")
	}
	revision, ok := parts["Revision"]
	if !ok {
		return "", fmt.Errorf("find ExpectedMiniaudioVersionRevision in generated bindings")
	}

	return major + "." + minor + "." + revision, nil
}

func downloadMiniaudioHeader(version, dstPath string) (err error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("miniaudio version must not be empty")
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

func compilerArgs(outPath, source, includeDir string) ([]string, error) {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-shared",
			"-I", includeDir,
			"-Wl,-soname," + filepath.Base(outPath),
			"-o", outPath, source,
			"-ldl", "-lm", "-lpthread",
		}, nil
	case "darwin":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-dynamiclib",
			"-I", includeDir,
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
			"-I", includeDir,
			"-o", outPath, source,
			"-lwinmm", "-lole32", "-luuid",
		}, nil
	case "freebsd", "netbsd":
		return []string{
			"-std=c11", "-O2", "-fPIC", "-shared",
			"-I", includeDir,
			"-o", outPath, source,
			"-lm", "-lpthread",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported GOOS for shared library build: %s", runtime.GOOS)
	}
}
