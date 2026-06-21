package lyric

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lyricsync/internal/core"
	"lyricsync/pkg/model"
)

type Manager struct {
	state   *core.State
	cache   *Cache
	local   *LocalProvider
	online  *OnlineProvider
	lastKey string
	mu      sync.Mutex
}

func NewManager(state *core.State) *Manager {
	return &Manager{state: state}
}

func (m *Manager) Start(ctx context.Context) {
	cfg := m.state.Config().Lyrics
	cache, err := NewCache(cfg.CacheDirectory)
	if err != nil {
		m.state.AddLog("warn", "lyrics", "cache disabled: "+err.Error())
	} else {
		m.cache = cache
	}
	m.local = NewLocalProvider(cfg.LocalScanPaths)
	m.online = NewOnlineProvider(cfg.Sources)

	m.state.SetService("lyrics", "running", "local lyric pipeline ready")
	if m.cache != nil {
		m.state.AddLog("info", "lyrics", "lyric cache directory: "+m.cache.Dir())
	}
	m.state.SetLyrics(DemoDocument())

	events, cancel := m.state.Subscribe(32)

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				m.state.SetService("lyrics", "stopped", "lyric manager stopped")
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type != "song_changed" {
					continue
				}
				track, ok := event.Payload.(model.Track)
				if !ok {
					continue
				}
				m.handleTrackChanged(ctx, track)
			}
		}
	}()
}

func (m *Manager) handleTrackChanged(ctx context.Context, track model.Track) {
	if !m.state.Config().Lyrics.AutoSearch {
		return
	}
	if !isSearchableTrack(track) {
		return
	}
	if track.SourceApp == "SMTC Simulator" {
		doc := DemoDocument()
		doc.TrackID = track.ID
		m.state.SetLyrics(doc)
		return
	}

	key := CacheKey(track)
	m.mu.Lock()
	if key == m.lastKey {
		m.mu.Unlock()
		return
	}
	m.lastKey = key
	m.mu.Unlock()

	go m.searchForTrack(ctx, track, key)
}

func (m *Manager) searchForTrack(parent context.Context, track model.Track, key string) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	m.state.SetService("lyrics", "searching", "searching lyrics for "+displayTrack(track))
	m.state.AddLog("info", "lyrics", "searching lyrics for "+displayTrack(track))

	doc, from, err := m.resolve(ctx, track, key)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "online lyric search failed")
		m.state.AddLog("warn", "lyrics", err.Error())
		return
	}

	m.state.SetLyrics(doc)
	m.state.SetService("lyrics", "running", "lyrics loaded from "+from)
	m.state.AddLog("info", "lyrics", "lyrics loaded from "+from+" for "+displayTrack(track))
}

func (m *Manager) Search(ctx context.Context, title, artist, album string) (model.LyricDocument, error) {
	track := model.Track{
		ID:     "manual-" + CacheKey(model.Track{Title: title, Artist: artist, Album: album}),
		Title:  title,
		Artist: artist,
		Album:  album,
	}
	if !isSearchableTrack(track) {
		return model.LyricDocument{}, fmt.Errorf("lyric: title is required")
	}

	key := CacheKey(track)
	searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	m.state.SetService("lyrics", "searching", "manual lyric search for "+displayTrack(track))
	doc, from, err := m.resolve(searchCtx, track, key)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "manual lyric search failed")
		return model.LyricDocument{}, err
	}
	m.state.SetLyrics(doc)
	m.state.SetService("lyrics", "running", "lyrics loaded from "+from)
	m.state.AddLog("info", "lyrics", "manual lyrics loaded from "+from+" for "+displayTrack(track))
	if m.online != nil {
		if candidates, err := m.online.SearchCandidates(searchCtx, track, 8); err == nil {
			m.state.SetLyricCandidates(candidates)
		}
	}
	return doc, nil
}

func (m *Manager) SearchCandidates(ctx context.Context, title, artist, album string) ([]model.LyricSearchCandidate, error) {
	track := model.Track{
		ID:     "manual-" + CacheKey(model.Track{Title: title, Artist: artist, Album: album}),
		Title:  title,
		Artist: artist,
		Album:  album,
	}
	if !isSearchableTrack(track) {
		return nil, fmt.Errorf("lyric: title is required")
	}
	if m.online == nil {
		return nil, fmt.Errorf("lyric: online provider is not ready")
	}

	searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m.state.SetService("lyrics", "searching", "searching lyric candidates for "+displayTrack(track))
	candidates, err := m.online.SearchCandidates(searchCtx, track, 8)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric candidate search failed")
		return nil, err
	}
	m.state.SetLyricCandidates(candidates)
	m.state.SetService("lyrics", "running", fmt.Sprintf("%d lyric candidates found", len(candidates)))
	m.state.AddLog("info", "lyrics", fmt.Sprintf("%d lyric candidates found for %s", len(candidates), displayTrack(track)))
	return candidates, nil
}

func (m *Manager) ApplyCandidate(ctx context.Context, candidate model.LyricSearchCandidate) (model.LyricDocument, error) {
	if m.online == nil {
		return model.LyricDocument{}, fmt.Errorf("lyric: online provider is not ready")
	}
	track := m.state.Snapshot().Track
	if track.ID == "" || track.ID == "idle" {
		track.ID = "manual-" + model.Now()
		track.Title = candidate.Title
		track.Artist = candidate.Artist
		track.Album = candidate.Album
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	m.state.SetService("lyrics", "searching", "loading lyrics from "+candidate.Source)
	doc, err := m.online.FetchCandidate(fetchCtx, track, candidate)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric candidate load failed")
		return model.LyricDocument{}, err
	}
	if m.cache != nil && m.state.Config().Lyrics.CacheEnabled && isSearchableTrack(track) {
		if err := m.cache.Save(CacheKey(track), doc); err != nil {
			m.state.AddLog("warn", "lyrics", "cache save failed: "+err.Error())
		}
	}
	m.state.SetLyrics(doc)
	m.state.SetService("lyrics", "running", "lyrics loaded from "+candidate.Source)
	m.state.AddLog("info", "lyrics", fmt.Sprintf("lyrics selected from %s: %s", candidate.Source, candidate.Title))
	return doc, nil
}

func (m *Manager) ImportFile(ctx context.Context, path string) (model.LyricDocument, error) {
	track := m.state.Snapshot().Track
	if track.ID == "" || track.ID == "idle" {
		track.ID = "local-import-" + model.Now()
	}
	doc, err := ImportFile(track, path)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric import failed")
		return model.LyricDocument{}, err
	}
	if m.cache != nil && m.state.Config().Lyrics.CacheEnabled && isSearchableTrack(track) {
		if err := m.cache.Save(CacheKey(track), doc); err != nil {
			m.state.AddLog("warn", "lyrics", "cache save failed: "+err.Error())
		}
	}
	m.state.SetLyrics(doc)
	m.state.SetService("lyrics", "running", "lyrics imported")
	m.state.AddLog("info", "lyrics", "lyrics imported from local file")
	return doc, nil
}

func (m *Manager) OffsetCurrent(offsetMs int64) (model.LyricDocument, error) {
	doc := m.state.Snapshot().Lyrics
	if len(doc.Lines) == 0 && doc.TTML == "" {
		return model.LyricDocument{}, fmt.Errorf("lyric: no current lyrics to offset")
	}
	next, err := OffsetDocument(doc, offsetMs)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric offset failed")
		return model.LyricDocument{}, err
	}
	m.state.SetLyrics(next)
	m.state.SetService("lyrics", "running", fmt.Sprintf("lyrics shifted by %d ms", offsetMs))
	m.state.AddLog("info", "lyrics", fmt.Sprintf("current lyrics shifted by %d ms", offsetMs))
	return next, nil
}

func (m *Manager) AlignCurrent(lineIndex int, positionMs int64) (model.LyricDocument, error) {
	doc := m.state.Snapshot().Lyrics
	if len(doc.Lines) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no current lyrics to align")
	}
	next, offsetMs, err := AlignDocument(doc, lineIndex, positionMs)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric alignment failed")
		return model.LyricDocument{}, err
	}
	m.state.SetLyrics(next)
	m.state.SetService("lyrics", "running", fmt.Sprintf("line %d aligned by %d ms", lineIndex+1, offsetMs))
	m.state.AddLog("info", "lyrics", fmt.Sprintf("line %d aligned to %d ms with offset %d ms", lineIndex+1, positionMs, offsetMs))
	return next, nil
}

func (m *Manager) SuggestAlignment(lineIndex int) (model.LyricCalibrationSuggestion, error) {
	snapshot := m.state.Snapshot()
	if len(snapshot.Lyrics.Lines) == 0 {
		return model.LyricCalibrationSuggestion{}, fmt.Errorf("lyric: no current lyrics to calibrate")
	}
	suggestion, err := SuggestAlignment(
		snapshot.Lyrics,
		m.state.AudioHistory(),
		lineIndex,
		snapshot.Playback.Position,
		snapshot.Track.ID,
	)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric calibration unavailable")
		return model.LyricCalibrationSuggestion{}, err
	}
	m.state.SetService("lyrics", "running", fmt.Sprintf("line %d calibration suggested", suggestion.LineIndex+1))
	m.state.AddLog("info", "lyrics", fmt.Sprintf("line %d calibration suggests offset %d ms", suggestion.LineIndex+1, suggestion.OffsetMs))
	return suggestion, nil
}

func (m *Manager) AutoAlignCurrent(lineIndex int) (model.LyricCalibrationResult, error) {
	suggestion, err := m.SuggestAlignment(lineIndex)
	if err != nil {
		return model.LyricCalibrationResult{}, err
	}
	doc := m.state.Snapshot().Lyrics
	next, offsetMs, err := AlignDocument(doc, suggestion.LineIndex, suggestion.SuggestedPositionMs)
	if err != nil {
		m.state.SetService("lyrics", "degraded", "lyric calibration failed")
		return model.LyricCalibrationResult{}, err
	}
	suggestion.OffsetMs = offsetMs
	m.state.SetLyrics(next)
	m.state.SetService("lyrics", "running", fmt.Sprintf("line %d auto calibrated by %d ms", suggestion.LineIndex+1, offsetMs))
	m.state.AddLog("info", "lyrics", fmt.Sprintf("line %d auto calibrated to %d ms with offset %d ms", suggestion.LineIndex+1, suggestion.SuggestedPositionMs, offsetMs))
	return model.LyricCalibrationResult{
		Suggestion:    suggestion,
		UpdatedLyrics: next,
	}, nil
}

func (m *Manager) resolve(ctx context.Context, track model.Track, key string) (model.LyricDocument, string, error) {
	if m.cache != nil && m.state.Config().Lyrics.CacheEnabled {
		if doc, ok, err := m.cache.Load(key); err == nil && ok {
			doc.TrackID = track.ID
			return doc, "cache", nil
		} else if err != nil {
			m.state.AddLog("warn", "lyrics", "cache read failed: "+err.Error())
		}
	}

	if m.local != nil {
		if doc, err := m.local.Search(ctx, track); err == nil {
			if m.cache != nil && m.state.Config().Lyrics.CacheEnabled {
				if err := m.cache.Save(key, doc); err != nil {
					m.state.AddLog("warn", "lyrics", "cache save failed: "+err.Error())
				}
			}
			return doc, "local file", nil
		}
	}

	doc, err := m.online.Search(ctx, track)
	if err != nil {
		return model.LyricDocument{}, "", err
	}

	if m.cache != nil && m.state.Config().Lyrics.CacheEnabled {
		if err := m.cache.Save(key, doc); err != nil {
			m.state.AddLog("warn", "lyrics", "cache save failed: "+err.Error())
		}
	}
	return doc, doc.Source, nil
}

func isSearchableTrack(track model.Track) bool {
	if track.Title == "" || track.ID == "idle" {
		return false
	}
	return queryForTrack(track) != ""
}

func displayTrack(track model.Track) string {
	if track.Artist == "" {
		return track.Title
	}
	return track.Title + " - " + track.Artist
}
