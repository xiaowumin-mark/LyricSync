package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	suiteaudio "github.com/xiaowumin-mark/smtc-suite-go/pkg/audio"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/audio/loopback"
	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/control"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/monitor"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

type Service struct {
	store    *state.Store
	spectrum *spectrumProcessor
}

func New(store *state.Store) *Service {
	return &Service{store: store, spectrum: newSpectrumProcessor()}
}

func (s *Service) Start(ctx context.Context) {
	if err := s.startSMTC(ctx); err != nil {
		s.store.AddLog("SMTC unavailable: " + err.Error())
		s.startSMTCSimulator(ctx)
	}
	if err := s.startAudio(ctx); err != nil {
		s.store.AddLog("Audio capture unavailable: " + err.Error())
		s.startAudioSimulator(ctx)
	}
}

func (s *Service) Control(command string, positionMs int64) error {
	sessionID := s.store.Snapshot().Track.ID
	if sessionID == "idle" || strings.HasPrefix(sessionID, "demo-") {
		sessionID = ""
	}
	ctrl, err := control.New(sessionID)
	if err != nil {
		s.store.AddLog("Control failed: " + err.Error())
		return err
	}
	defer ctrl.Close()

	switch command {
	case "play":
		err = ctrl.Play()
	case "pause":
		err = ctrl.Pause()
	case "next":
		err = ctrl.Next()
	case "previous":
		err = ctrl.Previous()
	case "seek":
		if positionMs < 0 {
			positionMs = 0
		}
		err = ctrl.Seek(time.Duration(positionMs) * time.Millisecond)
	default:
		err = fmt.Errorf("unsupported control command %q", command)
	}
	if err != nil {
		s.store.AddLog("Control " + command + " failed: " + err.Error())
		return err
	}
	s.store.AddLog("Control " + command + " sent")
	return nil
}

func (s *Service) SetVolume(level float64) error {
	level = math.Max(0, math.Min(1, level))
	playback := s.store.Snapshot().Playback
	playback.Volume = level
	playback.UpdatedAt = model.Now()
	s.store.SetPlayback(playback)
	s.store.AddLog(fmt.Sprintf("Volume command received: %.0f%%", level*100))
	return nil
}

func (s *Service) startSMTC(ctx context.Context) error {
	m, err := monitor.New(nil)
	if err != nil {
		return err
	}
	s.store.AddLog("Native SMTC monitor started")
	s.publishSessions(m)
	s.publishCurrent(m)

	go func() {
		defer func() {
			_ = m.Close()
			s.store.AddLog("Native SMTC monitor stopped")
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
				switch evt.Type {
				case monitor.ManagerEventSessionsChanged,
					monitor.ManagerEventCurrentSessionChanged:
					s.publishSessions(m)
					s.publishCurrent(m)
				case monitor.ManagerEventSessionPlaybackChanged,
					monitor.ManagerEventSessionTimelineChanged,
					monitor.ManagerEventSessionMediaChanged:
					s.publishSessions(m)
					s.publishCurrent(m)
				}
			case <-ticker.C:
				s.publishSessions(m)
				s.publishCurrent(m)
			}
		}
	}()
	return nil
}

func (s *Service) publishSessions(m *monitor.Monitor) {
	sessions := m.Sessions()
	currentID := ""
	if current := chooseSession(sessions, s.preferredCurrentID(m), s.store.Config().Media); current != nil {
		currentID = current.SessionID
	}

	result := make([]model.Session, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, sessionFromInfo(session, session.SessionID == currentID))
	}
	s.store.SetSessions(result)
}

func (s *Service) publishCurrent(m *monitor.Monitor) {
	cfg := s.store.Config().Media
	current := chooseSession(m.Sessions(), s.preferredCurrentID(m), cfg)
	if current == nil {
		title := "Waiting for playback"
		if cfg.SelectedSessionID != "" {
			title = "Waiting for selected SMTC session"
		}
		s.store.SetTrack(idleTrack(title))
		s.store.SetPlayback(model.Playback{State: "stopped", Volume: -1, UpdatedAt: model.Now()})
		return
	}
	s.store.SetTrack(trackFromSession(*current))
	s.store.SetPlayback(playbackFromSession(*current))
}

func (s *Service) preferredCurrentID(m *monitor.Monitor) string {
	currentID := s.store.Snapshot().Track.ID
	if currentID == "" || currentID == "idle" || strings.HasPrefix(currentID, "demo-") {
		return currentSessionID(m)
	}
	return currentID
}

func idleTrack(title string) model.Track {
	return model.Track{
		ID:        "idle",
		Title:     title,
		Artist:    "LyricSync",
		SourceApp: "System",
	}
}

func sessionFromInfo(session smtcsuite.SessionInfo, active bool) model.Session {
	position := session.TimelineInfo.Position.Milliseconds()
	if position < 0 {
		position = 0
	}
	duration := session.TimelineInfo.EndTime.Milliseconds()
	if duration < 0 {
		duration = 0
	}
	return model.Session{
		ID:        session.SessionID,
		Name:      sessionName(session),
		AppID:     session.SourceAppUserModelID,
		Title:     session.MediaInfo.Title,
		Artist:    firstNonEmpty(session.MediaInfo.Artist, session.MediaInfo.AlbumArtist),
		Album:     session.MediaInfo.AlbumTitle,
		State:     playbackState(session.PlaybackStatus),
		Position:  position,
		Duration:  duration,
		Active:    active,
		UpdatedAt: model.Now(),
	}
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
	track := model.Track{
		ID:        session.SessionID,
		Title:     title,
		Artist:    artist,
		Album:     session.MediaInfo.AlbumTitle,
		SourceApp: session.SourceAppUserModelID,
		Duration:  duration,
	}
	if len(session.MediaInfo.ThumbnailData) > 0 {
		track.CoverData = append([]byte(nil), session.MediaInfo.ThumbnailData...)
		track.CoverHash = strings.TrimSpace(session.MediaInfo.ThumbnailHash)
		if track.CoverHash == "" {
			sum := sha256.Sum256(track.CoverData)
			track.CoverHash = hex.EncodeToString(sum[:])
		}
		track.CoverMimeType = detectCoverMimeType(track.CoverData)
	}
	return track
}

func detectCoverMimeType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	contentType := http.DetectContentType(data)
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
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
		CanControl: hasControls(session.PlaybackControls),
		UpdatedAt:  model.Now(),
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func chooseSession(sessions []smtcsuite.SessionInfo, currentID string, cfg model.MediaConfig) *smtcsuite.SessionInfo {
	allowed := make([]smtcsuite.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		if sessionAllowed(session, cfg) {
			allowed = append(allowed, session)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	if cfg.SelectedSessionID != "" {
		return findSessionByID(allowed, cfg.SelectedSessionID)
	}
	if selected := matchSessionList(allowed, cfg.Preferred); selected != nil {
		return selected
	}
	if !cfg.AutoSelect {
		if selected := findSessionByID(allowed, currentID); selected != nil {
			return selected
		}
		return &allowed[0]
	}
	if selected := findSessionByID(allowed, currentID); selected != nil && selected.PlaybackStatus == smtcsuite.PlaybackStatusPlaying {
		return selected
	}
	for index := range allowed {
		if allowed[index].PlaybackStatus == smtcsuite.PlaybackStatusPlaying {
			return &allowed[index]
		}
	}
	if selected := findSessionByID(allowed, currentID); selected != nil {
		return selected
	}
	sort.SliceStable(allowed, func(i, j int) bool { return sessionName(allowed[i]) < sessionName(allowed[j]) })
	return &allowed[0]
}

func sessionAllowed(session smtcsuite.SessionInfo, cfg model.MediaConfig) bool {
	if len(cfg.Whitelist) > 0 && !sessionMatchesAny(session, cfg.Whitelist) {
		return false
	}
	return !sessionMatchesAny(session, cfg.Blacklist)
}

func matchSessionList(sessions []smtcsuite.SessionInfo, patterns []string) *smtcsuite.SessionInfo {
	for _, pattern := range patterns {
		for index := range sessions {
			if sessionMatches(sessions[index], pattern) {
				return &sessions[index]
			}
		}
	}
	return nil
}

func findSessionByID(sessions []smtcsuite.SessionInfo, id string) *smtcsuite.SessionInfo {
	if id == "" {
		return nil
	}
	for index := range sessions {
		if sessions[index].SessionID == id || sessions[index].SourceAppUserModelID == id {
			return &sessions[index]
		}
	}
	return nil
}

func sessionMatchesAny(session smtcsuite.SessionInfo, patterns []string) bool {
	for _, pattern := range patterns {
		if sessionMatches(session, pattern) {
			return true
		}
	}
	return false
}

func sessionMatches(session smtcsuite.SessionInfo, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	fields := []string{
		session.SessionID,
		session.SourceAppUserModelID,
		session.MediaInfo.Title,
		session.MediaInfo.Artist,
		session.MediaInfo.AlbumArtist,
		session.MediaInfo.AlbumTitle,
		sessionName(session),
	}
	for _, field := range fields {
		value := strings.ToLower(strings.TrimSpace(field))
		if value == pattern || strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func (s *Service) startAudio(ctx context.Context) error {
	capturer, err := loopback.New(&loopback.Config{EventBuffer: 128})
	if err != nil {
		return err
	}
	format := capturer.Format()
	if !suiteaudio.CanConvertToFloat32(format) {
		_ = capturer.Close()
		return fmt.Errorf("unsupported loopback format %s/%d-bit", format.SampleFormat, format.BitsPerSample)
	}
	if err := capturer.Start(); err != nil {
		_ = capturer.Close()
		return err
	}
	s.store.AddLog("Native WASAPI loopback started")

	go func() {
		defer func() {
			_ = capturer.Close()
			s.store.AddLog("Native WASAPI loopback stopped")
		}()
		var seq uint64
		var sampleBuf []float32
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-capturer.Errors():
				if ok && err != nil {
					s.store.AddLog("Audio error: " + err.Error())
				}
			case frame, ok := <-capturer.Frames():
				if !ok {
					return
				}
				seq++
				samples, err := suiteaudio.ConvertToFloat32(frame.Format, frame.Data, sampleBuf)
				if err != nil {
					s.store.AddLog("Audio convert failed: " + err.Error())
					continue
				}
				sampleBuf = samples[:0]
				s.store.SetAudio(s.audioFrame(seq, frame, samples))
			}
		}
	}()
	return nil
}

func (s *Service) audioFrame(seq uint64, frame loopback.Frame, samples []float32) model.AudioFrame {
	snapshot := s.store.Snapshot()
	rms, peak := level(samples)
	duration := 0
	if frame.Format.SampleRate > 0 {
		duration = int(float64(frame.Frames) / float64(frame.Format.SampleRate) * 1000)
	}
	return model.AudioFrame{
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
		Spectrum:     s.spectrum.Process(samples, 32, frame.Format.Channels, frame.Format.SampleRate),
		PCM:          append([]byte(nil), frame.Data...),
		ProviderMode: "wasapi-loopback",
	}
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
	return visualEnergy(math.Sqrt(sum / float64(len(samples)))), visualEnergy(peak)
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

func (s *Service) startSMTCSimulator(ctx context.Context) {
	track := model.Track{
		ID:        "demo-track",
		Title:     "LyricSync Demo",
		Artist:    "Local Session",
		Album:     "Development",
		SourceApp: "Simulator",
		Duration:  184000,
	}
	s.store.SetTrack(track)
	s.store.SetSessions([]model.Session{{
		ID:        "demo-session",
		Name:      "SMTC Simulator",
		AppID:     "lyricsync",
		Title:     track.Title,
		Artist:    track.Artist,
		Album:     track.Album,
		State:     "playing",
		Duration:  track.Duration,
		Active:    true,
		UpdatedAt: model.Now(),
	}})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				position := now.Sub(start).Milliseconds() % track.Duration
				s.store.SetPlayback(model.Playback{
					State:      "playing",
					Position:   position,
					Volume:     1,
					CanControl: false,
					UpdatedAt:  model.FormatTime(now),
				})
				s.store.SetSessions([]model.Session{{
					ID:        "demo-session",
					Name:      "SMTC Simulator",
					AppID:     "lyricsync",
					Title:     track.Title,
					Artist:    track.Artist,
					Album:     track.Album,
					State:     "playing",
					Position:  position,
					Duration:  track.Duration,
					Active:    true,
					UpdatedAt: model.FormatTime(now),
				}})
			}
		}
	}()
}

func (s *Service) startAudioSimulator(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		var seq uint64
		start := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				seq++
				elapsed := now.Sub(start).Seconds()
				s.store.SetAudio(model.AudioFrame{
					Sequence:     seq,
					Timestamp:    model.FormatTime(now),
					TrackID:      s.store.Snapshot().Track.ID,
					PositionMs:   s.store.Snapshot().Playback.Position,
					SampleRate:   48000,
					Channels:     2,
					Format:       "s16le",
					DurationMs:   50,
					RMS:          0.45 + 0.2*math.Sin(elapsed*2.4),
					Peak:         0.75 + 0.15*math.Sin(elapsed*4.8),
					Spectrum:     simulatorSpectrum(elapsed, 32),
					ProviderMode: "simulator",
				})
			}
		}
	}()
}

func simulatorSpectrum(t float64, bins int) []float64 {
	values := make([]float64, bins)
	for i := 0; i < bins; i++ {
		band := float64(i + 1)
		value := 0.48 + math.Sin(t*2.4+band*0.33)*0.3 + math.Sin(t*band*0.08)*0.16
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
