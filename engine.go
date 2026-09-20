//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"errors"
	"fmt"
	"unsafe"
)

var errNilEngine = errors.New("mago: nil engine")

// Vec3 is a position, direction or velocity in miniaudio's 3D space.
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

// EngineConfig configures an Engine. Only Channels and SampleRate are required
// when NoDevice is set, and both are optional when a device is created, because
// the device then supplies them.
type EngineConfig struct {
	// Context is the context the engine's device is created in. Nil lets
	// miniaudio pick a backend.
	Context *Context
	// Log receives miniaudio diagnostics. It must outlive the engine.
	Log *Log
	// ListenerCount is how many listeners the engine mixes for. It defaults to 1
	// and must not exceed 4.
	ListenerCount uint32
	// Channels and SampleRate are the engine's mixer format.
	Channels   uint32
	SampleRate uint32
	// PeriodSizeInFrames pins the mixing block size. PeriodSizeInMilliseconds is
	// used when it is zero.
	PeriodSizeInFrames       uint32
	PeriodSizeInMilliseconds uint32
	// GainSmoothTimeInFrames smooths spatialized gain changes.
	// GainSmoothTimeInMilliseconds is used when it is zero.
	GainSmoothTimeInFrames       uint32
	GainSmoothTimeInMilliseconds uint32
	// DefaultVolumeSmoothTimeInPCMFrames is the default smoothing applied to
	// sound volume changes.
	DefaultVolumeSmoothTimeInPCMFrames uint32
	// PreMixStackSizeInBytes bounds how deep the graph may be. Zero uses
	// miniaudio's default of 512 KiB per channel.
	PreMixStackSizeInBytes uint32
	// MonoExpansionMode controls how mono sounds are spread across the channels
	// when spatialization is disabled.
	MonoExpansionMode MonoExpansionMode
	// NoAutoStart creates the engine without starting its device.
	NoAutoStart bool
	// NoDevice skips device creation entirely. The engine is then read by hand
	// with Read, which is what makes it usable without audio hardware, and it
	// requires Channels and SampleRate to be set.
	NoDevice bool
}

// DefaultEngineConfig returns miniaudio's engine defaults for a mixer with the
// given format. Set NoDevice to build an engine that is read by hand.
func DefaultEngineConfig(channels, sampleRate uint32) EngineConfig {
	return EngineConfig{
		ListenerCount:     1,
		Channels:          channels,
		SampleRate:        sampleRate,
		MonoExpansionMode: MonoExpansionModeDuplicate,
	}
}

// engineConfigNative mirrors what ma_engine_config_init() would produce, then
// applies the caller's overrides. The resampler defaults matter: miniaudio gives
// the resource manager a linear resampler with an unknown format, and disables
// low-pass filtering on the pitch resampler because the biquads can go unstable.
func engineConfigNativeFrom(config EngineConfig) engineConfigNative {
	listenerCount := config.ListenerCount
	if listenerCount == 0 {
		listenerCount = 1
	}

	native := engineConfigNative{
		ListenerCount:                      listenerCount,
		Channels:                           config.Channels,
		SampleRate:                         config.SampleRate,
		PeriodSizeInFrames:                 config.PeriodSizeInFrames,
		PeriodSizeInMilliseconds:           config.PeriodSizeInMilliseconds,
		GainSmoothTimeInFrames:             config.GainSmoothTimeInFrames,
		GainSmoothTimeInMilliseconds:       config.GainSmoothTimeInMilliseconds,
		DefaultVolumeSmoothTimeInPCMFrames: config.DefaultVolumeSmoothTimeInPCMFrames,
		PreMixStackSizeInBytes:             config.PreMixStackSizeInBytes,
		MonoExpansionMode:                  config.MonoExpansionMode,
		NoAutoStart:                        boolToBool32(config.NoAutoStart),
		NoDevice:                           boolToBool32(config.NoDevice),
		ResourceManagerResampling:          resamplerConfigFrom(DefaultResamplerConfig(FormatUnknown, 0, 0, 0)),
	}
	if config.Context != nil {
		native.Context = config.Context.handle
	}
	if config.Log != nil {
		native.Log = config.Log.handle
	}

	pitch := DefaultResamplerConfig(FormatF32, 0, 0, 0)
	pitch.LinearLPFOrder = 0
	native.PitchResampling = resamplerConfigFrom(pitch)

	return native
}

// Engine is miniaudio's high-level mixer: a node graph with a device and a
// resource manager attached. It plays Sound and SoundGroup objects, and with
// NoDevice set it can be read frame by frame instead.
//
// Thread safety follows miniaudio and the node graph: Read is lock-free and
// belongs to one audio thread, while creating sounds, changing their properties
// and closing them are control-thread operations. Sound properties are stored
// atomically and applied on the mixing thread, so a control thread may change
// volume, pan and position while the audio thread is reading.
type Engine struct {
	lib        *Library
	handle     *engineHandle
	graph      *NodeGraph
	channels   uint32
	sampleRate uint32
}

// NewEngine creates an engine. With NoDevice unset it also creates and starts a
// playback device, so a real backend is required; with NoDevice set the engine is
// driven by Read and needs no audio hardware at all. A device-less engine must be
// given Channels and SampleRate.
func (lib *Library) NewEngine(config EngineConfig) (*Engine, error) {
	if lib == nil {
		return nil, errors.New("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if config.ListenerCount > maxEngineListeners {
		return nil, fmt.Errorf("mago: engine supports at most %d listeners, got %d", maxEngineListeners, config.ListenerCount)
	}

	raw := lib.bindings.magoAlloc(magoObjectEngine)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate engine failed")
	}
	handle := (*engineHandle)(raw)

	native := engineConfigNativeFrom(config)
	if result := lib.bindings.maEngineInit(&native, handle); result != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_engine_init", result)
	}

	channels := lib.bindings.maEngineGetChannels(handle)
	engine := &Engine{
		lib:        lib,
		handle:     handle,
		channels:   channels,
		sampleRate: lib.bindings.maEngineGetSampleRate(handle),
	}
	// An engine's first member is a ma_node_graph, so these are borrowed views of
	// the engine's own graph and endpoint rather than separate allocations.
	engine.graph = borrowedNodeGraph(
		lib,
		lib.bindings.maEngineGetNodeGraph(handle),
		lib.bindings.maEngineGetEndpoint(handle),
		channels,
	)

	return engine, nil
}

// maxEngineListeners mirrors MA_ENGINE_MAX_LISTENERS.
const maxEngineListeners uint32 = 4

func (e *Engine) ensure() error {
	if e == nil || e.handle == nil {
		return errNilEngine
	}
	return e.lib.ensureOpen()
}

// NodeGraph returns the engine's node graph. It is owned by the engine: it may be
// used to inspect and route into the graph, but Close on it reports an error.
func (e *Engine) NodeGraph() *NodeGraph {
	if e == nil {
		return nil
	}
	return e.graph
}

// Endpoint returns the node that all engine output collects into. Like the
// graph's own endpoint it is owned by the engine and must not be closed.
func (e *Engine) Endpoint() Node {
	if e == nil || e.graph == nil {
		return nil
	}
	return e.graph.Endpoint()
}

// Read fills out with interleaved f32 frames and returns how many of them the
// graph actually produced, which is zero when no sound is playing. miniaudio
// always writes the whole slice, silencing whatever the graph had nothing to
// contribute, so treat the return value as information rather than as a length.
//
// Read is only meaningful for a device-less engine: an engine with a device mixes
// on the audio thread, and reading from elsewhere would compete with it.
func (e *Engine) Read(out []float32) (uint64, error) {
	if err := e.ensure(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if e.channels == 0 || len(out)%int(e.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}

	frameCount := uint64(len(out) / int(e.channels))
	var framesRead uint64
	result := e.lib.bindings.maEngineReadPCMFrames(e.handle, unsafe.Pointer(&out[0]), frameCount, &framesRead)
	if result == AtEnd {
		return framesRead, nil
	}
	if result != Success {
		return framesRead, e.lib.resultError("ma_engine_read_pcm_frames", result)
	}
	return framesRead, nil
}

// Channels reports the engine's mixing channel count.
func (e *Engine) Channels() uint32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetChannels(e.handle)
}

// SampleRate reports the engine's mixing sample rate.
func (e *Engine) SampleRate() uint32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetSampleRate(e.handle)
}

// Start starts the engine's device. An engine created with NoAutoStart needs it.
func (e *Engine) Start() error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_start", e.lib.bindings.maEngineStart(e.handle))
}

// Stop stops the engine's device without uninitialising it.
func (e *Engine) Stop() error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_stop", e.lib.bindings.maEngineStop(e.handle))
}

// Volume reports the engine's output volume.
func (e *Engine) Volume() float32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetVolume(e.handle)
}

// SetVolume sets the engine's output volume, where 1.0 is unity gain.
func (e *Engine) SetVolume(volume float32) error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_set_volume", e.lib.bindings.maEngineSetVolume(e.handle, volume))
}

// GainDB reports the engine's output gain in decibels.
func (e *Engine) GainDB() float32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetGainDB(e.handle)
}

// SetGainDB sets the engine's output gain in decibels.
func (e *Engine) SetGainDB(gainDB float32) error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_set_gain_db", e.lib.bindings.maEngineSetGainDB(e.handle, gainDB))
}

// Time reports the engine's global clock, in frames.
func (e *Engine) Time() uint64 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetTimeInPCMFrames(e.handle)
}

// SetTime moves the engine's global clock, which reschedules everything that is
// waiting on a start or stop time.
func (e *Engine) SetTime(globalTime uint64) error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_set_time_in_pcm_frames", e.lib.bindings.maEngineSetTimeInPCMFrames(e.handle, globalTime))
}

// ListenerCount reports how many listeners the engine mixes for.
func (e *Engine) ListenerCount() uint32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineGetListenerCount(e.handle)
}

// Listener returns a view of one of the engine's spatialization listeners.
func (e *Engine) Listener(index uint32) *Listener {
	return &Listener{engine: e, index: index}
}

// ClosestListener reports the index of the listener nearest to position.
func (e *Engine) ClosestListener(position Vec3) uint32 {
	if e == nil || e.handle == nil {
		return 0
	}
	return e.lib.bindings.maEngineFindClosestListener(e.handle, position.X, position.Y, position.Z)
}

// PlaySound loads path through the engine's resource manager and plays it once,
// attaching it to group when one is given. The engine owns the sound and releases
// it when it ends, so it cannot be controlled afterwards.
func (e *Engine) PlaySound(path string, group *SoundGroup) error {
	if err := e.ensure(); err != nil {
		return err
	}
	return e.lib.resultError("ma_engine_play_sound", e.lib.bindings.maEnginePlaySound(e.handle, path, soundGroupHandle(group)))
}

// PlaySoundOn plays path attached to a specific node's input bus instead of the
// endpoint. The engine still owns the sound.
func (e *Engine) PlaySoundOn(path string, node Node, inputBusIndex uint32) error {
	if err := e.ensure(); err != nil {
		return err
	}
	if node == nil {
		return errNilNode
	}
	handle := node.nodeHandle()
	if handle == nil {
		return errNilNode
	}
	return e.lib.resultError("ma_engine_play_sound_ex",
		e.lib.bindings.maEnginePlaySoundEx(e.handle, path, handle, inputBusIndex))
}

// Close uninitialises the engine, its device and its resource manager. Close
// every sound first: miniaudio does not track them for you.
func (e *Engine) Close() error {
	if e == nil || e.handle == nil {
		return nil
	}
	if err := e.lib.ensureOpen(); err != nil {
		return err
	}

	e.lib.bindings.maEngineUninit(e.handle)
	e.lib.bindings.magoFree(unsafe.Pointer(e.handle))
	e.handle = nil
	if e.graph != nil {
		e.graph.handle = nil
		e.graph.endpoint = nil
		e.graph.closed = true
	}
	return nil
}

// vec3 calls one of the ma_vec3f getters and unpacks the result. purego cannot
// receive a struct return outside darwin and linux, so those getters are bridge
// shims that write through an out-parameter.
func (e *Engine) vec3(get func(*engineHandle, uint32, *float32), index uint32) Vec3 {
	if e == nil || e.handle == nil {
		return Vec3{}
	}
	var out [3]float32
	get(e.handle, index, &out[0])
	return Vec3{X: out[0], Y: out[1], Z: out[2]}
}

// Listener is a view of one of an engine's spatialization listeners. It holds no
// native memory of its own; the engine owns it.
type Listener struct {
	engine *Engine
	index  uint32
}

// Index reports which of the engine's listeners this is.
func (l *Listener) Index() uint32 {
	if l == nil {
		return 0
	}
	return l.index
}

// Engine reports the engine the listener belongs to.
func (l *Listener) Engine() *Engine {
	if l == nil {
		return nil
	}
	return l.engine
}

func (l *Listener) ensure() error {
	if l == nil || l.engine == nil {
		return errNilEngine
	}
	return l.engine.ensure()
}

// Position reports the listener's position.
func (l *Listener) Position() Vec3 {
	if l == nil {
		return Vec3{}
	}
	return l.engine.vec3(l.engine.lib.bindings.magoEngineListenerGetPosition, l.index)
}

// SetPosition moves the listener.
func (l *Listener) SetPosition(position Vec3) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetPosition(l.engine.handle, l.index, position.X, position.Y, position.Z)
	return nil
}

// Direction reports the direction the listener is facing. It only matters when
// the listener has a cone.
func (l *Listener) Direction() Vec3 {
	if l == nil {
		return Vec3{}
	}
	return l.engine.vec3(l.engine.lib.bindings.magoEngineListenerGetDirection, l.index)
}

// SetDirection sets the direction the listener is facing.
func (l *Listener) SetDirection(direction Vec3) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetDirection(l.engine.handle, l.index, direction.X, direction.Y, direction.Z)
	return nil
}

// Velocity reports the listener's velocity, which drives the Doppler effect.
func (l *Listener) Velocity() Vec3 {
	if l == nil {
		return Vec3{}
	}
	return l.engine.vec3(l.engine.lib.bindings.magoEngineListenerGetVelocity, l.index)
}

// SetVelocity sets the listener's velocity.
func (l *Listener) SetVelocity(velocity Vec3) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetVelocity(l.engine.handle, l.index, velocity.X, velocity.Y, velocity.Z)
	return nil
}

// WorldUp reports the listener's up vector, which orients the spatializer.
func (l *Listener) WorldUp() Vec3 {
	if l == nil {
		return Vec3{}
	}
	return l.engine.vec3(l.engine.lib.bindings.magoEngineListenerGetWorldUp, l.index)
}

// SetWorldUp sets the listener's up vector.
func (l *Listener) SetWorldUp(worldUp Vec3) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetWorldUp(l.engine.handle, l.index, worldUp.X, worldUp.Y, worldUp.Z)
	return nil
}

// Cone reports the listener's cone as inner and outer angles in radians and the
// gain outside the outer angle.
func (l *Listener) Cone() (innerAngle, outerAngle, outerGain float32) {
	if l == nil || l.engine == nil || l.engine.handle == nil {
		return 0, 0, 0
	}
	l.engine.lib.bindings.maEngineListenerGetCone(l.engine.handle, l.index, &innerAngle, &outerAngle, &outerGain)
	return innerAngle, outerAngle, outerGain
}

// SetCone gives the listener a cone, so that sounds behind it are attenuated.
func (l *Listener) SetCone(innerAngle, outerAngle, outerGain float32) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetCone(l.engine.handle, l.index, innerAngle, outerAngle, outerGain)
	return nil
}

// Enabled reports whether the listener takes part in spatialization. At least one
// listener should always be enabled.
func (l *Listener) Enabled() bool {
	if l == nil || l.engine == nil || l.engine.handle == nil {
		return false
	}
	return l.engine.lib.bindings.maEngineListenerIsEnabled(l.engine.handle, l.index) != 0
}

// SetEnabled enables or disables the listener.
func (l *Listener) SetEnabled(enabled bool) error {
	if err := l.ensure(); err != nil {
		return err
	}
	l.engine.lib.bindings.maEngineListenerSetEnabled(l.engine.handle, l.index, boolToBool32(enabled))
	return nil
}
