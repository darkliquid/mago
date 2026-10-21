// Command speaker-headless shows the beep-compatible speaker running with no
// audio hardware: InitWithConfig selects the null backend, so playback is
// driven by miniaudio's null device and never touches a real sound card. It is
// the same API an application uses to pick a real backend when one is present.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/speaker"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/generators"
)

func main() {
	// The null backend renders frames without an audio device, which is what
	// lets CI and headless boxes exercise exactly this beep playback pipeline.
	err := speaker.InitWithConfig(48000, 512, speaker.Config{
		Backends:    []mago.Backend{mago.BackendNull},
		DeviceIndex: -1,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer speaker.Close()

	// A short 440 Hz tone. SineTone is an endless generator, so bound it to a
	// finite duration with Take; otherwise PlayAndWait below would never see the
	// stream end and would block forever.
	tone, err := generators.SineTone(beep.SampleRate(48000), 440)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	finite := beep.Take(48000/2, &effects.Volume{Streamer: tone, Base: 2, Volume: -6, Silent: false})
	speaker.Play(finite)
	speaker.PlayAndWait(finite)
	<-time.After(100 * time.Millisecond)
	fmt.Println("played 440 Hz through the null backend")
}
