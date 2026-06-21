package smtc

import (
	"context"
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
	s.state.SetService("smtc", "running", "simulated SMTC provider active")
	s.state.AddLog("info", "smtc", "SMTC simulator started; native Windows provider will replace this layer")

	track := model.Track{
		ID:        "demo-track",
		Title:     "LyricSync Demo",
		Artist:    "Local Session",
		Album:     "Development Build",
		SourceApp: "SMTC Simulator",
		Duration:  184000,
	}
	s.state.SetTrack(track)
	s.state.SetSessions([]model.Session{
		{
			ID:        "demo-session",
			Name:      "SMTC Simulator",
			AppID:     "lyricsync.simulator",
			Active:    true,
			Available: true,
			UpdatedAt: model.Now(),
		},
	})

	go func() {
		start := time.Now()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.state.SetService("smtc", "stopped", "SMTC provider stopped")
				return
			case now := <-ticker.C:
				position := now.Sub(start).Milliseconds() % track.Duration
				s.state.SetPlayback(model.Playback{
					State:      "playing",
					Position:   position,
					Volume:     -1,
					UpdatedAt:  model.FormatTime(now),
					CanControl: true,
				})
			}
		}
	}()
	return nil
}
