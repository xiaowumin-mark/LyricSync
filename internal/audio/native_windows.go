//go:build windows && cgo

package audio

import (
	"context"
	"encoding/base64"
	"math"
	"strings"

	suiteaudio "github.com/xiaowumin-mark/smtc-suite-go/pkg/audio"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/audio/loopback"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

type NativeProvider struct {
	state *core.State
}

func NewNativeProvider(state *core.State) *NativeProvider {
	return &NativeProvider{state: state}
}

func (p *NativeProvider) Start(ctx context.Context) error {
	cfg := p.state.Config().Audio
	if !cfg.Enabled {
		p.state.SetService("audio", "disabled", "audio sync disabled")
		return nil
	}

	capturer, err := loopback.New(&loopback.Config{EventBuffer: 128})
	if err != nil {
		return err
	}

	format := capturer.Format()
	if !suiteaudio.CanConvertToFloat32(format) {
		_ = capturer.Close()
		return errUnsupportedFormat(format)
	}

	if err := capturer.Start(); err != nil {
		_ = capturer.Close()
		return err
	}

	p.state.SetService("audio", "running", "native WASAPI loopback active")
	p.state.AddLog("info", "audio", "native WASAPI loopback capture started")

	go func() {
		defer func() {
			_ = capturer.Close()
			p.state.SetService("audio", "stopped", "native WASAPI loopback stopped")
		}()

		var seq uint64
		var sampleBuf []float32
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-capturer.Errors():
				if ok && err != nil {
					p.state.AddLog("error", "audio", err.Error())
				}
			case frame, ok := <-capturer.Frames():
				if !ok {
					return
				}
				seq++
				samples, err := suiteaudio.ConvertToFloat32(frame.Format, frame.Data, sampleBuf)
				if err != nil {
					p.state.AddLog("warn", "audio", err.Error())
					continue
				}
				sampleBuf = samples[:0]
				p.state.SetAudioFrame(p.frameFromPCM(seq, frame, samples))
			}
		}
	}()

	return nil
}

func (p *NativeProvider) frameFromPCM(seq uint64, frame loopback.Frame, samples []float32) model.AudioFrame {
	cfg := p.state.Config().Audio
	snapshot := p.state.Snapshot()
	rms, peak := level(samples)
	duration := 0
	if frame.Format.SampleRate > 0 {
		duration = int(float64(frame.Frames) / float64(frame.Format.SampleRate) * 1000)
	}

	result := model.AudioFrame{
		Sequence:     seq,
		Timestamp:    model.FormatTime(frame.Timestamp),
		TrackID:      snapshot.Track.ID,
		PositionMs:   snapshot.Playback.Position,
		SampleRate:   frame.Format.SampleRate,
		Channels:     frame.Format.Channels,
		Format:       formatName(frame.Format),
		DurationMs:   duration,
		RMS:          rms,
		Peak:         peak,
		PCM:          append([]byte(nil), frame.Data...),
		Provider:     "smtc-suite-go",
		ProviderMode: "wasapi-loopback",
	}

	if cfg.IncludeFeatures {
		result.Spectrum = spectrumFromSamples(samples, 32, frame.Format.Channels, frame.Format.SampleRate)
	}
	if cfg.IncludePCM {
		result.PCMBase64 = base64.StdEncoding.EncodeToString(frame.Data)
	}
	return result
}

func level(samples []float32) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	var sum float64
	var peak float64
	for _, sample := range samples {
		value := float64(sample)
		abs := math.Abs(value)
		if abs > peak {
			peak = abs
		}
		sum += value * value
	}
	return math.Sqrt(sum / float64(len(samples))), peak
}

func spectrumFromSamples(samples []float32, bins, channels, sampleRate int) []float64 {
	values := make([]float64, bins)
	if len(samples) == 0 || bins <= 0 || sampleRate <= 0 {
		return values
	}
	if channels <= 0 {
		channels = 1
	}
	frames := len(samples) / channels
	if frames < 16 {
		return values
	}
	windowSize := frames
	if windowSize > 1024 {
		windowSize = 1024
	}
	startFrame := frames - windowSize
	windowSum := 0.0
	if windowSize > 1 {
		windowSum = float64(windowSize-1) / 2
	}
	if windowSum <= 0 {
		windowSum = float64(windowSize)
	}
	minFreq := 45.0
	maxFreq := math.Min(float64(sampleRate)/2, 16000)
	if maxFreq <= minFreq {
		maxFreq = float64(sampleRate) / 2
	}
	for i := 0; i < bins; i++ {
		ratio := 0.0
		if bins > 1 {
			ratio = float64(i) / float64(bins-1)
		}
		freq := minFreq * math.Pow(maxFreq/minFreq, ratio)
		angleStep := 2 * math.Pi * freq / float64(sampleRate)
		var real, imag float64
		for n := 0; n < windowSize; n++ {
			frameIndex := startFrame + n
			var mono float64
			for ch := 0; ch < channels; ch++ {
				mono += float64(samples[frameIndex*channels+ch])
			}
			mono /= float64(channels)
			window := 0.5 - 0.5*math.Cos(2*math.Pi*float64(n)/float64(windowSize-1))
			angle := angleStep * float64(n)
			real += mono * window * math.Cos(angle)
			imag -= mono * window * math.Sin(angle)
		}
		magnitude := 2 * math.Hypot(real, imag) / windowSum
		values[i] = visualEnergy(magnitude)
	}
	return values
}

func visualEnergy(value float64) float64 {
	if value <= 0.0004 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	scaled := math.Log10(1+value*180) / math.Log10(181)
	if scaled > 1 {
		scaled = 1
	}
	return math.Round(scaled*1000) / 1000
}

func formatName(format suiteaudio.Format) string {
	name := strings.ToLower(format.SampleFormat.String())
	switch name {
	case "float32":
		return "f32le"
	case "int16":
		return "s16le"
	case "int24":
		return "s24le"
	case "int32":
		return "s32le"
	default:
		return name
	}
}

func errUnsupportedFormat(format suiteaudio.Format) error {
	return &unsupportedFormatError{format: format}
}

type unsupportedFormatError struct {
	format suiteaudio.Format
}

func (e *unsupportedFormatError) Error() string {
	return "audio: unsupported loopback format " + e.format.SampleFormat.String()
}
