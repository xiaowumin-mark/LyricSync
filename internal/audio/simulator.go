package audio

import (
	"context"
	"math"
	"time"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

type Simulator struct {
	state *core.State
}

func NewSimulator(state *core.State) *Simulator {
	return &Simulator{state: state}
}

func (s *Simulator) Start(ctx context.Context) error {
	cfg := s.state.Config().Audio
	if !cfg.Enabled {
		s.state.SetService("audio", "disabled", "audio sync disabled")
		return nil
	}

	interval := time.Duration(cfg.FrameDurationMs) * time.Millisecond
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}

	s.state.SetService("audio", "running", "simulated feature stream active")
	s.state.AddLog("info", "audio", "audio feature stream started in simulator mode")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var seq uint64
		start := time.Now()
		for {
			select {
			case <-ctx.Done():
				s.state.SetService("audio", "stopped", "audio sync stopped")
				return
			case now := <-ticker.C:
				seq++
				snapshot := s.state.Snapshot()
				elapsed := now.Sub(start).Seconds()
				frame := model.AudioFrame{
					Sequence:     seq,
					Timestamp:    model.FormatTime(now),
					TrackID:      snapshot.Track.ID,
					PositionMs:   snapshot.Playback.Position,
					SampleRate:   cfg.SampleRate,
					Channels:     cfg.Channels,
					Format:       cfg.Format,
					DurationMs:   cfg.FrameDurationMs,
					RMS:          0.35 + 0.16*math.Sin(elapsed*2.4),
					Peak:         0.72 + 0.12*math.Sin(elapsed*4.8),
					Spectrum:     spectrum(elapsed, 32),
					Provider:     "lyricsync",
					ProviderMode: "simulator",
				}
				s.state.SetAudioFrame(frame)
			}
		}
	}()
	return nil
}

func spectrum(t float64, bins int) []float64 {
	values := make([]float64, bins)
	for i := 0; i < bins; i++ {
		band := float64(i + 1)
		base := math.Sin(t*band*0.15) * 0.5
		pulse := math.Sin(t*2.8+band*0.35) * 0.25
		value := 0.5 + base + pulse
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		values[i] = math.Round(value*1000) / 1000
	}
	return values
}
