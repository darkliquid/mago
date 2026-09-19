package mago

import (
	"testing"
	"unsafe"
)

func TestStandardStereoChannelMap(t *testing.T) {
	lib := newNullLibrary(t)

	m := lib.NewStandardChannelMap(StandardChannelMapMicrosoft, 2)
	if m.Len() != 2 {
		t.Fatalf("length %d, want 2", m.Len())
	}
	if m.Get(0) != ChannelFrontLeft || m.Get(1) != ChannelFrontRight {
		t.Fatalf("stereo map = %v, want front-left, front-right", m.Channels())
	}
	if m.String() == "" {
		t.Fatal("expected a non-empty channel-map string")
	}

	clone := m.Clone()
	if clone.Len() != m.Len() || clone.Get(0) != m.Get(0) {
		t.Fatalf("clone %v does not match %v", clone.Channels(), m.Channels())
	}
}

func TestBlankChannelMap(t *testing.T) {
	lib := newNullLibrary(t)

	m := lib.NewBlankChannelMap(4)
	if m.Len() != 4 {
		t.Fatalf("length %d, want 4", m.Len())
	}
	for i := 0; i < m.Len(); i++ {
		if m.Get(i) != ChannelNone {
			t.Fatalf("blank map[%d] = %v, want none", i, m.Get(i))
		}
	}
}

func TestChannelConverterStereoToMono(t *testing.T) {
	lib := newNullLibrary(t)

	converter, err := lib.NewChannelConverter(ChannelConverterConfig{
		Format:      FormatF32,
		ChannelsIn:  2,
		ChannelsOut: 1,
		MixingMode:  ChannelMixModeRectangular,
	})
	if err != nil {
		t.Fatalf("NewChannelConverter: %v", err)
	}
	defer func() { _ = converter.Close() }()

	in := []float32{0.5, 0.5, -0.5, -0.5}
	out := make([]float32, 2)

	if err := converter.ProcessPCMFrames(
		unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), 2,
	); err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}
	if out[0] == 0 {
		t.Fatal("expected a non-zero down-mix")
	}

	if _, err := converter.InputChannelMap(); err != nil {
		t.Fatalf("InputChannelMap: %v", err)
	}
	if _, err := converter.OutputChannelMap(); err != nil {
		t.Fatalf("OutputChannelMap: %v", err)
	}
}

func TestChannelConverterRejectsCustomWeights(t *testing.T) {
	lib := newNullLibrary(t)

	if _, err := lib.NewChannelConverter(ChannelConverterConfig{
		Format:      FormatF32,
		ChannelsIn:  2,
		ChannelsOut: 2,
		MixingMode:  ChannelMixModeCustomWeights,
	}); err == nil {
		t.Fatal("expected custom weights to be rejected")
	}
}

func TestDefaultChannelMapFor(t *testing.T) {
	lib := newNullLibrary(t)

	def := lib.DefaultChannelMapFor(ChannelMap{}, 2)
	if def.Len() != 2 {
		t.Fatalf("length %d, want 2", def.Len())
	}
	if def.Get(0) != ChannelFrontLeft || def.Get(1) != ChannelFrontRight {
		t.Fatalf("default 2-channel map = %v, want front-left, front-right", def.Channels())
	}

	// A supplied map must be copied rather than replaced with the default.
	supplied := lib.NewBlankChannelMap(2)
	copied := lib.DefaultChannelMapFor(supplied, 2)
	if copied.Len() != 2 || copied.Get(0) != ChannelNone {
		t.Fatalf("supplied map was not copied: %v", copied.Channels())
	}
}
