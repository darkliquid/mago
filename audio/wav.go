package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type wavFormat struct {
	audioFormat   uint16
	channels      uint16
	sampleRate    uint32
	blockAlign    uint16
	bitsPerSample uint16
}

func decodeWAV(r io.Reader) (*Clip, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("audio: only RIFF/WAVE streams are supported")
	}

	offset := 12
	var format wavFormat
	var pcm []byte
	seenFmt := false
	for offset+8 <= len(data) {
		id := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if offset+size > len(data) {
			return nil, fmt.Errorf("audio: invalid WAV chunk size")
		}
		chunk := data[offset : offset+size]
		offset += size
		if size%2 == 1 && offset < len(data) {
			offset++
		}

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, fmt.Errorf("audio: invalid fmt chunk")
			}
			format = wavFormat{
				audioFormat:   binary.LittleEndian.Uint16(chunk[0:2]),
				channels:      binary.LittleEndian.Uint16(chunk[2:4]),
				sampleRate:    binary.LittleEndian.Uint32(chunk[4:8]),
				blockAlign:    binary.LittleEndian.Uint16(chunk[12:14]),
				bitsPerSample: binary.LittleEndian.Uint16(chunk[14:16]),
			}
			seenFmt = true
		case "data":
			pcm = chunk
		}
	}
	if !seenFmt || len(pcm) == 0 {
		return nil, fmt.Errorf("audio: missing WAV fmt or data chunk")
	}
	if format.channels == 0 || format.sampleRate == 0 || format.blockAlign == 0 {
		return nil, fmt.Errorf("audio: invalid WAV format")
	}

	frameCount := len(pcm) / int(format.blockAlign)
	samples, err := decodePCMFrames(format, pcm)
	if err != nil {
		return nil, err
	}
	return &Clip{
		samples:    samples,
		channels:   int(format.channels),
		sampleRate: int(format.sampleRate),
		frameCount: frameCount,
	}, nil
}

func decodePCMFrames(format wavFormat, pcm []byte) ([]float32, error) {
	bytesPerSample := int(format.bitsPerSample / 8)
	if bytesPerSample == 0 {
		return nil, fmt.Errorf("audio: invalid bits per sample")
	}
	totalSamples := len(pcm) / bytesPerSample
	out := make([]float32, 0, totalSamples)
	reader := bytes.NewReader(pcm)

	switch format.audioFormat {
	case 1:
		for reader.Len() > 0 {
			v, err := decodePCMInteger(reader, format.bitsPerSample)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	case 3:
		if format.bitsPerSample != 32 {
			return nil, fmt.Errorf("audio: only 32-bit float WAV is supported")
		}
		for reader.Len() > 0 {
			var bits uint32
			if err := binary.Read(reader, binary.LittleEndian, &bits); err != nil {
				return nil, err
			}
			out = append(out, math.Float32frombits(bits))
		}
	default:
		return nil, fmt.Errorf("audio: unsupported WAV format code %d", format.audioFormat)
	}

	return out, nil
}

func decodePCMInteger(r io.Reader, bits uint16) (float32, error) {
	switch bits {
	case 8:
		var v uint8
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return (float32(v) - 128) / 128, nil
	case 16:
		var v int16
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return float32(v) / 32768, nil
	case 24:
		var buf [3]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		value := int32(buf[0]) | int32(buf[1])<<8 | int32(buf[2])<<16
		if value&0x800000 != 0 {
			value |= ^0xffffff
		}
		return float32(value) / 8388608, nil
	case 32:
		var v int32
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return float32(v) / 2147483648, nil
	default:
		return 0, fmt.Errorf("audio: unsupported PCM bit depth %d", bits)
	}
}
