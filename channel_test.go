package mago

import "testing"

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
