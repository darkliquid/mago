//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	errNilSound          = errors.New("mago: nil sound")
	errSoundEngineClosed = errors.New("mago: sound's engine is closed")
)

// SoundFlags mirror the MA_SOUND_FLAG_* flags.
type SoundFlags uint32

const (
	// SoundFlagStream streams from the resource manager instead of decoding the
	// whole sound into memory.
	SoundFlagStream SoundFlags = 0x00000001
	// SoundFlagDecode decodes the sound in full up front.
	SoundFlagDecode SoundFlags = 0x00000002
	// SoundFlagAsync returns before the sound has finished loading.
	SoundFlagAsync SoundFlags = 0x00000004
	// SoundFlagWaitInit blocks until an asynchronous load completes.
	SoundFlagWaitInit SoundFlags = 0x00000008
	// SoundFlagUnknownLength declares that the length is not known.
	SoundFlagUnknownLength SoundFlags = 0x00000010
	// SoundFlagLooping makes the sound restart when it reaches the end.
	SoundFlagLooping SoundFlags = 0x00000020

	// SoundFlagNoDefaultAttachment leaves the sound unattached so it can be
	// routed into a node graph by hand.
	SoundFlagNoDefaultAttachment SoundFlags = 0x00001000
	// SoundFlagNoPitch disables pitch shifting, which also disables the Doppler
	// effect for that sound.
	SoundFlagNoPitch SoundFlags = 0x00002000
	// SoundFlagNoSpatialization disables 3D spatialization for that sound.
	SoundFlagNoSpatialization SoundFlags = 0x00004000
)

// SoundConfig configures a Sound.
type SoundConfig struct {
	// Flags are the MA_SOUND_FLAG_* options.
	Flags SoundFlags
	// Group attaches the sound to a group's input bus. Nil attaches it directly
	// to the engine endpoint.
	Group *SoundGroup
}

// SoundGroupConfig configures a SoundGroup.
type SoundGroupConfig struct {
	// Flags are the MA_SOUND_FLAG_* options.
	Flags SoundFlags
	// Parent nests the group inside another group, so their gains and pans
	// cascade. Nil attaches it directly to the engine endpoint.
	Parent *SoundGroup
}

// soundCommon is the shared implementation behind Sound and SoundGroup.
// miniaudio declares ma_sound_group as a typedef of ma_sound and implements every
// ma_sound_group_* accessor as a forward to its ma_sound_* twin, so one
// implementation covers both.
type soundCommon struct {
	lib    *Library
	handle *soundHandle
	engine *Engine
}

// usable reports whether the native sound can still be touched. A sound outlives
// nothing: once its engine is closed, miniaudio's own functions would reach into
// the engine's freed node graph, so they must not be called.
func (s *soundCommon) usable() bool {
	return s != nil && s.handle != nil && s.engine != nil && s.engine.handle != nil
}

func (s *soundCommon) ensure() error {
	if !s.usable() {
		return errNilSound
	}
	if s.engine == nil || s.engine.handle == nil {
		return errSoundEngineClosed
	}
	return s.lib.ensureOpen()
}

// soundGroupHandle resolves a possibly nil group into a handle miniaudio accepts.
// It has to be a free function: calling a method promoted from the embedded
// soundCommon on a nil *SoundGroup dereferences the nil pointer before the
// receiver check can run.
func soundGroupHandle(group *SoundGroup) *soundHandle {
	if group == nil {
		return nil
	}
	return group.handle
}

// Engine reports the engine the sound belongs to.
func (s *soundCommon) Engine() *Engine {
	if s == nil {
		return nil
	}
	return s.engine
}

// closeSound uninitialises and frees the native sound object. miniaudio's
// ma_sound_uninit does not free the struct, because the caller owns it.
func (s *soundCommon) closeSound() error {
	if !s.usable() {
		return nil
	}
	if err := s.lib.ensureOpen(); err != nil {
		return err
	}

	handle := s.handle
	s.handle = nil
	if s.engine == nil || s.engine.handle == nil {
		// The engine is gone, so ma_sound_uninit would detach from a freed node
		// graph. Release only our own allocation and leave a note in the docs that
		// sounds must be closed before their engine.
		s.lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil
	}
	s.lib.bindings.maSoundUninit(handle)
	s.lib.bindings.magoFree(unsafe.Pointer(handle))
	return nil
}

// Start starts or resumes playback.
func (s *soundCommon) Start() error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.lib.resultError("ma_sound_start", s.lib.bindings.maSoundStart(s.handle))
}

// Stop stops playback without rewinding it.
func (s *soundCommon) Stop() error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.lib.resultError("ma_sound_stop", s.lib.bindings.maSoundStop(s.handle))
}

// StopWithFade fades out over fadeLengthInFrames frames and then stops. Use
// ResetStopTimeAndFade before restarting the sound.
func (s *soundCommon) StopWithFade(fadeLengthInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.lib.resultError("ma_sound_stop_with_fade_in_pcm_frames",
		s.lib.bindings.maSoundStopWithFadeInPCMFrames(s.handle, fadeLengthInFrames))
}

// StopWithFadeInDuration is StopWithFade in Go time units, converted with the
// engine's sample rate.
func (s *soundCommon) StopWithFadeInDuration(fadeLength time.Duration) error {
	return s.StopWithFade(s.engineFrames(fadeLength))
}

// ResetStartTime clears a scheduled start.
func (s *soundCommon) ResetStartTime() error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundResetStartTime(s.handle)
	return nil
}

// ResetStopTime clears a scheduled stop.
func (s *soundCommon) ResetStopTime() error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundResetStopTime(s.handle)
	return nil
}

// ResetFade clears the current fade.
func (s *soundCommon) ResetFade() error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundResetFade(s.handle)
	return nil
}

// ResetStopTimeAndFade clears both a scheduled stop and the current fade. It does
// not rewind the sound.
func (s *soundCommon) ResetStopTimeAndFade() error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundResetStopTimeAndFade(s.handle)
	return nil
}

// Volume reports the sound's volume, where 1.0 is unity gain.
func (s *soundCommon) Volume() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetVolume(s.handle)
}

// SetVolume sets the sound's volume.
func (s *soundCommon) SetVolume(volume float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetVolume(s.handle, volume)
	return nil
}

// Pan reports the sound's stereo position, from -1 (left) to 1 (right).
func (s *soundCommon) Pan() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetPan(s.handle)
}

// SetPan sets the sound's stereo position, from -1 (left) to 1 (right).
func (s *soundCommon) SetPan(pan float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPan(s.handle, pan)
	return nil
}

// PanMode reports whether pan is a balance control or a true pan.
func (s *soundCommon) PanMode() PanMode {
	if !s.usable() {
		return PanModeBalance
	}
	return s.lib.bindings.maSoundGetPanMode(s.handle)
}

// SetPanMode selects a balance control or a true pan.
func (s *soundCommon) SetPanMode(mode PanMode) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPanMode(s.handle, mode)
	return nil
}

// Pitch reports the sound's pitch multiplier, where 1.0 is unshifted.
func (s *soundCommon) Pitch() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetPitch(s.handle)
}

// SetPitch sets the sound's pitch multiplier. Pitch shifting is unavailable on a
// sound created with SoundFlagNoPitch.
func (s *soundCommon) SetPitch(pitch float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPitch(s.handle, pitch)
	return nil
}

// SpatializationEnabled reports whether the sound is spatialized in 3D.
func (s *soundCommon) SpatializationEnabled() bool {
	if !s.usable() {
		return false
	}
	return s.lib.bindings.maSoundIsSpatializationEnabled(s.handle) != 0
}

// SetSpatializationEnabled turns 3D spatialization on or off.
func (s *soundCommon) SetSpatializationEnabled(enabled bool) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetSpatializationEnabled(s.handle, boolToBool32(enabled))
	return nil
}

// PinnedListenerIndex reports the listener the sound always spatializes against,
// or maxEngineListeners when it uses the closest one.
func (s *soundCommon) PinnedListenerIndex() uint32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetPinnedListenerIndex(s.handle)
}

// SetPinnedListenerIndex pins the sound to one listener. Pass maxEngineListeners
// to go back to using the closest listener.
func (s *soundCommon) SetPinnedListenerIndex(listenerIndex uint32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPinnedListenerIndex(s.handle, listenerIndex)
	return nil
}

// ListenerIndex reports the listener the engine chose for the sound.
func (s *soundCommon) ListenerIndex() uint32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetListenerIndex(s.handle)
}

// Position reports the sound's position in the space the listener is in.
func (s *soundCommon) Position() Vec3 {
	if !s.usable() {
		return Vec3{}
	}
	return s.vec3(s.lib.bindings.magoSoundGetPosition)
}

// SetPosition moves the sound.
func (s *soundCommon) SetPosition(position Vec3) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPosition(s.handle, position.X, position.Y, position.Z)
	return nil
}

// Direction reports the direction the sound is facing, which only matters when it
// has a cone.
func (s *soundCommon) Direction() Vec3 {
	if !s.usable() {
		return Vec3{}
	}
	return s.vec3(s.lib.bindings.magoSoundGetDirection)
}

// SetDirection sets the direction the sound is facing.
func (s *soundCommon) SetDirection(direction Vec3) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetDirection(s.handle, direction.X, direction.Y, direction.Z)
	return nil
}

// Velocity reports the sound's velocity, which drives the Doppler effect.
func (s *soundCommon) Velocity() Vec3 {
	if !s.usable() {
		return Vec3{}
	}
	return s.vec3(s.lib.bindings.magoSoundGetVelocity)
}

// SetVelocity sets the sound's velocity.
func (s *soundCommon) SetVelocity(velocity Vec3) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetVelocity(s.handle, velocity.X, velocity.Y, velocity.Z)
	return nil
}

// DirectionToListener reports the unit vector from the sound to its listener.
func (s *soundCommon) DirectionToListener() Vec3 {
	if !s.usable() {
		return Vec3{}
	}
	return s.vec3(s.lib.bindings.magoSoundGetDirectionToListener)
}

// AttenuationModel reports how the sound fades with distance.
func (s *soundCommon) AttenuationModel() AttenuationModel {
	if !s.usable() {
		return AttenuationNone
	}
	return s.lib.bindings.maSoundGetAttenuationModel(s.handle)
}

// SetAttenuationModel selects how the sound fades with distance. With
// AttenuationNone the sound is neither attenuated nor spatialized.
func (s *soundCommon) SetAttenuationModel(model AttenuationModel) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetAttenuationModel(s.handle, model)
	return nil
}

// Positioning reports whether the sound's position is absolute or relative to the
// listener.
func (s *soundCommon) Positioning() Positioning {
	if !s.usable() {
		return PositioningAbsolute
	}
	return s.lib.bindings.maSoundGetPositioning(s.handle)
}

// SetPositioning selects absolute or listener-relative coordinates.
func (s *soundCommon) SetPositioning(positioning Positioning) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetPositioning(s.handle, positioning)
	return nil
}

// Rolloff reports the attenuation curve's rolloff factor.
func (s *soundCommon) Rolloff() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetRolloff(s.handle)
}

// SetRolloff sets the attenuation curve's rolloff factor.
func (s *soundCommon) SetRolloff(rolloff float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetRolloff(s.handle, rolloff)
	return nil
}

// MinGain reports the lower bound on the spatialized gain.
func (s *soundCommon) MinGain() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetMinGain(s.handle)
}

// SetMinGain sets the lower bound on the spatialized gain.
func (s *soundCommon) SetMinGain(minGain float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetMinGain(s.handle, minGain)
	return nil
}

// MaxGain reports the upper bound on the spatialized gain.
func (s *soundCommon) MaxGain() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetMaxGain(s.handle)
}

// SetMaxGain sets the upper bound on the spatialized gain.
func (s *soundCommon) SetMaxGain(maxGain float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetMaxGain(s.handle, maxGain)
	return nil
}

// MinDistance reports the distance at which attenuation begins.
func (s *soundCommon) MinDistance() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetMinDistance(s.handle)
}

// SetMinDistance sets the distance at which attenuation begins.
func (s *soundCommon) SetMinDistance(minDistance float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetMinDistance(s.handle, minDistance)
	return nil
}

// MaxDistance reports the distance at which attenuation stops.
func (s *soundCommon) MaxDistance() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetMaxDistance(s.handle)
}

// SetMaxDistance sets the distance at which attenuation stops.
func (s *soundCommon) SetMaxDistance(maxDistance float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetMaxDistance(s.handle, maxDistance)
	return nil
}

// Cone reports the sound's cone as inner and outer angles in radians and the gain
// outside the outer angle.
func (s *soundCommon) Cone() (innerAngle, outerAngle, outerGain float32) {
	if !s.usable() {
		return 0, 0, 0
	}
	s.lib.bindings.maSoundGetCone(s.handle, &innerAngle, &outerAngle, &outerGain)
	return innerAngle, outerAngle, outerGain
}

// SetCone gives the sound a cone, so that listeners outside it are attenuated.
func (s *soundCommon) SetCone(innerAngle, outerAngle, outerGain float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetCone(s.handle, innerAngle, outerAngle, outerGain)
	return nil
}

// DopplerFactor reports how strongly the Doppler effect applies to the sound.
func (s *soundCommon) DopplerFactor() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetDopplerFactor(s.handle)
}

// SetDopplerFactor scales the Doppler effect for the sound.
func (s *soundCommon) SetDopplerFactor(dopplerFactor float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetDopplerFactor(s.handle, dopplerFactor)
	return nil
}

// DirectionalAttenuationFactor reports how much the sound's own direction affects
// its attenuation.
func (s *soundCommon) DirectionalAttenuationFactor() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetDirectionalAttenuationFactor(s.handle)
}

// SetDirectionalAttenuationFactor scales how much the sound's own direction
// affects its attenuation.
func (s *soundCommon) SetDirectionalAttenuationFactor(factor float32) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetDirectionalAttenuationFactor(s.handle, factor)
	return nil
}

// CurrentFadeVolume reports the volume the current fade has reached.
func (s *soundCommon) CurrentFadeVolume() float32 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetCurrentFadeVolume(s.handle)
}

// SetFade fades the volume from volumeBeg to volumeEnd over fadeLengthInFrames
// frames, starting when the sound is next mixed.
func (s *soundCommon) SetFade(volumeBeg, volumeEnd float32, fadeLengthInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetFadeInPCMFrames(s.handle, volumeBeg, volumeEnd, fadeLengthInFrames)
	return nil
}

// SetFadeInDuration is SetFade in Go time units, converted with the engine's
// sample rate.
func (s *soundCommon) SetFadeInDuration(volumeBeg, volumeEnd float32, fadeLength time.Duration) error {
	return s.SetFade(volumeBeg, volumeEnd, s.engineFrames(fadeLength))
}

// SetFadeStart fades the volume over fadeLengthInFrames frames starting at an
// absolute point on the engine's clock.
func (s *soundCommon) SetFadeStart(volumeBeg, volumeEnd float32, fadeLengthInFrames, absoluteGlobalTimeInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetFadeStartInPCMFrames(s.handle, volumeBeg, volumeEnd, fadeLengthInFrames, absoluteGlobalTimeInFrames)
	return nil
}

// SetStartTime schedules playback to begin at an absolute point on the engine's
// clock.
func (s *soundCommon) SetStartTime(absoluteGlobalTimeInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetStartTimeInPCMFrames(s.handle, absoluteGlobalTimeInFrames)
	return nil
}

// SetStartTimeInDuration is SetStartTime in Go time units, converted with the
// engine's sample rate.
func (s *soundCommon) SetStartTimeInDuration(absolute time.Duration) error {
	return s.SetStartTime(s.engineFrames(absolute))
}

// SetStopTime schedules playback to stop at an absolute point on the engine's
// clock.
func (s *soundCommon) SetStopTime(absoluteGlobalTimeInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetStopTimeInPCMFrames(s.handle, absoluteGlobalTimeInFrames)
	return nil
}

// SetStopTimeInDuration is SetStopTime in Go time units, converted with the
// engine's sample rate.
func (s *soundCommon) SetStopTimeInDuration(absolute time.Duration) error {
	return s.SetStopTime(s.engineFrames(absolute))
}

// SetStopTimeWithFade schedules a fade-out that ends with the sound stopping at an
// absolute point on the engine's clock.
func (s *soundCommon) SetStopTimeWithFade(stopAbsoluteGlobalTimeInFrames, fadeLengthInFrames uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetStopTimeWithFadeInPCMFrames(s.handle, stopAbsoluteGlobalTimeInFrames, fadeLengthInFrames)
	return nil
}

// IsPlaying reports whether the sound is currently producing output.
func (s *soundCommon) IsPlaying() bool {
	if !s.usable() {
		return false
	}
	return s.lib.bindings.maSoundIsPlaying(s.handle) != 0
}

// TimeInPCMFrames reports how far into the sound playback has reached.
func (s *soundCommon) TimeInPCMFrames() uint64 {
	if !s.usable() {
		return 0
	}
	return s.lib.bindings.maSoundGetTimeInPCMFrames(s.handle)
}

// engineFrames converts a Go duration into frames on the engine's clock, using the
// same millisecond arithmetic miniaudio uses for its millisecond variants.
func (s *soundCommon) engineFrames(d time.Duration) uint64 {
	if s == nil || s.engine == nil || s.engine.sampleRate == 0 {
		return 0
	}
	return uint64(d) / uint64(time.Millisecond) * uint64(s.engine.sampleRate) / 1000
}

// vec3 calls one of the ma_vec3f getters and unpacks the result. purego cannot
// receive a struct return outside darwin and linux, so those getters are bridge
// shims that write through an out-parameter.
func (s *soundCommon) vec3(get func(*soundHandle, *float32)) Vec3 {
	var out [3]float32
	get(s.handle, &out[0])
	return Vec3{X: out[0], Y: out[1], Z: out[2]}
}

// SoundGroup mixes a set of sounds together. Groups nest, so their gains and pans
// cascade down to the sounds attached to them.
type SoundGroup struct {
	soundCommon
}

// NewSoundGroup creates a group, optionally parented to another group.
func (e *Engine) NewSoundGroup(config SoundGroupConfig) (*SoundGroup, error) {
	if err := e.ensure(); err != nil {
		return nil, err
	}

	handle, err := e.newSoundObject()
	if err != nil {
		return nil, err
	}

	result := e.lib.bindings.maSoundGroupInit(e.handle, uint32(config.Flags), soundGroupHandle(config.Parent), handle)
	if result != Success {
		e.lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, e.lib.resultError("ma_sound_group_init", result)
	}

	return &SoundGroup{soundCommon: soundCommon{lib: e.lib, handle: handle, engine: e}}, nil
}

// Close uninitialises the group and frees its native memory. Close the sounds
// attached to it first.
func (g *SoundGroup) Close() error {
	if g == nil {
		return nil
	}
	return g.closeSound()
}

var _ = (*SoundGroup)(nil)

// Sound is a single playing item: a decoded or streamed file, or any data source.
//
// A new sound starts stopped, matching miniaudio, so call Start to hear it.
// Groups, unlike sounds, start playing as soon as they are created.
type Sound struct {
	soundCommon
	source   DataSource
	wrapper  *CustomDataSource
	endToken uintptr
}

// SoundCallback is called when a sound reaches its end. It runs on the mixing
// thread, so it must not touch the sound or the engine: signal another goroutine
// instead.
type SoundCallback func(*Sound)

type soundEndState struct {
	sound *Sound
	fn    SoundCallback
}

var (
	soundCallbackSeq atomic.Uint64
	soundCallbacks   sync.Map // token -> *soundEndState
)

var soundEndCallbackPtr = purego.NewCallback(func(token uintptr, _ uintptr) uintptr {
	value, ok := soundCallbacks.Load(token)
	if !ok {
		return 0
	}
	if state, ok := value.(*soundEndState); ok && state.fn != nil {
		state.fn(state.sound)
	}
	return 0
})

// newSoundObject allocates the native ma_sound object.
func (e *Engine) newSoundObject() (*soundHandle, error) {
	raw := e.lib.bindings.magoAlloc(magoObjectSound)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate sound failed")
	}
	return (*soundHandle)(raw), nil
}

// NewSoundFromFile loads path through the engine's resource manager. The engine
// decodes with its own settings, so the sound is always f32 at the engine's
// sample rate.
func (e *Engine) NewSoundFromFile(path string, config SoundConfig) (*Sound, error) {
	if err := e.ensure(); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, errors.New("mago: empty sound path")
	}

	handle, err := e.newSoundObject()
	if err != nil {
		return nil, err
	}

	result := e.lib.bindings.maSoundInitFromFile(
		e.handle, path, uint32(config.Flags), soundGroupHandle(config.Group), nil, handle)
	if result != Success {
		e.lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, e.lib.resultError("ma_sound_init_from_file", result)
	}

	return &Sound{soundCommon: soundCommon{lib: e.lib, handle: handle, engine: e}}, nil
}

// NewSoundFromDataSource plays any DataSource. Sources miniaudio already
// understands are read natively; any other Go source is registered as a custom
// data source that the sound owns and closes with itself. The source must produce
// f32 audio, because that is all the engine mixes.
func (e *Engine) NewSoundFromDataSource(source DataSource, config SoundConfig) (*Sound, error) {
	if err := e.ensure(); err != nil {
		return nil, err
	}

	native, wrapper, err := e.lib.resolveDataSource(source)
	if err != nil {
		return nil, err
	}

	handle, err := e.newSoundObject()
	if err != nil {
		if wrapper != nil {
			_ = wrapper.Close()
		}
		return nil, err
	}

	result := e.lib.bindings.maSoundInitFromDataSource(
		e.handle, native, uint32(config.Flags), soundGroupHandle(config.Group), handle)
	if result != Success {
		e.lib.bindings.magoFree(unsafe.Pointer(handle))
		if wrapper != nil {
			_ = wrapper.Close()
		}
		return nil, e.lib.resultError("ma_sound_init_from_data_source", result)
	}

	return &Sound{
		soundCommon: soundCommon{lib: e.lib, handle: handle, engine: e},
		source:      source,
		wrapper:     wrapper,
	}, nil
}

// NewCopy makes a second sound from an existing one, sharing the same data
// source but with its own playback state.
func (s *Sound) NewCopy(config SoundConfig) (*Sound, error) {
	if s == nil || s.engine == nil {
		return nil, errNilSound
	}
	if err := s.ensure(); err != nil {
		return nil, err
	}

	handle, err := s.engine.newSoundObject()
	if err != nil {
		return nil, err
	}

	result := s.lib.bindings.maSoundInitCopy(s.engine.handle, s.handle, uint32(config.Flags), soundGroupHandle(config.Group), handle)
	if result != Success {
		s.lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, s.lib.resultError("ma_sound_init_copy", result)
	}

	// The copy shares the original's data source, so it does not own the wrapper.
	return &Sound{soundCommon: soundCommon{lib: s.lib, handle: handle, engine: s.engine}, source: s.source}, nil
}

// DataSource reports the source the sound was built from, or nil for a sound
// loaded from a file.
func (s *Sound) DataSource() DataSource {
	if s == nil {
		return nil
	}
	return s.source
}

// SetLooping makes the sound restart when it reaches the end.
func (s *Sound) SetLooping(looping bool) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.lib.bindings.maSoundSetLooping(s.handle, boolToBool32(looping))
	return nil
}

// IsLooping reports whether the sound restarts when it reaches the end.
func (s *Sound) IsLooping() bool {
	if !s.usable() {
		return false
	}
	return s.lib.bindings.maSoundIsLooping(s.handle) != 0
}

// AtEnd reports whether the sound has reached the end of its data.
func (s *Sound) AtEnd() bool {
	if !s.usable() {
		return false
	}
	return s.lib.bindings.maSoundAtEnd(s.handle) != 0
}

// SeekToPCMFrame moves the sound's read cursor.
func (s *Sound) SeekToPCMFrame(frameIndex uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.lib.resultError("ma_sound_seek_to_pcm_frame", s.lib.bindings.maSoundSeekToPCMFrame(s.handle, frameIndex))
}

// SeekToDuration moves the read cursor to a point in time, using the sound's own
// sample rate rather than the engine's.
func (s *Sound) SeekToDuration(offset time.Duration) error {
	_, _, rate, err := s.DataFormat()
	if err != nil {
		return err
	}
	if rate == 0 {
		return fmt.Errorf("mago: sound has no sample rate to seek with")
	}
	return s.SeekToPCMFrame(uint64(offset) / uint64(time.Millisecond) * uint64(rate) / 1000)
}

// DataFormat reports the format, channel count and sample rate of the sound's
// data source.
func (s *Sound) DataFormat() (Format, uint32, uint32, error) {
	if !s.usable() {
		return FormatUnknown, 0, 0, errNilSound
	}

	var format Format
	var channels, rate uint32
	if result := s.lib.bindings.maSoundGetDataFormat(s.handle, &format, &channels, &rate, nil, 0); result != Success {
		return FormatUnknown, 0, 0, s.lib.resultError("ma_sound_get_data_format", result)
	}
	return format, channels, rate, nil
}

// CursorInPCMFrames reports how far into its data the sound has read.
func (s *Sound) CursorInPCMFrames() (uint64, error) {
	return s.readFrameCount("ma_sound_get_cursor_in_pcm_frames", s.lib.bindings.maSoundGetCursorInPCMFrames)
}

// LengthInPCMFrames reports how long the sound's data is.
func (s *Sound) LengthInPCMFrames() (uint64, error) {
	return s.readFrameCount("ma_sound_get_length_in_pcm_frames", s.lib.bindings.maSoundGetLengthInPCMFrames)
}

func (s *Sound) readFrameCount(op string, fn func(*soundHandle, *uint64) Result) (uint64, error) {
	if !s.usable() {
		return 0, errNilSound
	}

	var value uint64
	if result := fn(s.handle, &value); result != Success {
		return 0, s.lib.resultError(op, result)
	}
	return value, nil
}

// SetEndCallback registers a callback for when the sound reaches its end. Pass
// nil to remove it.
func (s *Sound) SetEndCallback(callback SoundCallback) error {
	if err := s.ensure(); err != nil {
		return err
	}

	if s.endToken != 0 {
		soundCallbacks.Delete(s.endToken)
		s.endToken = 0
	}

	if callback == nil {
		return s.lib.resultError("ma_sound_set_end_callback",
			s.lib.bindings.maSoundSetEndCallback(s.handle, 0, 0))
	}

	token := uintptr(soundCallbackSeq.Add(1))
	soundCallbacks.Store(token, &soundEndState{sound: s, fn: callback})

	if result := s.lib.bindings.maSoundSetEndCallback(s.handle, soundEndCallbackPtr, token); result != Success {
		soundCallbacks.Delete(token)
		return s.lib.resultError("ma_sound_set_end_callback", result)
	}
	s.endToken = token
	return nil
}

// Close uninitialises the sound and frees its native memory, along with the custom
// data source wrapper if this sound created one.
func (s *Sound) Close() error {
	if s == nil {
		return nil
	}
	if s.endToken != 0 {
		soundCallbacks.Delete(s.endToken)
		s.endToken = 0
	}

	err := s.closeSound()
	if s.wrapper != nil {
		_ = s.wrapper.Close()
		s.wrapper = nil
	}
	return err
}
