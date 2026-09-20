package mago

import (
	"testing"
	"unsafe"

	"github.com/darkliquid/mago/internal/abi"
)

func TestMirroredStructLayouts(t *testing.T) {
	probe, err := abi.Run(".")
	if err != nil {
		t.Fatalf("layout probe: %v", err)
	}

	if got, want := uint64(unsafe.Sizeof(deviceIDNative{})), probe["sizeof:ma_device_id"]; got != want {
		t.Errorf("sizeof ma_device_id: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.Name)), probe["offsetof:ma_device_info.name"]; got != want {
		t.Errorf("offsetof ma_device_info.name: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.IsDefault)), probe["offsetof:ma_device_info.isDefault"]; got != want {
		t.Errorf("offsetof ma_device_info.isDefault: mirror %d, header %d", got, want)
	}

	if got, want := uint64(unsafe.Sizeof(Channel(0))), probe["sizeof:ma_channel"]; got != want {
		t.Errorf("sizeof ma_channel: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(channelConverterConfigNative{})), probe["sizeof:ma_channel_converter_config"]; got != want {
		t.Errorf("sizeof channelConverterConfigNative: mirror %d, header %d", got, want)
	}

	mirror := channelConverterConfigNative{}
	offsets := map[string]struct {
		got uintptr
		key string
	}{
		"pChannelMapIn":                   {unsafe.Offsetof(mirror.ChannelMapIn), "offsetof:ma_channel_converter_config.pChannelMapIn"},
		"pChannelMapOut":                  {unsafe.Offsetof(mirror.ChannelMapOut), "offsetof:ma_channel_converter_config.pChannelMapOut"},
		"mixingMode":                      {unsafe.Offsetof(mirror.MixingMode), "offsetof:ma_channel_converter_config.mixingMode"},
		"calculateLFEFromSpatialChannels": {unsafe.Offsetof(mirror.CalculateLFEFromSpatialChannels), "offsetof:ma_channel_converter_config.calculateLFEFromSpatialChannels"},
		"ppWeights":                       {unsafe.Offsetof(mirror.Weights), "offsetof:ma_channel_converter_config.ppWeights"},
	}
	for name, check := range offsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_channel_converter_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(resamplerConfigNative{})), probe["sizeof:ma_resampler_config"]; got != want {
		t.Errorf("sizeof resamplerConfigNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(linearResamplerConfigNative{})), probe["sizeof:ma_linear_resampler_config"]; got != want {
		t.Errorf("sizeof linearResamplerConfigNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(dataConverterConfigNative{})), probe["sizeof:ma_data_converter_config"]; got != want {
		t.Errorf("sizeof dataConverterConfigNative: mirror %d, header %d", got, want)
	}

	resampler := resamplerConfigNative{}
	linear := linearResamplerConfigNative{}
	converter := dataConverterConfigNative{}
	resampleOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"resampler.algorithm":        {unsafe.Offsetof(resampler.Algorithm), "offsetof:ma_resampler_config.algorithm"},
		"resampler.linear.lpfOrder":  {unsafe.Offsetof(resampler.Linear) + unsafe.Offsetof(resamplerLinearConfigNative{}.LPFOrder), "offsetof:ma_resampler_config.linear.lpfOrder"},
		"linear.lpfOrder":            {unsafe.Offsetof(linear.LPFOrder), "offsetof:ma_linear_resampler_config.lpfOrder"},
		"linear.lpfNyquistFactor":    {unsafe.Offsetof(linear.LPFNyquistFactor), "offsetof:ma_linear_resampler_config.lpfNyquistFactor"},
		"converter.pChannelMapIn":    {unsafe.Offsetof(converter.ChannelMapIn), "offsetof:ma_data_converter_config.pChannelMapIn"},
		"converter.ditherMode":       {unsafe.Offsetof(converter.DitherMode), "offsetof:ma_data_converter_config.ditherMode"},
		"converter.channelMixMode":   {unsafe.Offsetof(converter.ChannelMixMode), "offsetof:ma_data_converter_config.channelMixMode"},
		"converter.lfeFromSpatial":   {unsafe.Offsetof(converter.CalculateLFEFromSpatialChannels), "offsetof:ma_data_converter_config.calculateLFEFromSpatialChannels"},
		"converter.ppChannelWeights": {unsafe.Offsetof(converter.ChannelWeights), "offsetof:ma_data_converter_config.ppChannelWeights"},
		"converter.allowDynamicRate": {unsafe.Offsetof(converter.AllowDynamicSampleRate), "offsetof:ma_data_converter_config.allowDynamicSampleRate"},
		"converter.resampling":       {unsafe.Offsetof(converter.Resampling), "offsetof:ma_data_converter_config.resampling"},
	}
	for name, check := range resampleOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof %s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(allocationCallbacksNative{})), probe["sizeof:ma_allocation_callbacks"]; got != want {
		t.Errorf("sizeof allocationCallbacksNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(audioBufferConfigNative{})), probe["sizeof:ma_audio_buffer_config"]; got != want {
		t.Errorf("sizeof audioBufferConfigNative: mirror %d, header %d", got, want)
	}

	audioBuffer := audioBufferConfigNative{}
	bufferOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"sizeInFrames":        {unsafe.Offsetof(audioBuffer.SizeInFrames), "offsetof:ma_audio_buffer_config.sizeInFrames"},
		"pData":               {unsafe.Offsetof(audioBuffer.Data), "offsetof:ma_audio_buffer_config.pData"},
		"allocationCallbacks": {unsafe.Offsetof(audioBuffer.AllocationCallbacks), "offsetof:ma_audio_buffer_config.allocationCallbacks"},
	}
	for name, check := range bufferOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_audio_buffer_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(waveformConfigNative{})), probe["sizeof:ma_waveform_config"]; got != want {
		t.Errorf("sizeof waveformConfigNative: mirror %d, header %d", got, want)
	}
	waveformCfg := waveformConfigNative{}
	waveformOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":     {unsafe.Offsetof(waveformCfg.Format), "offsetof:ma_waveform_config.format"},
		"channels":   {unsafe.Offsetof(waveformCfg.Channels), "offsetof:ma_waveform_config.channels"},
		"sampleRate": {unsafe.Offsetof(waveformCfg.SampleRate), "offsetof:ma_waveform_config.sampleRate"},
		"type":       {unsafe.Offsetof(waveformCfg.Type), "offsetof:ma_waveform_config.type"},
		"amplitude":  {unsafe.Offsetof(waveformCfg.Amplitude), "offsetof:ma_waveform_config.amplitude"},
		"frequency":  {unsafe.Offsetof(waveformCfg.Frequency), "offsetof:ma_waveform_config.frequency"},
	}
	for name, check := range waveformOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_waveform_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(noiseConfigNative{})), probe["sizeof:ma_noise_config"]; got != want {
		t.Errorf("sizeof noiseConfigNative: mirror %d, header %d", got, want)
	}
	noiseCfg := noiseConfigNative{}
	noiseOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":            {unsafe.Offsetof(noiseCfg.Format), "offsetof:ma_noise_config.format"},
		"channels":          {unsafe.Offsetof(noiseCfg.Channels), "offsetof:ma_noise_config.channels"},
		"type":              {unsafe.Offsetof(noiseCfg.Type), "offsetof:ma_noise_config.type"},
		"seed":              {unsafe.Offsetof(noiseCfg.Seed), "offsetof:ma_noise_config.seed"},
		"amplitude":         {unsafe.Offsetof(noiseCfg.Amplitude), "offsetof:ma_noise_config.amplitude"},
		"duplicateChannels": {unsafe.Offsetof(noiseCfg.DuplicateChannels), "offsetof:ma_noise_config.duplicateChannels"},
	}
	for name, check := range noiseOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_noise_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(biquadConfigNative{})), probe["sizeof:ma_biquad_config"]; got != want {
		t.Errorf("sizeof biquadConfigNative: mirror %d, header %d", got, want)
	}
	bqCfg := biquadConfigNative{}
	bqOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":   {unsafe.Offsetof(bqCfg.Format), "offsetof:ma_biquad_config.format"},
		"channels": {unsafe.Offsetof(bqCfg.Channels), "offsetof:ma_biquad_config.channels"},
		"b0":       {unsafe.Offsetof(bqCfg.B0), "offsetof:ma_biquad_config.b0"},
		"b1":       {unsafe.Offsetof(bqCfg.B1), "offsetof:ma_biquad_config.b1"},
		"b2":       {unsafe.Offsetof(bqCfg.B2), "offsetof:ma_biquad_config.b2"},
		"a0":       {unsafe.Offsetof(bqCfg.A0), "offsetof:ma_biquad_config.a0"},
		"a1":       {unsafe.Offsetof(bqCfg.A1), "offsetof:ma_biquad_config.a1"},
		"a2":       {unsafe.Offsetof(bqCfg.A2), "offsetof:ma_biquad_config.a2"},
	}
	for name, check := range bqOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_biquad_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(lpf1ConfigNative{})), probe["sizeof:ma_lpf1_config"]; got != want {
		t.Errorf("sizeof lpf1ConfigNative: mirror %d, header %d", got, want)
	}
	lpf1Cfg := lpf1ConfigNative{}
	lpf1Offsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":          {unsafe.Offsetof(lpf1Cfg.Format), "offsetof:ma_lpf1_config.format"},
		"channels":        {unsafe.Offsetof(lpf1Cfg.Channels), "offsetof:ma_lpf1_config.channels"},
		"sampleRate":      {unsafe.Offsetof(lpf1Cfg.SampleRate), "offsetof:ma_lpf1_config.sampleRate"},
		"cutoffFrequency": {unsafe.Offsetof(lpf1Cfg.CutoffFrequency), "offsetof:ma_lpf1_config.cutoffFrequency"},
		"q":               {unsafe.Offsetof(lpf1Cfg.Q), "offsetof:ma_lpf1_config.q"},
	}
	for name, check := range lpf1Offsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_lpf1_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(lpfConfigNative{})), probe["sizeof:ma_lpf_config"]; got != want {
		t.Errorf("sizeof lpfConfigNative: mirror %d, header %d", got, want)
	}
	lpfCfg := lpfConfigNative{}
	lpfOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":          {unsafe.Offsetof(lpfCfg.Format), "offsetof:ma_lpf_config.format"},
		"channels":        {unsafe.Offsetof(lpfCfg.Channels), "offsetof:ma_lpf_config.channels"},
		"sampleRate":      {unsafe.Offsetof(lpfCfg.SampleRate), "offsetof:ma_lpf_config.sampleRate"},
		"cutoffFrequency": {unsafe.Offsetof(lpfCfg.CutoffFrequency), "offsetof:ma_lpf_config.cutoffFrequency"},
		"order":           {unsafe.Offsetof(lpfCfg.Order), "offsetof:ma_lpf_config.order"},
	}
	for name, check := range lpfOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_lpf_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(notch2ConfigNative{})), probe["sizeof:ma_notch2_config"]; got != want {
		t.Errorf("sizeof notch2ConfigNative: mirror %d, header %d", got, want)
	}
	notchCfg := notch2ConfigNative{}
	notchOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":     {unsafe.Offsetof(notchCfg.Format), "offsetof:ma_notch2_config.format"},
		"channels":   {unsafe.Offsetof(notchCfg.Channels), "offsetof:ma_notch2_config.channels"},
		"sampleRate": {unsafe.Offsetof(notchCfg.SampleRate), "offsetof:ma_notch2_config.sampleRate"},
		"q":          {unsafe.Offsetof(notchCfg.Q), "offsetof:ma_notch2_config.q"},
		"frequency":  {unsafe.Offsetof(notchCfg.Frequency), "offsetof:ma_notch2_config.frequency"},
	}
	for name, check := range notchOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_notch2_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(peak2ConfigNative{})), probe["sizeof:ma_peak2_config"]; got != want {
		t.Errorf("sizeof peak2ConfigNative: mirror %d, header %d", got, want)
	}
	peakCfg := peak2ConfigNative{}
	peakOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":     {unsafe.Offsetof(peakCfg.Format), "offsetof:ma_peak2_config.format"},
		"channels":   {unsafe.Offsetof(peakCfg.Channels), "offsetof:ma_peak2_config.channels"},
		"sampleRate": {unsafe.Offsetof(peakCfg.SampleRate), "offsetof:ma_peak2_config.sampleRate"},
		"gainDB":     {unsafe.Offsetof(peakCfg.GainDB), "offsetof:ma_peak2_config.gainDB"},
		"q":          {unsafe.Offsetof(peakCfg.Q), "offsetof:ma_peak2_config.q"},
		"frequency":  {unsafe.Offsetof(peakCfg.Frequency), "offsetof:ma_peak2_config.frequency"},
	}
	for name, check := range peakOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_peak2_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(loshelf2ConfigNative{})), probe["sizeof:ma_loshelf2_config"]; got != want {
		t.Errorf("sizeof loshelf2ConfigNative: mirror %d, header %d", got, want)
	}
	shelfCfg := loshelf2ConfigNative{}
	shelfOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":     {unsafe.Offsetof(shelfCfg.Format), "offsetof:ma_loshelf2_config.format"},
		"channels":   {unsafe.Offsetof(shelfCfg.Channels), "offsetof:ma_loshelf2_config.channels"},
		"sampleRate": {unsafe.Offsetof(shelfCfg.SampleRate), "offsetof:ma_loshelf2_config.sampleRate"},
		"gainDB":     {unsafe.Offsetof(shelfCfg.GainDB), "offsetof:ma_loshelf2_config.gainDB"},
		"shelfSlope": {unsafe.Offsetof(shelfCfg.ShelfSlope), "offsetof:ma_loshelf2_config.shelfSlope"},
		"frequency":  {unsafe.Offsetof(shelfCfg.Frequency), "offsetof:ma_loshelf2_config.frequency"},
	}
	for name, check := range shelfOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_loshelf2_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(delayConfigNative{})), probe["sizeof:ma_delay_config"]; got != want {
		t.Errorf("sizeof delayConfigNative: mirror %d, header %d", got, want)
	}
	delayCfg := delayConfigNative{}
	delayOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"channels":      {unsafe.Offsetof(delayCfg.Channels), "offsetof:ma_delay_config.channels"},
		"sampleRate":    {unsafe.Offsetof(delayCfg.SampleRate), "offsetof:ma_delay_config.sampleRate"},
		"delayInFrames": {unsafe.Offsetof(delayCfg.DelayInFrames), "offsetof:ma_delay_config.delayInFrames"},
		"delayStart":    {unsafe.Offsetof(delayCfg.DelayStart), "offsetof:ma_delay_config.delayStart"},
		"wet":           {unsafe.Offsetof(delayCfg.Wet), "offsetof:ma_delay_config.wet"},
		"dry":           {unsafe.Offsetof(delayCfg.Dry), "offsetof:ma_delay_config.dry"},
		"decay":         {unsafe.Offsetof(delayCfg.Decay), "offsetof:ma_delay_config.decay"},
	}
	for name, check := range delayOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_delay_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(decoderConfigNative{})), probe["sizeof:ma_decoder_config"]; got != want {
		t.Errorf("sizeof decoderConfigNative: mirror %d, header %d", got, want)
	}
	decoderCfg := decoderConfigNative{}
	decoderOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"pChannelMap":         {unsafe.Offsetof(decoderCfg.ChannelMap), "offsetof:ma_decoder_config.pChannelMap"},
		"channelMixMode":      {unsafe.Offsetof(decoderCfg.ChannelMixMode), "offsetof:ma_decoder_config.channelMixMode"},
		"ditherMode":          {unsafe.Offsetof(decoderCfg.DitherMode), "offsetof:ma_decoder_config.ditherMode"},
		"resampling":          {unsafe.Offsetof(decoderCfg.Resampling), "offsetof:ma_decoder_config.resampling"},
		"allocationCallbacks": {unsafe.Offsetof(decoderCfg.AllocationCallbacks), "offsetof:ma_decoder_config.allocationCallbacks"},
		"encodingFormat":      {unsafe.Offsetof(decoderCfg.EncodingFormat), "offsetof:ma_decoder_config.encodingFormat"},
		"seekPointCount":      {unsafe.Offsetof(decoderCfg.SeekPointCount), "offsetof:ma_decoder_config.seekPointCount"},
		"ppCustomBackendVTables": {
			unsafe.Offsetof(decoderCfg.CustomBackendVTables),
			"offsetof:ma_decoder_config.ppCustomBackendVTables",
		},
		"customBackendCount":     {unsafe.Offsetof(decoderCfg.CustomBackendCount), "offsetof:ma_decoder_config.customBackendCount"},
		"pCustomBackendUserData": {unsafe.Offsetof(decoderCfg.CustomBackendUserData), "offsetof:ma_decoder_config.pCustomBackendUserData"},
	}
	for name, check := range decoderOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_decoder_config.%s: mirror %d, header %d", name, got, want)
		}
	}
}
