package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type functionSpec struct {
	FieldName string
	Symbol    string
	Type      string
}

type constSpec struct {
	Name  string
	Type  string
	Value string
}

var functions = []functionSpec{
	{FieldName: "maVersion", Symbol: "ma_version", Type: "func(*uint32, *uint32, *uint32)"},
	{FieldName: "maVersionString", Symbol: "ma_version_string", Type: "func() string"},
	{FieldName: "maResultDescription", Symbol: "ma_result_description", Type: "func(Result) string"},
	{FieldName: "maContextInit", Symbol: "ma_context_init", Type: "func(*Backend, uint32, unsafe.Pointer, *contextHandle) Result"},
	{FieldName: "maContextUninit", Symbol: "ma_context_uninit", Type: "func(*contextHandle)"},
	{FieldName: "maContextEnumerateDevices", Symbol: "ma_context_enumerate_devices", Type: "func(*contextHandle, uintptr, uintptr) Result"},
	{FieldName: "magoDeviceInit", Symbol: "mago_device_init", Type: "func(*contextHandle, *deviceConfigNative, **deviceHandle) Result"},
	{FieldName: "magoDeviceUninitFree", Symbol: "mago_device_uninit_free", Type: "func(*deviceHandle)"},
	{FieldName: "maDeviceStart", Symbol: "ma_device_start", Type: "func(*deviceHandle) Result"},
	{FieldName: "maDeviceStop", Symbol: "ma_device_stop", Type: "func(*deviceHandle) Result"},
	{FieldName: "maDeviceGetState", Symbol: "ma_device_get_state", Type: "func(*deviceHandle) DeviceState"},
	{FieldName: "maDeviceGetName", Symbol: "ma_device_get_name", Type: "func(*deviceHandle, DeviceType, *byte, uintptr, *uintptr) Result"},
	{FieldName: "maDeviceGetInfo", Symbol: "ma_device_get_info", Type: "func(*deviceHandle, DeviceType, *deviceInfoNative) Result"},
	{FieldName: "maDeviceGetLog", Symbol: "ma_device_get_log", Type: "func(*deviceHandle) *logHandle"},
	{FieldName: "maDeviceGetContext", Symbol: "ma_device_get_context", Type: "func(*deviceHandle) *contextHandle"},
	{FieldName: "maDeviceSetMasterVolume", Symbol: "ma_device_set_master_volume", Type: "func(*deviceHandle, float32) Result"},
	{FieldName: "maDeviceGetMasterVolume", Symbol: "ma_device_get_master_volume", Type: "func(*deviceHandle, *float32) Result"},
	{FieldName: "maDeviceSetMasterVolumeDB", Symbol: "ma_device_set_master_volume_db", Type: "func(*deviceHandle, float32) Result"},
	{FieldName: "maDeviceGetMasterVolumeDB", Symbol: "ma_device_get_master_volume_db", Type: "func(*deviceHandle, *float32) Result"},
	{FieldName: "maContextGetLog", Symbol: "ma_context_get_log", Type: "func(*contextHandle) *logHandle"},
	{FieldName: "maContextGetDeviceInfo", Symbol: "ma_context_get_device_info", Type: "func(*contextHandle, DeviceType, unsafe.Pointer, *deviceInfoNative) Result"},
	{FieldName: "magoAlloc", Symbol: "mago_alloc", Type: "func(int32) unsafe.Pointer"},
	{FieldName: "magoFree", Symbol: "mago_free", Type: "func(unsafe.Pointer)"},
	{FieldName: "maLogInit", Symbol: "ma_log_init", Type: "func(unsafe.Pointer, *logHandle) Result"},
	{FieldName: "maLogUninit", Symbol: "ma_log_uninit", Type: "func(*logHandle)"},
	{FieldName: "maLogPost", Symbol: "ma_log_post", Type: "func(*logHandle, uint32, string) Result"},
	{FieldName: "maLogLevelToString", Symbol: "ma_log_level_to_string", Type: "func(uint32) string"},
	{FieldName: "magoLogRegisterCallback", Symbol: "mago_log_register_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
	{FieldName: "magoLogUnregisterCallback", Symbol: "mago_log_unregister_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
	{FieldName: "magoContextConfigInit", Symbol: "mago_context_config_init", Type: "func(unsafe.Pointer)"},
	{FieldName: "magoContextConfigSetLog", Symbol: "mago_context_config_set_log", Type: "func(unsafe.Pointer, *logHandle)"},
}

var baseConstants = []constSpec{
	{Name: "Success", Type: "Result", Value: "0"},
	{Name: "Error", Type: "Result", Value: "-1"},
	{Name: "InvalidArgs", Type: "Result", Value: "-2"},
	{Name: "OutOfMemory", Type: "Result", Value: "-4"},
	{Name: "NoBackend", Type: "Result", Value: "-203"},
	{Name: "NoDevice", Type: "Result", Value: "-204"},
	{Name: "InvalidDeviceConfig", Type: "Result", Value: "-206"},
	{Name: "DeviceNotInitialized", Type: "Result", Value: "-300"},
	{Name: "DeviceAlreadyInitialized", Type: "Result", Value: "-301"},
	{Name: "DeviceNotStarted", Type: "Result", Value: "-302"},
	{Name: "DeviceNotStopped", Type: "Result", Value: "-303"},
	{Name: "BackendWASAPI", Type: "Backend", Value: "0"},
	{Name: "BackendDSound", Type: "Backend", Value: "1"},
	{Name: "BackendWinMM", Type: "Backend", Value: "2"},
	{Name: "BackendCoreAudio", Type: "Backend", Value: "3"},
	{Name: "BackendSndIO", Type: "Backend", Value: "4"},
	{Name: "BackendAudio4", Type: "Backend", Value: "5"},
	{Name: "BackendOSS", Type: "Backend", Value: "6"},
	{Name: "BackendPulseAudio", Type: "Backend", Value: "7"},
	{Name: "BackendALSA", Type: "Backend", Value: "8"},
	{Name: "BackendJACK", Type: "Backend", Value: "9"},
	{Name: "BackendAAudio", Type: "Backend", Value: "10"},
	{Name: "BackendOpenSL", Type: "Backend", Value: "11"},
	{Name: "BackendWebAudio", Type: "Backend", Value: "12"},
	{Name: "BackendCustom", Type: "Backend", Value: "13"},
	{Name: "BackendNull", Type: "Backend", Value: "14"},
	{Name: "DeviceTypePlayback", Type: "DeviceType", Value: "1"},
	{Name: "DeviceTypeCapture", Type: "DeviceType", Value: "2"},
	{Name: "DeviceTypeDuplex", Type: "DeviceType", Value: "3"},
	{Name: "DeviceTypeLoopback", Type: "DeviceType", Value: "4"},
	{Name: "ShareModeShared", Type: "ShareMode", Value: "0"},
	{Name: "ShareModeExclusive", Type: "ShareMode", Value: "1"},
	{Name: "PerformanceProfileLowLatency", Type: "PerformanceProfile", Value: "0"},
	{Name: "PerformanceProfileConservative", Type: "PerformanceProfile", Value: "1"},
	{Name: "FormatUnknown", Type: "Format", Value: "0"},
	{Name: "FormatU8", Type: "Format", Value: "1"},
	{Name: "FormatS16", Type: "Format", Value: "2"},
	{Name: "FormatF32", Type: "Format", Value: "5"},
	{Name: "NotificationStarted", Type: "NotificationType", Value: "0"},
	{Name: "NotificationStopped", Type: "NotificationType", Value: "1"},
	{Name: "NotificationRerouted", Type: "NotificationType", Value: "2"},
	{Name: "NotificationInterruptionBegan", Type: "NotificationType", Value: "3"},
	{Name: "NotificationInterruptionEnded", Type: "NotificationType", Value: "4"},
	{Name: "NotificationUnlocked", Type: "NotificationType", Value: "5"},
}

func resolveVersion(root, versionFlag string) (major, minor, revision string, err error) {
	if strings.TrimSpace(versionFlag) != "" {
		parts := strings.Split(strings.TrimSpace(versionFlag), ".")
		if len(parts) != 3 {
			return "", "", "", fmt.Errorf("invalid version format %q, expected x.y.z", versionFlag)
		}
		return parts[0], parts[1], parts[2], nil
	}

	headerPath := filepath.Join(root, "miniaudio.h")
	if data, err := os.ReadFile(headerPath); err == nil {
		majorRe := regexp.MustCompile(`#define\s+MA_VERSION_MAJOR\s+(\d+)`)
		minorRe := regexp.MustCompile(`#define\s+MA_VERSION_MINOR\s+(\d+)`)
		revRe := regexp.MustCompile(`#define\s+MA_VERSION_REVISION\s+(\d+)`)

		majorMatch := majorRe.FindSubmatch(data)
		minorMatch := minorRe.FindSubmatch(data)
		revMatch := revRe.FindSubmatch(data)

		if len(majorMatch) > 1 && len(minorMatch) > 1 && len(revMatch) > 1 {
			return string(majorMatch[1]), string(minorMatch[1]), string(revMatch[1]), nil
		}
	}

	bindingsPath := filepath.Join(root, "zz_generated.bindings.go")
	if data, err := os.ReadFile(bindingsPath); err == nil {
		pattern := regexp.MustCompile(`ExpectedMiniaudioVersion(Major|Minor|Revision)\s+uint32\s*=\s*(\d+)`)
		parts := map[string]string{}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			parts[match[1]] = match[2]
		}
		if maj, ok1 := parts["Major"]; ok1 {
			if minorVal, ok2 := parts["Minor"]; ok2 {
				if rev, ok3 := parts["Revision"]; ok3 {
					return maj, minorVal, rev, nil
				}
			}
		}
	}

	return "0", "11", "25", nil
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	versionFlag := flag.String("version", "", "miniaudio version (e.g. 0.11.26)")
	flag.Parse()

	major, minor, revision, err := resolveVersion(root, *versionFlag)
	if err != nil {
		panic(err)
	}

	constants := append([]constSpec{
		{Name: "ExpectedMiniaudioVersionMajor", Type: "uint32", Value: major},
		{Name: "ExpectedMiniaudioVersionMinor", Type: "uint32", Value: minor},
		{Name: "ExpectedMiniaudioVersionRevision", Type: "uint32", Value: revision},
	}, baseConstants...)

	outPath := filepath.Join(root, "zz_generated.bindings.go")
	var buf bytes.Buffer

	buf.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	buf.WriteString("\n")
	buf.WriteString("package mago\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"unsafe\"\n\n")
	buf.WriteString("\t\"github.com/ebitengine/purego\"\n")
	buf.WriteString(")\n\n")

	buf.WriteString("const (\n")
	for _, c := range constants {
		fmt.Fprintf(&buf, "\t%s %s = %s\n", c.Name, c.Type, c.Value)
	}
	buf.WriteString(")\n\n")

	buf.WriteString("type bindingSet struct {\n")
	for _, fn := range functions {
		fmt.Fprintf(&buf, "\t%s %s\n", fn.FieldName, fn.Type)
	}
	buf.WriteString("}\n\n")

	buf.WriteString("func (b *bindingSet) register(handle uintptr) {\n")
	for _, fn := range functions {
		fmt.Fprintf(&buf, "\tpurego.RegisterLibFunc(&b.%s, handle, %q)\n", fn.FieldName, fn.Symbol)
	}
	buf.WriteString("}\n\n")
	buf.WriteString("var _ unsafe.Pointer\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		panic(fmt.Errorf("format generated bindings: %w\n%s", err, buf.String()))
	}

	if err := os.WriteFile(outPath, formatted, 0o600); err != nil {
		panic(err)
	}
}
