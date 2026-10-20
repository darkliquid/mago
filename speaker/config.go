package speaker

import (
	"strings"

	"github.com/darkliquid/mago"
)

// Config controls how InitWithConfig opens playback. The zero value is valid
// and matches Init's defaults: it leaves backend selection to the library,
// picks the default playback device, and uses mago's default performance and
// share modes.
type Config struct {
	Backends           []mago.Backend
	DeviceIndex        int
	DeviceName         string
	PerformanceProfile mago.PerformanceProfile
	ShareMode          mago.ShareMode
}

func findDeviceIndex(devices []mago.DeviceInfo, name string) int {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		for i, device := range devices {
			if device.IsDefault {
				return i
			}
		}
		if len(devices) == 0 {
			return -1
		}
		return 0
	}
	for i, device := range devices {
		if strings.Contains(strings.ToLower(device.Name), needle) {
			return i
		}
	}
	return -1
}
