package abi

import "testing"

func TestParse(t *testing.T) {
	out := "sizeof:ma_device_id 256\n" +
		"offsetof:ma_device_info.name 256\n" +
		"offsetof:ma_device_info.isDefault 512\n"

	got, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["sizeof:ma_device_id"] != 256 {
		t.Errorf("sizeof ma_device_id = %d, want 256", got["sizeof:ma_device_id"])
	}
	if got["offsetof:ma_device_info.name"] != 256 {
		t.Errorf("offsetof name = %d, want 256", got["offsetof:ma_device_info.name"])
	}
	if got["offsetof:ma_device_info.isDefault"] != 512 {
		t.Errorf("offsetof isDefault = %d, want 512", got["offsetof:ma_device_info.isDefault"])
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	if _, err := Parse("garbage line without value\n"); err == nil {
		t.Fatal("expected error for malformed probe output")
	}
}
