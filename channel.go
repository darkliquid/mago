//go:build darwin || freebsd || linux || netbsd || windows

package mago

// ChannelMap is an ordered list of channel positions.
type ChannelMap struct {
	lib      *Library
	channels []uint8
}

// NewStandardChannelMap builds a channel map using one of miniaudio's standard
// channel layouts.
func (lib *Library) NewStandardChannelMap(std StandardChannelMap, channels uint32) ChannelMap {
	m := ChannelMap{lib: lib}
	if channels == 0 || lib == nil {
		return m
	}
	m.channels = make([]uint8, channels)
	lib.bindings.maChannelMapInitStandard(std, &m.channels[0], uintptr(channels), channels)
	return m
}

// NewBlankChannelMap builds a channel map with every position unset.
func (lib *Library) NewBlankChannelMap(channels uint32) ChannelMap {
	m := ChannelMap{lib: lib}
	if channels == 0 || lib == nil {
		return m
	}
	m.channels = make([]uint8, channels)
	lib.bindings.maChannelMapInitBlank(&m.channels[0], channels)
	return m
}

// Len reports the number of channels in the map.
func (m ChannelMap) Len() int {
	return len(m.channels)
}

// Channels returns a copy of the channel positions.
func (m ChannelMap) Channels() []Channel {
	out := make([]Channel, len(m.channels))
	for i, channel := range m.channels {
		out[i] = Channel(channel)
	}
	return out
}

// Get returns the channel position at index i, or ChannelNone when out of range.
func (m ChannelMap) Get(i int) Channel {
	if m.lib == nil || i < 0 || i >= len(m.channels) {
		return ChannelNone
	}
	return m.lib.bindings.maChannelMapGetChannel(&m.channels[0], uint32(len(m.channels)), uint32(i))
}

// Clone returns an independent copy of the map.
func (m ChannelMap) Clone() ChannelMap {
	out := ChannelMap{lib: m.lib}
	if m.lib == nil || len(m.channels) == 0 {
		return out
	}
	out.channels = make([]uint8, len(m.channels))
	m.lib.bindings.maChannelMapCopy(&out.channels[0], &m.channels[0], uint32(len(m.channels)))
	return out
}

// String renders the map using miniaudio's channel names.
func (m ChannelMap) String() string {
	if m.lib == nil || len(m.channels) == 0 {
		return ""
	}
	var buffer [512]byte
	length := m.lib.bindings.maChannelMapToString(&m.channels[0], uint32(len(m.channels)), &buffer[0], uintptr(len(buffer)))
	if length > uintptr(len(buffer)) {
		length = uintptr(len(buffer))
	}
	return string(buffer[:length])
}
