package smtc

import (
	"testing"

	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"

	"lyricsync/pkg/model"
)

func TestChooseSessionPrefersManualSelection(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "spotify", SourceAppUserModelID: "Spotify.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
		{SessionID: "foobar", SourceAppUserModelID: "foobar2000.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
	}
	selected := chooseSession(sessions, "spotify", model.SessionConfig{
		AutoSelect: false,
		Preferred:  []string{"foobar"},
	})
	if selected == nil || selected.SessionID != "foobar" {
		t.Fatalf("expected foobar, got %#v", selected)
	}
}

func TestChooseSessionAppliesWhitelistAndBlacklist(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "browser", SourceAppUserModelID: "Browser.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
		{SessionID: "music", SourceAppUserModelID: "Music.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
	}
	selected := chooseSession(sessions, "browser", model.SessionConfig{
		AutoSelect: true,
		Whitelist:  []string{"music"},
		Blacklist:  []string{"browser"},
	})
	if selected == nil || selected.SessionID != "music" {
		t.Fatalf("expected whitelisted music session, got %#v", selected)
	}
}

func TestChooseSessionAutoSelectsPlayingSession(t *testing.T) {
	sessions := []smtcsuite.SessionInfo{
		{SessionID: "paused", SourceAppUserModelID: "Paused.exe", PlaybackStatus: smtcsuite.PlaybackStatusPaused},
		{SessionID: "playing", SourceAppUserModelID: "Playing.exe", PlaybackStatus: smtcsuite.PlaybackStatusPlaying},
	}
	selected := chooseSession(sessions, "paused", model.SessionConfig{AutoSelect: true})
	if selected == nil || selected.SessionID != "playing" {
		t.Fatalf("expected playing session, got %#v", selected)
	}
}
