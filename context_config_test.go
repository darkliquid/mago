package mago

import "testing"

func TestContextAttachesLog(t *testing.T) {
	lib := newNullLibrary(t)

	log, err := lib.NewLog()
	if err != nil {
		t.Fatalf("NewLog: %v", err)
	}
	defer func() { _ = log.Close() }()

	ctx, err := lib.NewContextWithLog(log, BackendNull)
	if err != nil {
		t.Fatalf("NewContextWithLog: %v", err)
	}
	defer func() { _ = ctx.Close() }()

	if ctx.Log() == nil {
		t.Fatal("expected the context to report the attached log")
	}
}

func TestContextDeviceInfoForDefaultDevice(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	if _, err := ctx.DeviceInfo(DeviceTypePlayback, nil); err != nil {
		t.Fatalf("DeviceInfo: %v", err)
	}
}
