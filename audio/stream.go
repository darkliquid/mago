package audio

import (
	"fmt"
	"time"
)

// Duration returns the clip duration based on the decoded frame count and sample rate.
func (c *Clip) Duration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return durationFromFrames(c.frameCount, c.sampleRate)
}

// SampleRate returns the decoded sample rate of the clip.
func (c *Clip) SampleRate() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sampleRate
}

// Channels returns the decoded channel count of the clip.
func (c *Clip) Channels() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channels
}

// Release frees the clip's decoded sample buffer.
//
// Streams that still refer to the clip will stop once the released buffer is observed.
func (c *Clip) Release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samples = nil
	c.frameCount = 0
}

// Start starts or restarts playback for the stream.
func (s *Stream) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == streamClosed {
		return fmt.Errorf("audio: stream is closed")
	}
	s.state = streamPlaying
	return nil
}

// Pause pauses playback without resetting the current stream position.
func (s *Stream) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == streamPlaying {
		s.state = streamPaused
	}
}

// Resume resumes playback from the current position.
func (s *Stream) Resume() error {
	return s.Start()
}

// Stop stops playback and resets the stream position to the start or end,
// depending on the current playback direction.
func (s *Stream) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == streamClosed {
		return
	}
	s.state = streamStopped
	frameCount := s.clipFrameCountLocked()
	s.resetPositionLocked(frameCount)
	s.fade = fadeState{}
}

// Close removes the stream from the engine mixer and marks it unusable.
func (s *Stream) Close() {
	if s == nil || s.engine == nil {
		return
	}
	s.engine.mu.Lock()
	delete(s.engine.streams, s)
	s.engine.mu.Unlock()

	s.mu.Lock()
	s.state = streamClosed
	s.fade = fadeState{}
	s.mu.Unlock()
}

// SetLooping enables or disables looping for the stream.
func (s *Stream) SetLooping(loop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.looping = loop
}

// SetVolume sets the stream gain used by the software mixer.
func (s *Stream) SetVolume(volume float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volume = volume
}

// SetSpeed updates the playback speed multiplier.
//
// Values greater than 1 play faster, values between 0 and 1 play slower.
func (s *Stream) SetSpeed(speed float64) error {
	if speed <= 0 {
		return fmt.Errorf("audio: speed must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.speed = speed
	return nil
}

// SetReverse switches the stream between forward and reverse playback.
func (s *Stream) SetReverse(reverse bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reverse == reverse {
		return
	}
	s.reverse = reverse
	frameCount := s.clipFrameCountLocked()
	if frameCount > 0 {
		if reverse && s.position <= 0 {
			s.position = float64(frameCount - 1)
		}
		if !reverse && s.position >= float64(frameCount) {
			s.position = 0
		}
	}
}

// Seek moves the playback cursor to the requested offset within the clip.
//
// Offsets outside the clip duration are clamped to the valid range.
func (s *Stream) Seek(offset time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, sampleRate, frameCount := s.clipSnapshotLocked()
	if sampleRate == 0 || frameCount == 0 {
		return fmt.Errorf("audio: clip is released")
	}
	frame := int(float64(offset) / float64(time.Second) * float64(sampleRate))
	if frame < 0 {
		frame = 0
	}
	if frame >= frameCount {
		frame = frameCount - 1
	}
	s.position = float64(frame)
	return nil
}

// Position returns the current playback position within the clip.
func (s *Stream) Position() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, sampleRate, _ := s.clipSnapshotLocked()
	if sampleRate == 0 {
		return 0
	}
	return time.Duration((s.position / float64(sampleRate)) * float64(time.Second))
}

// FadeTo schedules a volume fade from the current gain to volume over duration.
func (s *Stream) FadeTo(volume float64, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fadeLocked(s.volume, volume, duration, false)
}

// FadeOutAndStop schedules a fade to silence and stops the stream when it completes.
func (s *Stream) FadeOutAndStop(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fadeLocked(s.volume, 0, duration, true)
}

func (s *Stream) fadeLocked(start, end float64, duration time.Duration, stopWhenDone bool) {
	if duration <= 0 || s.engine == nil || s.engine.sampleRate == 0 {
		s.volume = end
		if stopWhenDone {
			s.state = streamStopped
			frameCount := s.clipFrameCountLocked()
			s.resetPositionLocked(frameCount)
		}
		return
	}
	s.fade = fadeState{
		active:       true,
		start:        start,
		end:          end,
		remaining:    int(float64(duration) / float64(time.Second) * float64(s.engine.sampleRate)),
		total:        int(float64(duration) / float64(time.Second) * float64(s.engine.sampleRate)),
		stopWhenDone: stopWhenDone,
	}
	if s.fade.remaining <= 0 {
		s.fade.remaining = 1
		s.fade.total = 1
	}
}

func (s *Stream) mixInto(out []float32, outFrames int, outChannels int, outRate int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == streamClosed {
		return false
	}
	if s.state != streamPlaying {
		return true
	}

	samples, channels, sampleRate, frameCount := s.clipSnapshotLocked()
	if sampleRate == 0 || frameCount == 0 {
		s.state = streamStopped
		return false
	}
	if s.speed <= 0 {
		s.speed = 1
	}

	step := s.speed * float64(sampleRate) / float64(outRate)
	for frame := 0; frame < outFrames; frame++ {
		position, ok := s.ensurePlayablePositionLocked(frameCount)
		if !ok {
			break
		}

		gain := s.currentGainLocked()
		for ch := 0; ch < outChannels; ch++ {
			out[frame*outChannels+ch] += gain * interpolateSample(samples, channels, frameCount, position, ch, outChannels)
		}

		if s.reverse {
			s.position -= step
		} else {
			s.position += step
		}
		s.advanceFadeLocked(frameCount)
	}
	return true
}

func (s *Stream) currentGainLocked() float32 {
	gain := s.volume
	if s.fade.active && s.fade.total > 0 {
		progress := 1 - float64(s.fade.remaining)/float64(s.fade.total)
		gain = s.fade.start + (s.fade.end-s.fade.start)*progress
	}
	return float32(gain)
}

func (s *Stream) advanceFadeLocked(frameCount int) {
	if !s.fade.active {
		return
	}
	s.fade.remaining--
	if s.fade.remaining > 0 {
		return
	}
	s.volume = s.fade.end
	stopWhenDone := s.fade.stopWhenDone
	s.fade = fadeState{}
	if stopWhenDone {
		s.state = streamStopped
		s.resetPositionLocked(frameCount)
	}
}

func (s *Stream) ensurePlayablePositionLocked(frameCount int) (float64, bool) {
	if frameCount == 0 {
		return 0, false
	}
	if s.looping {
		for s.position < 0 {
			s.position += float64(frameCount)
		}
		for s.position >= float64(frameCount) {
			s.position -= float64(frameCount)
		}
		return s.position, true
	}
	if s.position < 0 || s.position >= float64(frameCount) {
		s.state = streamStopped
		s.resetPositionLocked(frameCount)
		return 0, false
	}
	return s.position, true
}

func (s *Stream) resetPositionLocked(frameCount int) {
	if frameCount <= 0 {
		s.position = 0
		return
	}
	if s.reverse {
		s.position = float64(frameCount - 1)
		return
	}
	s.position = 0
}

func (s *Stream) clipSnapshotLocked() ([]float32, int, int, int) {
	if s.clip == nil {
		return nil, 0, 0, 0
	}
	s.clip.mu.RLock()
	defer s.clip.mu.RUnlock()
	if len(s.clip.samples) == 0 || s.clip.frameCount == 0 {
		return nil, 0, 0, 0
	}
	return s.clip.samples, s.clip.channels, s.clip.sampleRate, s.clip.frameCount
}

func (s *Stream) clipFrameCountLocked() int {
	if s.clip == nil {
		return 0
	}
	s.clip.mu.RLock()
	defer s.clip.mu.RUnlock()
	return s.clip.frameCount
}

func interpolateSample(samples []float32, srcChannels int, frameCount int, position float64, outChannel int, outChannels int) float32 {
	if frameCount == 0 || srcChannels == 0 {
		return 0
	}
	if position < 0 {
		position = 0
	}
	if position > float64(frameCount-1) {
		position = float64(frameCount - 1)
	}
	base := int(position)
	next := base + 1
	if next >= frameCount {
		next = base
	}
	frac := float32(position - float64(base))
	v0 := channelSample(samples, srcChannels, base, outChannel, outChannels)
	v1 := channelSample(samples, srcChannels, next, outChannel, outChannels)
	return v0 + (v1-v0)*frac
}

func channelSample(samples []float32, srcChannels int, frame int, outChannel int, outChannels int) float32 {
	base := frame * srcChannels
	if srcChannels == 1 {
		return samples[base]
	}
	if outChannels == 1 {
		var sum float32
		for ch := 0; ch < srcChannels; ch++ {
			sum += samples[base+ch]
		}
		return sum / float32(srcChannels)
	}
	if outChannel < srcChannels {
		return samples[base+outChannel]
	}
	return samples[base+srcChannels-1]
}
