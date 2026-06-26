package media

import (
	"bytes"
	"math"
	"testing"
	"time"

	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestChooseSessionAutoSelectsPlaying(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "paused", SourceAppUserModelID: "Paused.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
		{SessionID: "playing", SourceAppUserModelID: "Playing.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
	}
	selected := chooseSession(sessions, "paused", model.MediaConfig{AutoSelect: true})
	if selected == nil || selected.SessionID != "playing" {
		t.Fatalf("expected playing session, got %#v", selected)
	}
}

func TestChooseSessionManualSelectionWins(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "browser", SourceAppUserModelID: "Browser.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
		{SessionID: "music", SourceAppUserModelID: "Music.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
	}
	selected := chooseSession(sessions, "browser", model.MediaConfig{
		AutoSelect:        true,
		SelectedSessionID: "music",
	})
	if selected == nil || selected.SessionID != "music" {
		t.Fatalf("expected manual music session, got %#v", selected)
	}
}

func TestChooseSessionManualSelectionMissingDoesNotFallback(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "browser", SourceAppUserModelID: "Browser.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
	}
	selected := chooseSession(sessions, "browser", model.MediaConfig{
		AutoSelect:        true,
		SelectedSessionID: "missing",
	})
	if selected != nil {
		t.Fatalf("expected no session when manual selection is missing, got %#v", selected)
	}
}

func TestChooseSessionAutoSelectKeepsCurrentPlaying(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "first", SourceAppUserModelID: "First.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
		{SessionID: "current", SourceAppUserModelID: "Current.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
	}
	selected := chooseSession(sessions, "current", model.MediaConfig{AutoSelect: true})
	if selected == nil || selected.SessionID != "current" {
		t.Fatalf("expected current playing session to stay selected, got %#v", selected)
	}
}

func TestChooseSessionAppliesWhitelistAndBlacklist(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "browser", SourceAppUserModelID: "Browser.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
		{SessionID: "music", SourceAppUserModelID: "Music.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
	}
	selected := chooseSession(sessions, "browser", model.MediaConfig{
		AutoSelect: true,
		Whitelist:  []string{"music"},
		Blacklist:  []string{"browser"},
	})
	if selected == nil || selected.SessionID != "music" {
		t.Fatalf("expected music session, got %#v", selected)
	}
}

func TestTrackFromSessionCopiesThumbnail(t *testing.T) {
	thumbnail := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	track := trackFromSession(smtcsuite.SessionInfo{
		SessionID:            "session-1",
		SourceAppUserModelID: "Music.exe",
		MediaInfo: smtcsuite.MediaInfo{
			Title:         "Song",
			Artist:        "Artist",
			AlbumTitle:    "Album",
			ThumbnailData: thumbnail,
			ThumbnailHash: "cover-hash",
		},
	})
	thumbnail[0] = 0
	if track.CoverHash != "cover-hash" {
		t.Fatalf("expected cover hash to be preserved, got %q", track.CoverHash)
	}
	if track.CoverMimeType != "image/png" {
		t.Fatalf("expected PNG mime type, got %q", track.CoverMimeType)
	}
	if !bytes.Equal(track.CoverData, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		t.Fatalf("expected thumbnail data to be copied, got %#v", track.CoverData)
	}
}

func TestSessionFromInfoIncludesPlaybackMetadata(t *testing.T) {
	session := sessionFromInfo(smtcsuite.SessionInfo{
		SessionID:            "session-1",
		SourceAppUserModelID: "Music.exe",
		MediaInfo: smtcsuite.MediaInfo{
			Title:      "Song",
			Artist:     "Artist",
			AlbumTitle: "Album",
		},
		PlaybackStatus: smtcsuite.PlaybackStatusPlaying,
		TimelineInfo: smtcsuite.TimelineInfo{
			Position: 45 * time.Second,
			EndTime:  3 * time.Minute,
		},
	}, true)
	if session.ID != "session-1" || session.AppID != "Music.exe" {
		t.Fatalf("unexpected identity: %#v", session)
	}
	if session.Title != "Song" || session.Artist != "Artist" || session.Album != "Album" {
		t.Fatalf("unexpected media metadata: %#v", session)
	}
	if session.State != "playing" || session.Position != 45000 || session.Duration != 180000 || !session.Active {
		t.Fatalf("unexpected playback metadata: %#v", session)
	}
}

func TestSpectrumProcessorProducesFiniteBands(t *testing.T) {
	processor := newSpectrumProcessor()
	samples := make([]float32, 2048)
	for frame := 0; frame < len(samples)/2; frame++ {
		value := float32(math.Sin(2 * math.Pi * 1000 * float64(frame) / 48000))
		samples[frame*2] = value
		samples[frame*2+1] = value
	}
	got := processor.Process(samples, 32, 2, 48000)
	if len(got) != 32 {
		t.Fatalf("expected 32 bands, got %d", len(got))
	}
	var maxValue float64
	for _, value := range got {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("expected finite value, got %v", value)
		}
		if value < 0 || value > 1 {
			t.Fatalf("expected normalized value, got %v", value)
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == 0 {
		t.Fatal("expected sine wave to produce non-zero spectrum")
	}
}
