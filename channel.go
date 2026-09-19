//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

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
	return m.lib.bindings.maChannelMapGetChannel(&m.channels[0], uint32(len(m.channels)), uint32(i)) //nolint:gosec // bounded by the map length
}

// Clone returns an independent copy of the map.
func (m ChannelMap) Clone() ChannelMap {
	out := ChannelMap{lib: m.lib}
	if m.lib == nil || len(m.channels) == 0 {
		return out
	}
	out.channels = make([]uint8, len(m.channels))
	m.lib.bindings.maChannelMapCopy(&out.channels[0], &m.channels[0], uint32(len(m.channels))) //nolint:gosec // bounded by the map length
	return out
}

// DefaultChannelMapFor builds a channel map for the given channel count. When m
// already holds a map, it is copied; otherwise miniaudio's default layout for
// that channel count is used.
func (lib *Library) DefaultChannelMapFor(m ChannelMap, channels uint32) ChannelMap {
	out := ChannelMap{lib: lib}
	if lib == nil || channels == 0 {
		return out
	}
	out.channels = make([]uint8, channels)

	var in *uint8
	if len(m.channels) > 0 {
		in = &m.channels[0]
	}

	lib.bindings.maChannelMapCopyOrDefault(&out.channels[0], uintptr(channels), in, channels)
	return out
}

// String renders the map using miniaudio's channel names.
func (m ChannelMap) String() string {
	if m.lib == nil || len(m.channels) == 0 {
		return ""
	}
	var buffer [512]byte
	length := m.lib.bindings.maChannelMapToString(&m.channels[0], uint32(len(m.channels)), &buffer[0], uintptr(len(buffer))) //nolint:gosec // bounded by the map length
	if length > uintptr(len(buffer)) {
		length = uintptr(len(buffer))
	}
	return string(buffer[:length])
}

// ChannelConverterConfig describes a channel conversion.
type ChannelConverterConfig struct {
	Format                          Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	ChannelMapIn                    ChannelMap
	ChannelMapOut                   ChannelMap
	MixingMode                      ChannelMixMode
	CalculateLFEFromSpatialChannels bool
}

// ChannelConverter mixes between channel layouts for a fixed format.
type ChannelConverter struct {
	lib         *Library
	handle      *channelConverterHandle
	channelsIn  uint32
	channelsOut uint32
}

// NewChannelConverter creates a channel converter. Custom mixing weights are not
// supported yet and are rejected.
func (lib *Library) NewChannelConverter(config ChannelConverterConfig) (*ChannelConverter, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if config.ChannelsIn == 0 || config.ChannelsOut == 0 {
		return nil, fmt.Errorf("mago: channel converter requires non-zero channel counts")
	}
	if config.MixingMode == ChannelMixModeCustomWeights {
		return nil, fmt.Errorf("mago: custom channel mixing weights are not supported")
	}

	native := channelConverterConfigNative{
		Format:                          config.Format,
		ChannelsIn:                      config.ChannelsIn,
		ChannelsOut:                     config.ChannelsOut,
		ChannelMapIn:                    channelMapDataPtr(config.ChannelMapIn),
		ChannelMapOut:                   channelMapDataPtr(config.ChannelMapOut),
		MixingMode:                      config.MixingMode,
		CalculateLFEFromSpatialChannels: boolToBool32(config.CalculateLFEFromSpatialChannels),
	}

	handle := (*channelConverterHandle)(lib.bindings.magoAlloc(magoObjectChannelConverter))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate channel converter: out of memory")
	}
	if result := lib.bindings.maChannelConverterInit(&native, nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_channel_converter_init", result)
	}

	return &ChannelConverter{
		lib:         lib,
		handle:      handle,
		channelsIn:  config.ChannelsIn,
		channelsOut: config.ChannelsOut,
	}, nil
}

func channelMapDataPtr(m ChannelMap) *uint8 {
	if len(m.channels) == 0 {
		return nil
	}
	return &m.channels[0]
}

// Process processes float32 audio frames through the channel converter.
func (c *ChannelConverter) Process(out, in []float32) error {
	if c == nil || c.handle == nil {
		return fmt.Errorf("mago: nil channel converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}
	if len(in) == 0 {
		return nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 {
		return ErrInvalidSliceLength
	}
	frameCount := uint64(len(in) / int(c.channelsIn))
	requiredOut := int(frameCount * uint64(c.channelsOut))
	if len(out) < requiredOut {
		return ErrOutputTooSmall
	}
	return c.lib.resultError("ma_channel_converter_process_pcm_frames",
		c.lib.bindings.maChannelConverterProcessPCMFrames(c.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the channel converter.
func (c *ChannelConverter) ProcessS16(out, in []int16) error {
	if c == nil || c.handle == nil {
		return fmt.Errorf("mago: nil channel converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}
	if len(in) == 0 {
		return nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 {
		return ErrInvalidSliceLength
	}
	frameCount := uint64(len(in) / int(c.channelsIn))
	requiredOut := int(frameCount * uint64(c.channelsOut))
	if len(out) < requiredOut {
		return ErrOutputTooSmall
	}
	return c.lib.resultError("ma_channel_converter_process_pcm_frames",
		c.lib.bindings.maChannelConverterProcessPCMFrames(c.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// InputChannelMap reports the converter's resolved input channel map.
func (c *ChannelConverter) InputChannelMap() (ChannelMap, error) {
	if c == nil || c.handle == nil {
		return ChannelMap{}, fmt.Errorf("mago: nil channel converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return ChannelMap{}, err
	}
	if c.channelsIn == 0 {
		return ChannelMap{}, nil
	}

	m := ChannelMap{lib: c.lib, channels: make([]uint8, c.channelsIn)}
	if result := c.lib.bindings.maChannelConverterGetInputChannelMap(c.handle, &m.channels[0], uintptr(c.channelsIn)); result != Success {
		return ChannelMap{}, c.lib.resultError("ma_channel_converter_get_input_channel_map", result)
	}
	return m, nil
}

// OutputChannelMap reports the converter's resolved output channel map.
func (c *ChannelConverter) OutputChannelMap() (ChannelMap, error) {
	if c == nil || c.handle == nil {
		return ChannelMap{}, fmt.Errorf("mago: nil channel converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return ChannelMap{}, err
	}
	if c.channelsOut == 0 {
		return ChannelMap{}, nil
	}

	m := ChannelMap{lib: c.lib, channels: make([]uint8, c.channelsOut)}
	if result := c.lib.bindings.maChannelConverterGetOutputChannelMap(c.handle, &m.channels[0], uintptr(c.channelsOut)); result != Success {
		return ChannelMap{}, c.lib.resultError("ma_channel_converter_get_output_channel_map", result)
	}
	return m, nil
}

// Close uninitializes the converter and frees it.
func (c *ChannelConverter) Close() error {
	if c == nil || c.handle == nil {
		return nil
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}

	c.lib.bindings.maChannelConverterUninit(c.handle, nil)
	c.lib.bindings.magoFree(unsafe.Pointer(c.handle))
	c.handle = nil
	return nil
}
