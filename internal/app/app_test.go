package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/song"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

func TestLyricRevisionRejectsStaleLyrics(t *testing.T) {
	store := state.New(config.Default())
	runtime := &Runtime{store: store, aiLimiter: make(chan struct{}, 1)}
	trackA := model.Track{ID: "track-a", Title: "Song A", Artist: "Artist"}
	trackB := model.Track{ID: "track-b", Title: "Song B", Artist: "Artist"}

	store.SetTrack(trackA)
	keyA := song.UniqueKey(song.InputFromTrack(trackA))
	revA := runtime.advanceLyricRevision(keyA)

	store.SetTrack(trackB)
	keyB := song.UniqueKey(song.InputFromTrack(trackB))
	revB := runtime.advanceLyricRevision(keyB)

	if runtime.setLyricsForRevision(revA, keyA, trackA, model.CurrentLyrics{TrackID: trackA.ID, Source: "stale"}) {
		t.Fatal("expected stale revision to be rejected")
	}
	if got := store.Snapshot().Lyrics.Source; got != "" {
		t.Fatalf("stale lyrics changed store source to %q", got)
	}
	if !runtime.setLyricsForRevision(revB, keyB, trackB, model.CurrentLyrics{TrackID: trackB.ID, Source: "current"}) {
		t.Fatal("expected current revision to be accepted")
	}
	if got := store.Snapshot().Lyrics.Source; got != "current" {
		t.Fatalf("current lyrics source = %q", got)
	}
}

func TestLyricRevisionRejectsSameSongDifferentTrackID(t *testing.T) {
	store := state.New(config.Default())
	runtime := &Runtime{store: store, aiLimiter: make(chan struct{}, 1)}
	trackA := model.Track{ID: "track-a", Title: "Same Song", Artist: "Artist"}
	trackB := model.Track{ID: "track-b", Title: "Same Song", Artist: "Artist"}

	store.SetTrack(trackA)
	key := song.UniqueKey(song.InputFromTrack(trackA))
	revision := runtime.advanceLyricRevision(key)

	store.SetTrack(trackB)
	if runtime.setLyricsForRevision(revision, key, trackA, model.CurrentLyrics{TrackID: trackA.ID, Source: "stale"}) {
		t.Fatal("expected same song with stale track id to be rejected")
	}
	if got := store.Snapshot().Lyrics.Source; got != "" {
		t.Fatalf("stale same-song lyrics changed store source to %q", got)
	}
}

func TestRunAIExclusiveSerializesTasks(t *testing.T) {
	runtime := &Runtime{aiLimiter: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := make(chan struct{}, 2)
	release := make(chan struct{})
	var running int32
	var maxRunning int32

	errs := make(chan error, 2)
	run := func() {
		errs <- runtime.runAIExclusive(ctx, func(ctx context.Context) error {
			current := atomic.AddInt32(&running, 1)
			for {
				max := atomic.LoadInt32(&maxRunning)
				if current <= max || atomic.CompareAndSwapInt32(&maxRunning, max, current) {
					break
				}
			}
			start <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			atomic.AddInt32(&running, -1)
			return nil
		})
	}

	go run()
	<-start
	go run()
	select {
	case <-start:
		t.Fatal("second AI task started before first released")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if max := atomic.LoadInt32(&maxRunning); max != 1 {
		t.Fatalf("max concurrent AI tasks = %d, want 1", max)
	}
}

func TestAIConfiguredRequiresProviderAndFeature(t *testing.T) {
	cfg := config.Default()
	cfg.AI.BaseURL = "https://example.test/v1"
	cfg.AI.APIKey = "key"
	cfg.AI.Model = "model"
	if aiConfigured(cfg) {
		t.Fatal("expected AI to be disabled without lyric AI feature")
	}
	cfg.Lyrics.AITranslate = true
	if !aiConfigured(cfg) {
		t.Fatal("expected AI to be enabled with provider and feature")
	}
	cfg.AI.APIKey = ""
	if aiConfigured(cfg) {
		t.Fatal("expected AI to require API key")
	}
}

func TestAIApplyWaitDurationClampsConfig(t *testing.T) {
	cfg := config.Default()
	cfg.AI.ApplyWaitSeconds = -1
	if got := aiApplyWaitDuration(cfg); got != 0 {
		t.Fatalf("negative wait = %s, want 0", got)
	}
	cfg.AI.ApplyWaitSeconds = 3
	if got := aiApplyWaitDuration(cfg); got != 3*time.Second {
		t.Fatalf("wait = %s, want 3s", got)
	}
	cfg.AI.ApplyWaitSeconds = 99
	if got := aiApplyWaitDuration(cfg); got != 15*time.Second {
		t.Fatalf("large wait = %s, want 15s", got)
	}
}

func TestAIEnhancementAllowedSourceExcludesTTMLDBAndCustom(t *testing.T) {
	blocked := []string{
		model.LyricSourceTTMLDB,
		model.LyricSourceCustom,
		"",
		"unknown",
	}
	for _, source := range blocked {
		if aiEnhancementAllowedSource(source) {
			t.Fatalf("expected source %q to be excluded from AI enhancement", source)
		}
	}

	allowed := []string{
		model.LyricSourceQQ,
		model.LyricSourceKugou,
		model.LyricSourceNetease,
	}
	for _, source := range allowed {
		if !aiEnhancementAllowedSource(source) {
			t.Fatalf("expected source %q to allow AI enhancement", source)
		}
	}
}
