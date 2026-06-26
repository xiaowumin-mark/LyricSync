package state

import (
	"bytes"
	"sync"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const maxLogs = 50

type Store struct {
	mu          sync.RWMutex
	config      model.Config
	track       model.Track
	playback    model.Playback
	audio       model.AudioFrame
	sessions    []model.Session
	amll        model.AMLLConnection
	logs        []string
	subscribers map[chan model.Event]struct{}
}

func New(cfg model.Config) *Store {
	now := model.Now()
	return &Store{
		config: cfg,
		track: model.Track{
			ID:        "idle",
			Title:     "Waiting for playback",
			Artist:    "LyricSync",
			SourceApp: "System",
		},
		playback: model.Playback{
			State:     "stopped",
			Volume:    1,
			UpdatedAt: now,
		},
		amll: model.AMLLConnection{
			URL:     cfg.AMLL.URL,
			Status:  "disconnected",
			Message: "not connected",
		},
		logs:        []string{now + " LyricSync started"},
		subscribers: map[chan model.Event]struct{}{},
	}
}

func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return model.Snapshot{
		Version:   model.Version,
		Config:    s.config,
		Track:     cloneTrack(s.track),
		Playback:  s.playback,
		Audio:     s.audio,
		Sessions:  append([]model.Session(nil), s.sessions...),
		AMLL:      s.amll,
		Logs:      append([]string(nil), s.logs...),
		UpdatedAt: model.Now(),
	}
}

func (s *Store) Config() model.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *Store) SetConfig(cfg model.Config) {
	s.mu.Lock()
	s.config = cfg
	s.amll.URL = cfg.AMLL.URL
	s.publishLocked("config_changed", cfg)
	s.mu.Unlock()
}

func (s *Store) SetTrack(track model.Track) {
	s.mu.Lock()
	track = cloneTrack(track)
	if trackEqual(s.track, track) {
		s.mu.Unlock()
		return
	}
	s.track = track
	s.publishLocked("track_changed", cloneTrack(track))
	s.mu.Unlock()
}

func (s *Store) SetPlayback(playback model.Playback) {
	s.mu.Lock()
	if playback.UpdatedAt == "" {
		playback.UpdatedAt = model.Now()
	}
	if playback.Volume < 0 {
		playback.Volume = s.playback.Volume
	}
	s.playback = playback
	s.publishLocked("playback_changed", playback)
	s.mu.Unlock()
}

func (s *Store) SetAudio(frame model.AudioFrame) {
	s.mu.Lock()
	s.audio = frame
	s.publishLocked("audio_frame", frame)
	s.mu.Unlock()
}

func (s *Store) SetSessions(sessions []model.Session) {
	s.mu.Lock()
	s.sessions = append([]model.Session(nil), sessions...)
	s.publishLocked("sessions_changed", s.sessions)
	s.mu.Unlock()
}

func (s *Store) SetAMLL(conn model.AMLLConnection) {
	s.mu.Lock()
	if conn.URL == "" {
		conn.URL = s.config.AMLL.URL
	}
	s.amll = conn
	s.publishLocked("amll_changed", conn)
	s.mu.Unlock()
}

func (s *Store) AMLL() model.AMLLConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.amll
}

func (s *Store) AddLog(message string) {
	s.mu.Lock()
	line := time.Now().Format("15:04:05") + " " + message
	s.logs = append(s.logs, line)
	if len(s.logs) > maxLogs {
		s.logs = append([]string(nil), s.logs[len(s.logs)-maxLogs:]...)
	}
	s.publishLocked("log", line)
	s.mu.Unlock()
}

func (s *Store) Subscribe(buffer int) (<-chan model.Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan model.Event, buffer)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
}

func (s *Store) publishLocked(eventType string, payload any) {
	event := model.Event{Type: eventType, Payload: payload, Time: model.Now()}
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func cloneTrack(track model.Track) model.Track {
	track.CoverData = append([]byte(nil), track.CoverData...)
	return track
}

func trackEqual(a, b model.Track) bool {
	return a.ID == b.ID &&
		a.Title == b.Title &&
		a.Artist == b.Artist &&
		a.Album == b.Album &&
		a.SourceApp == b.SourceApp &&
		a.Duration == b.Duration &&
		a.CoverMimeType == b.CoverMimeType &&
		a.CoverHash == b.CoverHash &&
		bytes.Equal(a.CoverData, b.CoverData)
}
