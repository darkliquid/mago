package mago

import (
	"testing"
	"unsafe"

	"github.com/darkliquid/mago/internal/abi"
)

func TestMirroredStructLayouts(t *testing.T) {
	probe, err := abi.Run(".")
	if err != nil {
		t.Fatalf("layout probe: %v", err)
	}

	if got, want := uint64(unsafe.Sizeof(deviceIDNative{})), probe["sizeof:ma_device_id"]; got != want {
		t.Errorf("sizeof ma_device_id: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.Name)), probe["offsetof:ma_device_info.name"]; got != want {
		t.Errorf("offsetof ma_device_info.name: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.IsDefault)), probe["offsetof:ma_device_info.isDefault"]; got != want {
		t.Errorf("offsetof ma_device_info.isDefault: mirror %d, header %d", got, want)
	}
}
