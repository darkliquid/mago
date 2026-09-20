package mago

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

// newPlayingSound creates a sound fed by a mono sine waveform and registers
// cleanup. A mono source in a stereo engine exercises mono expansion and the
// panner at the same time.
func newPlayingSound(t *testing.T, engine *Engine, amplitude float64) *Sound {
	t.Helper()

	waveform := newSineWaveform(t, engine.lib, 1, amplitude)
	sound, err := engine.NewSoundFromDataSource(waveform, SoundConfig{})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}
	t.Cleanup(func() {
		if err := sound.Close(); err != nil {
			t.Errorf("close sound: %v", err)
		}
	})

	// Sounds start stopped, so every test that expects audio has to start it.
	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return sound
}

func TestEngineSoundFromDataSourcePlays(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)
	sound := newPlayingSound(t, engine, 0.5)

	if got := sound.Engine(); got != engine {
		t.Errorf("sound engine = %p, want %p", got, engine)
	}
	if sound.DataSource() == nil {
		t.Error("DataSource returned nil for a sound built from a data source")
	}
	if !sound.SpatializationEnabled() {
		t.Error("spatialization is disabled by default")
	}

	if !sound.IsPlaying() {
		t.Error("sound is not playing after Start")
	}

	// The default is inverse attenuation with a minimum distance of one, and the
	// sound sits on the listener, so it plays at full amplitude.
	peaks := readEnginePeaks(t, engine, 512)
	if peak := peakOf(peaks); peak < 0.49 || peak > 0.51 {
		t.Errorf("sound peak = %f, want ~0.5", peak)
	}
	if math.Abs(peaks[0]-peaks[1]) > 1e-4 {
		t.Errorf("channels are unbalanced at the origin: %v", peaks)
	}

	if got := engine.Time(); got < 512 {
		t.Errorf("engine clock = %d, want at least 512", got)
	}
	if sound.TimeInPCMFrames() == 0 {
		t.Error("sound time did not advance")
	}

	if err := sound.SetVolume(0.5); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if got := sound.Volume(); got != 0.5 {
		t.Errorf("Volume = %f, want 0.5", got)
	}
	if peak := readEnginePeak(t, engine, 512); peak < 0.24 || peak > 0.26 {
		t.Errorf("half-volume peak = %f, want ~0.25", peak)
	}

	if err := sound.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if sound.IsPlaying() {
		t.Error("sound still playing after Stop")
	}
	if peak := readEnginePeak(t, engine, 256); peak != 0 {
		t.Errorf("peak after Stop = %f, want 0", peak)
	}

	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !sound.IsPlaying() {
		t.Error("sound not playing after Start")
	}
}

func TestSoundGroupGainCascade(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)

	// The reference is an ungrouped sound at full volume.
	reference := newPlayingSound(t, engine, 0.5)
	referencePeak := readEnginePeak(t, engine, 512)
	if err := reference.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	engine2 := newDeviceLessEngine(t, 2)

	outer, err := engine2.NewSoundGroup(SoundGroupConfig{})
	if err != nil {
		t.Fatalf("NewSoundGroup: %v", err)
	}
	t.Cleanup(func() {
		if err := outer.Close(); err != nil {
			t.Errorf("close outer group: %v", err)
		}
	})

	inner, err := engine2.NewSoundGroup(SoundGroupConfig{Parent: outer})
	if err != nil {
		t.Fatalf("NewSoundGroup: %v", err)
	}
	t.Cleanup(func() {
		if err := inner.Close(); err != nil {
			t.Errorf("close inner group: %v", err)
		}
	})

	waveform := newSineWaveform(t, engine2.lib, 1, 0.5)
	sound, err := engine2.NewSoundFromDataSource(waveform, SoundConfig{Group: inner})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}
	t.Cleanup(func() {
		if err := sound.Close(); err != nil {
			t.Errorf("close sound: %v", err)
		}
	})

	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := outer.SetVolume(0.5); err != nil {
		t.Fatalf("outer SetVolume: %v", err)
	}
	if err := inner.SetVolume(0.5); err != nil {
		t.Fatalf("inner SetVolume: %v", err)
	}
	if err := sound.SetVolume(0.5); err != nil {
		t.Fatalf("sound SetVolume: %v", err)
	}

	if got := outer.Engine(); got != engine2 {
		t.Errorf("group engine = %p, want %p", got, engine2)
	}
	if got := sound.Volume(); got != 0.5 {
		t.Errorf("sound volume = %f, want 0.5", got)
	}
	if got := inner.Volume(); got != 0.5 {
		t.Errorf("inner group volume = %f, want 0.5", got)
	}

	// Three halvings of the reference peak.
	cascaded := readEnginePeak(t, engine2, 512)
	want := referencePeak * 0.125
	if cascaded < want*0.9 || cascaded > want*1.1 {
		t.Errorf("cascaded peak = %f, want about %f", cascaded, want)
	}

	// Muting the outer group silences everything beneath it.
	if err := outer.SetVolume(0); err != nil {
		t.Fatalf("outer SetVolume: %v", err)
	}
	if peak := readEnginePeak(t, engine2, 512); peak != 0 {
		t.Errorf("peak with a muted group = %f, want 0", peak)
	}
}

func TestSoundPan(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)
	sound := newPlayingSound(t, engine, 0.5)

	// Panning is applied on top of spatialization, but with spatialization on the
	// spatializer owns the pan, so a manually panned sound turns it off.
	if err := sound.SetSpatializationEnabled(false); err != nil {
		t.Fatalf("SetSpatializationEnabled: %v", err)
	}
	if sound.SpatializationEnabled() {
		t.Error("spatialization still enabled after disabling it")
	}

	if got := sound.PanMode(); got != PanModeBalance {
		t.Errorf("PanMode = %d, want PanModeBalance", got)
	}
	if err := sound.SetPanMode(PanModePan); err != nil {
		t.Fatalf("SetPanMode: %v", err)
	}
	if got := sound.PanMode(); got != PanModePan {
		t.Errorf("PanMode = %d, want PanModePan", got)
	}
	if err := sound.SetPanMode(PanModeBalance); err != nil {
		t.Fatalf("SetPanMode: %v", err)
	}

	if err := sound.SetPan(-1); err != nil {
		t.Fatalf("SetPan: %v", err)
	}
	if got := sound.Pan(); got != -1 {
		t.Errorf("Pan = %f, want -1", got)
	}
	leftPeaks := readEnginePeaks(t, engine, 512)
	if leftPeaks[0] <= leftPeaks[1] {
		t.Errorf("hard-left peaks are left %f, right %f; want left louder", leftPeaks[0], leftPeaks[1])
	}
	if leftPeaks[1] > 1e-4 {
		t.Errorf("hard-left right channel peak = %f, want 0", leftPeaks[1])
	}

	if err := sound.SetPan(1); err != nil {
		t.Fatalf("SetPan: %v", err)
	}
	rightPeaks := readEnginePeaks(t, engine, 512)
	if rightPeaks[1] <= rightPeaks[0] {
		t.Errorf("hard-right peaks are left %f, right %f; want right louder", rightPeaks[0], rightPeaks[1])
	}
}

func TestSoundAttenuationFollowsListener(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)
	sound := newPlayingSound(t, engine, 0.5)
	listener := engine.Listener(0)

	// An inverse model with a minimum distance of one gives a gain of
	// 1/(1+rolloff*(distance-minDistance)) once the listener is far enough away.
	if got := sound.AttenuationModel(); got != AttenuationInverse {
		t.Errorf("AttenuationModel = %d, want AttenuationInverse", got)
	}
	if got := sound.MinDistance(); got != 1 {
		t.Errorf("MinDistance = %f, want 1", got)
	}
	if got := sound.Rolloff(); got != 1 {
		t.Errorf("Rolloff = %f, want 1", got)
	}
	if got := sound.Positioning(); got != PositioningAbsolute {
		t.Errorf("Positioning = %d, want PositioningAbsolute", got)
	}

	if err := sound.SetPosition(Vec3{}); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if got := sound.Position(); got != (Vec3{}) {
		t.Errorf("Position = %+v, want the origin", got)
	}
	hear := readEnginePeak(t, engine, 512)

	// Move the listener away and the sound gets quieter, without touching it.
	if err := listener.SetPosition(Vec3{X: 4}); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if got := listener.Position(); got != (Vec3{X: 4}) {
		t.Errorf("listener Position = %+v, want {4 0 0}", got)
	}
	// The engine smooths spatialized gain changes over 8 ms by default, so let it
	// settle before measuring.
	readEnginePeak(t, engine, 2048)
	quiet := readEnginePeak(t, engine, 512)
	if quiet >= hear {
		t.Errorf("peak at distance 4 = %f, want less than %f at distance 0", quiet, hear)
	}
	if want := hear * 0.25; quiet < want*0.8 || quiet > want*1.2 {
		t.Errorf("peak at distance 4 = %f, want about %f", quiet, want)
	}

	// An explicit maximum distance cuts the sound off entirely.
	if err := sound.SetMaxDistance(2); err != nil {
		t.Fatalf("SetMaxDistance: %v", err)
	}
	if got := sound.MaxDistance(); got != 2 {
		t.Errorf("MaxDistance = %f, want 2", got)
	}
	// The inverse model clamps the distance to the maximum, so a listener at 4 with
	// a maximum of 2 is attenuated as if it were at 2: a gain of 0.5, not 0.25.
	readEnginePeak(t, engine, 2048)
	clamped := readEnginePeak(t, engine, 512)
	if want := hear * 0.5; clamped < want*0.9 || clamped > want*1.1 {
		t.Errorf("peak beyond the maximum distance = %f, want about %f", clamped, want)
	}

	// Turning attenuation off brings the sound back regardless of distance.
	if err := sound.SetAttenuationModel(AttenuationNone); err != nil {
		t.Fatalf("SetAttenuationModel: %v", err)
	}
	if got := sound.AttenuationModel(); got != AttenuationNone {
		t.Errorf("AttenuationModel = %d, want AttenuationNone", got)
	}
	readEnginePeak(t, engine, 2048)
	if peak := readEnginePeak(t, engine, 512); peak < hear*0.9 {
		t.Errorf("peak with attenuation off = %f, want about %f", peak, hear)
	}

	// The remaining spatialization accessors are all round-trippable.
	if err := sound.SetPositioning(PositioningRelative); err != nil {
		t.Fatalf("SetPositioning: %v", err)
	}
	if got := sound.Positioning(); got != PositioningRelative {
		t.Errorf("Positioning = %d, want PositioningRelative", got)
	}
	if err := sound.SetMinGain(0.1); err != nil {
		t.Fatalf("SetMinGain: %v", err)
	}
	if got := sound.MinGain(); got != 0.1 {
		t.Errorf("MinGain = %f, want 0.1", got)
	}
	if err := sound.SetMaxGain(2); err != nil {
		t.Fatalf("SetMaxGain: %v", err)
	}
	if got := sound.MaxGain(); got != 2 {
		t.Errorf("MaxGain = %f, want 2", got)
	}
	if err := sound.SetRolloff(2); err != nil {
		t.Fatalf("SetRolloff: %v", err)
	}
	if got := sound.Rolloff(); got != 2 {
		t.Errorf("Rolloff = %f, want 2", got)
	}
	if err := sound.SetDopplerFactor(0.5); err != nil {
		t.Fatalf("SetDopplerFactor: %v", err)
	}
	if got := sound.DopplerFactor(); got != 0.5 {
		t.Errorf("DopplerFactor = %f, want 0.5", got)
	}
	if err := sound.SetDirectionalAttenuationFactor(0.25); err != nil {
		t.Fatalf("SetDirectionalAttenuationFactor: %v", err)
	}
	if got := sound.DirectionalAttenuationFactor(); got != 0.25 {
		t.Errorf("DirectionalAttenuationFactor = %f, want 0.25", got)
	}
	if err := sound.SetVelocity(Vec3{X: 1}); err != nil {
		t.Fatalf("SetVelocity: %v", err)
	}
	if got := sound.Velocity(); got != (Vec3{X: 1}) {
		t.Errorf("Velocity = %+v, want {1 0 0}", got)
	}
	if err := sound.SetDirection(Vec3{Y: 1}); err != nil {
		t.Fatalf("SetDirection: %v", err)
	}
	if got := sound.Direction(); got != (Vec3{Y: 1}) {
		t.Errorf("Direction = %+v, want {0 1 0}", got)
	}
	if err := sound.SetCone(1, 2, 0.5); err != nil {
		t.Fatalf("SetCone: %v", err)
	}
	inner, outer, outerGain := sound.Cone()
	if inner != 1 || outer != 2 || outerGain != 0.5 {
		t.Errorf("Cone = (%f, %f, %f), want (1, 2, 0.5)", inner, outer, outerGain)
	}
	if got := sound.ListenerIndex(); got != 0 {
		t.Errorf("ListenerIndex = %d, want 0", got)
	}
	if err := sound.SetPinnedListenerIndex(0); err != nil {
		t.Fatalf("SetPinnedListenerIndex: %v", err)
	}
	if got := sound.PinnedListenerIndex(); got != 0 {
		t.Errorf("PinnedListenerIndex = %d, want 0", got)
	}
	_ = sound.DirectionToListener()
}

func TestSoundSchedulingAndFades(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)
	sound := newPlayingSound(t, engine, 0.5)

	const block = 480 // 10 ms at 48 kHz.

	// Fade in from silence over one block: the first block is quieter than the
	// second, and by the third the fade is done.
	if err := sound.SetFadeInDuration(0, 1, 10*time.Millisecond); err != nil {
		t.Fatalf("SetFadeInDuration: %v", err)
	}
	fading := readEnginePeak(t, engine, block)
	if err := sound.SetFade(1, 1, 0); err != nil {
		t.Fatalf("SetFade: %v", err)
	}
	settled := readEnginePeak(t, engine, block*2)
	if fading >= settled {
		t.Errorf("fade-in peak = %f, want less than the settled peak of %f", fading, settled)
	}
	if got := sound.CurrentFadeVolume(); math.Abs(float64(got)-1) > 1e-3 {
		t.Errorf("CurrentFadeVolume = %f, want 1 after the fade finished", got)
	}
	if err := sound.ResetFade(); err != nil {
		t.Fatalf("ResetFade: %v", err)
	}

	// A scheduled stop silences the sound once the clock passes it. The time is
	// absolute, so it is expressed relative to where the engine is now.
	if err := sound.SetStopTime(engine.Time() + 2*block); err != nil {
		t.Fatalf("SetStopTime: %v", err)
	}
	readEnginePeak(t, engine, block*4)
	if peak := readEnginePeak(t, engine, block); peak != 0 {
		t.Errorf("peak after the scheduled stop = %f, want 0", peak)
	}
	if err := sound.ResetStopTimeAndFade(); err != nil {
		t.Fatalf("ResetStopTimeAndFade: %v", err)
	}
	if err := sound.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}

	// Fading a stop out is scheduled on the engine's clock too, and resetting it
	// puts the sound back.
	if err := sound.SetStopTimeWithFade(engine.Time()+(block*4), block); err != nil {
		t.Fatalf("SetStopTimeWithFade: %v", err)
	}
	if err := sound.ResetStopTimeAndFade(); err != nil {
		t.Fatalf("ResetStopTimeAndFade: %v", err)
	}
	if err := sound.StopWithFadeInDuration(10 * time.Millisecond); err != nil {
		t.Fatalf("StopWithFadeInDuration: %v", err)
	}
	// Let the fade run out first, then check that the sound really stopped. A peak
	// measured across the fade itself would still see the loud start.
	readEnginePeak(t, engine, block*2)
	if peak := readEnginePeak(t, engine, block); peak != 0 {
		t.Errorf("peak after the fade-out = %f, want 0", peak)
	}

	// A scheduled start holds a sound back until an absolute point on the clock.
	// miniaudio only consults the schedule for a node that is marked started, so
	// the sound has to be started first.
	scheduled := newSound(t, engine, 0.5)
	if err := scheduled.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := scheduled.SetStartTimeInDuration(time.Duration(engine.Time()+4*block) * time.Second / 48000); err != nil {
		t.Fatalf("SetStartTimeInDuration: %v", err)
	}
	if peak := readEnginePeak(t, engine, block*2); peak != 0 {
		t.Errorf("peak before the scheduled start = %f, want 0", peak)
	}
	readEnginePeak(t, engine, block*2)
	if peak := readEnginePeak(t, engine, block); peak < 0.4 {
		t.Errorf("peak after the scheduled start = %f, want ~0.5", peak)
	}
	if err := scheduled.ResetStartTime(); err != nil {
		t.Fatalf("ResetStartTime: %v", err)
	}
}

// newSound creates a sound without starting it, which is what the scheduling test
// needs.
func newSound(t *testing.T, engine *Engine, amplitude float64) *Sound {
	t.Helper()

	waveform := newSineWaveform(t, engine.lib, 1, amplitude)
	sound, err := engine.NewSoundFromDataSource(waveform, SoundConfig{})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}
	t.Cleanup(func() {
		if err := sound.Close(); err != nil {
			t.Errorf("close sound: %v", err)
		}
	})
	return sound
}

func TestSoundEndCallback(t *testing.T) {
	engine := newDeviceLessEngine(t, 1)

	// A finite source is needed: an endless waveform never ends.
	frames := uint64(4800)
	source := &countingSource{frames: frames}
	sound, err := engine.NewSoundFromDataSource(source, SoundConfig{})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}
	t.Cleanup(func() {
		if err := sound.Close(); err != nil {
			t.Errorf("close sound: %v", err)
		}
	})

	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ended := make(chan struct{}, 1)
	if err := sound.SetEndCallback(func(*Sound) {
		select {
		case ended <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("SetEndCallback: %v", err)
	}

	// Read past the end of the source. The callback fires on the mixing thread,
	// which for a device-less engine is this goroutine.
	fired := false
	for i := 0; i < 20 && !fired; i++ {
		readEnginePeak(t, engine, 512)
		select {
		case <-ended:
			fired = true
		default:
		}
	}
	if !fired {
		select {
		case <-ended:
		case <-time.After(2 * time.Second):
			t.Fatal("end callback did not fire")
		}
	}

	if err := sound.SetEndCallback(nil); err != nil {
		t.Fatalf("SetEndCallback(nil): %v", err)
	}
	if err := sound.SetLooping(true); err != nil {
		t.Fatalf("SetLooping: %v", err)
	}
	if !sound.IsLooping() {
		t.Error("IsLooping = false after SetLooping(true)")
	}
	if err := sound.SetLooping(false); err != nil {
		t.Fatalf("SetLooping: %v", err)
	}
	if err := sound.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	cursor, err := sound.CursorInPCMFrames()
	if err != nil {
		t.Fatalf("CursorInPCMFrames: %v", err)
	}
	if cursor != 0 {
		t.Errorf("cursor after seeking to 0 = %d, want 0", cursor)
	}
	length, err := sound.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != frames {
		t.Errorf("LengthInPCMFrames = %d, want %d", length, frames)
	}
	if err := sound.SeekToDuration(100 * time.Millisecond); err != nil {
		t.Fatalf("SeekToDuration: %v", err)
	}
	cursor, err = sound.CursorInPCMFrames()
	if err != nil {
		t.Fatalf("CursorInPCMFrames: %v", err)
	}
	if cursor != 4800 {
		t.Errorf("cursor after seeking 100ms = %d, want 4800", cursor)
	}
}

func TestSoundFromFileAndPlaySound(t *testing.T) {
	engine := newDeviceLessEngine(t, 1)
	path := writeTestWAV(t, engine.lib, 1, 4800)

	sound, err := engine.NewSoundFromFile(path, SoundConfig{Flags: SoundFlagDecode})
	if err != nil {
		t.Fatalf("NewSoundFromFile: %v", err)
	}
	t.Cleanup(func() {
		if err := sound.Close(); err != nil {
			t.Errorf("close sound: %v", err)
		}
	})

	if sound.DataSource() != nil {
		t.Error("DataSource is non-nil for a file-backed sound")
	}
	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	length, err := sound.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != 4800 {
		t.Errorf("LengthInPCMFrames = %d, want 4800", length)
	}
	if peak := readEnginePeak(t, engine, 512); peak < 0.2 {
		t.Errorf("file-backed sound peak = %f, want the encoded tone", peak)
	}

	// The sound can be copied, sharing the decoded data but not the play state.
	copySound, err := sound.NewCopy(SoundConfig{})
	if err != nil {
		t.Fatalf("NewCopy: %v", err)
	}
	if err := copySound.Close(); err != nil {
		t.Fatalf("close copy: %v", err)
	}

	// Once the original is closed the engine falls silent again.
	if err := sound.Close(); err != nil {
		t.Fatalf("close sound: %v", err)
	}
	if peaks, _ := readEngine(t, engine, 256); peakOf(peaks) != 0 {
		t.Errorf("peak after closing the sound = %f, want 0", peakOf(peaks))
	}
	if err := sound.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// PlaySound is fire and forget: the engine owns the sound.
	if err := engine.PlaySound(path, nil); err != nil {
		t.Fatalf("PlaySound: %v", err)
	}
	if peak := readEnginePeak(t, engine, 512); peak < 0.2 {
		t.Errorf("PlaySound peak = %f, want the encoded tone", peak)
	}

	// Missing files are reported rather than ignored.
	if err := engine.PlaySound(filepath.Join(t.TempDir(), "absent.wav"), nil); err == nil {
		t.Error("PlaySound accepted a missing file")
	}
	if _, err := engine.NewSoundFromFile(filepath.Join(t.TempDir(), "absent.wav"), SoundConfig{}); err == nil {
		t.Error("NewSoundFromFile accepted a missing file")
	}
	if _, err := engine.NewSoundFromFile("", SoundConfig{}); err == nil {
		t.Error("NewSoundFromFile accepted an empty path")
	}
}

// writeTestWAV encodes a short tone to a temporary WAV file and returns its path.
func writeTestWAV(t *testing.T, lib *Library, channels uint32, frames int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tone.wav")
	encoder, err := lib.NewEncoderFile(path, DefaultEncoderConfig(channels, 48000))
	if err != nil {
		t.Fatalf("NewEncoderFile: %v", err)
	}

	samples := make([]float32, frames*int(channels))
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i/int(channels))/48000))
	}
	if _, err := encoder.WriteF32(samples); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close encoder: %v", err)
	}
	return path
}

func TestSoundRejectsNonFloatDataSource(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)

	if _, err := engine.NewSoundFromDataSource(s16Source{}, SoundConfig{}); err == nil {
		t.Error("NewSoundFromDataSource accepted an s16 data source")
	}
	if _, err := engine.NewSoundFromDataSource(nil, SoundConfig{}); err == nil {
		t.Error("NewSoundFromDataSource accepted a nil data source")
	}
}

// wrappedSource is a Go-only DataSource, so the engine's sound has to register it
// through the bridge instead of reading a native object.
type wrappedSource struct {
	frames uint64
	sample uint64
}

func (s *wrappedSource) DataFormat() (Format, uint32, uint32, error) { return FormatF32, 1, 48000, nil }
func (s *wrappedSource) CursorInPCMFrames() (uint64, error)          { return s.sample, nil }
func (s *wrappedSource) LengthInPCMFrames() (uint64, error)          { return s.frames, nil }
func (s *wrappedSource) SetLooping(bool) error                       { return nil }

func (s *wrappedSource) SeekToPCMFrame(frameIndex uint64) error {
	s.sample = frameIndex
	return nil
}

func (s *wrappedSource) ReadPCMFrames(out []byte) (uint64, error) {
	remaining := s.frames - s.sample
	if remaining == 0 {
		return 0, nil
	}
	frames := uint64(len(out)) / 4
	if frames > remaining {
		frames = remaining
	}
	for i := uint64(0); i < frames; i++ {
		value := float32(0.5)
		if (s.sample+i)%2 == 1 {
			value = -0.5
		}
		putFloat32(out[i*4:], value)
	}
	s.sample += frames
	return frames, nil
}

func putFloat32(out []byte, value float32) {
	bits := math.Float32bits(value)
	out[0] = byte(bits)
	out[1] = byte(bits >> 8)
	out[2] = byte(bits >> 16)
	out[3] = byte(bits >> 24)
}

func TestSoundWrapsGoDataSource(t *testing.T) {
	engine := newDeviceLessEngine(t, 1)
	source := &wrappedSource{frames: 48000}

	sound, err := engine.NewSoundFromDataSource(source, SoundConfig{})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}

	if got := sound.DataSource(); got != DataSource(source) {
		t.Errorf("DataSource = %v, want the registered Go source", got)
	}
	if err := sound.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if peak := readEnginePeak(t, engine, 480); peak < 0.45 || peak > 0.55 {
		t.Errorf("peak from a Go source = %f, want ~0.5", peak)
	}

	// Closing the sound also releases the registration the engine created.
	if err := sound.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := sound.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestSoundLifecycleAfterEngineClose(t *testing.T) {
	lib := newNullLibrary(t)

	config := DefaultEngineConfig(1, 48000)
	config.NoDevice = true
	engine, err := lib.NewEngine(config)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	waveform := newSineWaveform(t, lib, 1, 0.5)
	sound, err := engine.NewSoundFromDataSource(waveform, SoundConfig{})
	if err != nil {
		t.Fatalf("NewSoundFromDataSource: %v", err)
	}

	if err := engine.Close(); err != nil {
		t.Fatalf("close engine: %v", err)
	}
	if err := engine.Close(); err != nil {
		t.Fatalf("second engine close: %v", err)
	}

	// A sound whose engine is gone refuses to do anything rather than crashing.
	if err := sound.SetVolume(0.5); err == nil {
		t.Error("SetVolume on a sound whose engine is closed was allowed")
	}
	if err := sound.Start(); err == nil {
		t.Error("Start on a sound whose engine is closed was allowed")
	}
	if !sound.IsPlaying() && sound.Volume() != 0 {
		t.Error("a closed engine left the sound reporting an active volume")
	}
	if err := sound.Close(); err != nil {
		t.Fatalf("close sound after engine: %v", err)
	}
	if err := sound.Close(); err != nil {
		t.Fatalf("second sound close: %v", err)
	}
}

func TestEngineErrorsOnClosedLibrary(t *testing.T) {
	lib := newNullLibrary(t)
	engine := newDeviceLessEngine(t, 1)
	_ = engine

	if err := lib.Close(); err != nil {
		t.Fatalf("close library: %v", err)
	}
	if _, err := lib.NewEngine(DefaultEngineConfig(1, 48000)); err == nil {
		t.Error("NewEngine was allowed on a closed library")
	}
}
