//go:build windows && cgo

package smtc

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/monitor"

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
	m, err := monitor.New(nil)
	if err != nil {
		return err
	}

	p.state.SetService("smtc", "running", "native Windows SMTC monitor active")
	p.state.AddLog("info", "smtc", "native Windows SMTC monitor started")

	p.publishSessions(m)
	p.publishCurrent(m)

	go func() {
		defer func() {
			_ = m.Close()
			p.state.SetService("smtc", "stopped", "native Windows SMTC monitor stopped")
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-m.Events():
				if !ok {
					return
				}
				p.handleEvent(m, evt)
			case <-ticker.C:
				p.publishCurrent(m)
			}
		}
	}()

	return nil
}

func (p *NativeProvider) handleEvent(m *monitor.Monitor, evt monitor.ManagerEvent) {
	switch evt.Type {
	case monitor.ManagerEventSessionsChanged:
		p.publishSessions(m)
		p.publishCurrent(m)
	case monitor.ManagerEventCurrentSessionChanged:
		p.publishSessions(m)
		p.publishCurrent(m)
	case monitor.ManagerEventSessionPlaybackChanged,
		monitor.ManagerEventSessionTimelineChanged,
		monitor.ManagerEventSessionMediaChanged:
		p.publishCurrent(m)
	}
}

func (p *NativeProvider) publishSessions(m *monitor.Monitor) {
	sessions := m.Sessions()
	currentID := ""
	if selected := chooseSession(sessions, currentSessionID(m), p.state.Config().Session); selected != nil {
		currentID = selected.SessionID
	}

	result := make([]model.Session, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, model.Session{
			ID:        session.SessionID,
			Name:      sessionName(session),
			AppID:     session.SourceAppUserModelID,
			Active:    session.SessionID == currentID,
			Available: true,
			UpdatedAt: model.Now(),
		})
	}
	p.state.SetSessions(result)
}

func (p *NativeProvider) publishCurrent(m *monitor.Monitor) {
	sessions := m.Sessions()
	current := chooseSession(sessions, currentSessionID(m), p.state.Config().Session)
	if current == nil {
		p.state.SetPlayback(model.Playback{
			State:      "stopped",
			UpdatedAt:  model.Now(),
			Volume:     -1,
			CanControl: false,
		})
		return
	}

	p.state.SetTrack(trackFromSession(*current))
	p.state.SetPlayback(playbackFromSession(*current))
}

func currentSessionID(m *monitor.Monitor) string {
	if current := m.CurrentSession(); current != nil {
		return current.SessionID
	}
	return ""
}

func trackFromSession(session smtcsuite.SessionInfo) model.Track {
	title := session.MediaInfo.Title
	if title == "" {
		title = session.SourceAppUserModelID
	}

	artist := session.MediaInfo.Artist
	if artist == "" {
		artist = session.MediaInfo.AlbumArtist
	}

	duration := session.TimelineInfo.EndTime.Milliseconds()
	if duration < 0 {
		duration = 0
	}

	return model.Track{
		ID:        session.SessionID,
		Title:     title,
		Artist:    artist,
		Album:     session.MediaInfo.AlbumTitle,
		SourceApp: session.SourceAppUserModelID,
		Artwork:   thumbnailDataURI(session.MediaInfo.ThumbnailData),
		Duration:  duration,
	}
}

func playbackFromSession(session smtcsuite.SessionInfo) model.Playback {
	position := session.TimelineInfo.Position.Milliseconds()
	if position < 0 {
		position = 0
	}

	return model.Playback{
		State:      playbackState(session.PlaybackStatus),
		Position:   position,
		Volume:     -1,
		UpdatedAt:  model.Now(),
		CanControl: hasControls(session.PlaybackControls),
	}
}

func playbackState(status smtcsuite.PlaybackStatus) string {
	switch status {
	case smtcsuite.PlaybackStatusPlaying:
		return "playing"
	case smtcsuite.PlaybackStatusPaused:
		return "paused"
	case smtcsuite.PlaybackStatusStopped, smtcsuite.PlaybackStatusClosed:
		return "stopped"
	case smtcsuite.PlaybackStatusChanging:
		return "changing"
	default:
		return "opened"
	}
}

func hasControls(controls smtcsuite.PlaybackControls) bool {
	return controls.Play || controls.Pause || controls.PlayPauseToggle || controls.Next || controls.Previous || controls.PlaybackPosition
}

func sessionName(session smtcsuite.SessionInfo) string {
	parts := []string{}
	if session.MediaInfo.Title != "" {
		parts = append(parts, session.MediaInfo.Title)
	}
	if session.MediaInfo.Artist != "" {
		parts = append(parts, session.MediaInfo.Artist)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " - ")
	}
	return session.SourceAppUserModelID
}

func thumbnailDataURI(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	mime := http.DetectContentType(data)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
