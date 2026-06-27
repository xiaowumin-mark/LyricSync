package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/amll"
	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/lyric"
	"github.com/xiaowumin-mark/LyricSync/internal/media"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/paths"
	"github.com/xiaowumin-mark/LyricSync/internal/song"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

type Runtime struct {
	store       *state.Store
	media       *media.Service
	connector   *amll.Connector
	songs       *song.Repository
	lyrics      *lyric.SearchService
	ctx         context.Context
	cancel      context.CancelFunc
	lyricMu     sync.Mutex
	lyricCancel context.CancelFunc
	lyricSeq    uint64
}

func New(store *state.Store, songs *song.Repository) *Runtime {
	mediaSvc := media.New(store)
	connector := amll.NewConnector(store)
	connector.SetCommandHandlers(mediaSvc.Control, mediaSvc.SetVolume)
	ttmlCacheDir, err := paths.TTMLDBCacheDir()
	if err != nil {
		store.AddLog("TTML DB cache path failed: " + err.Error())
	}
	return &Runtime{
		store:     store,
		media:     mediaSvc,
		connector: connector,
		songs:     songs,
		lyrics:    lyric.NewSearchService(ttmlCacheDir),
	}
}

func (r *Runtime) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	r.ctx = ctx
	r.cancel = cancel
	r.startSongRecorder(ctx)
	r.media.Start(ctx)
	cfg := r.store.Config()
	if cfg.AMLL.AutoConnect {
		if err := r.connector.Connect(ctx, cfg.AMLL.URL, cfg.AMLL.SendAudio); err != nil {
			r.store.AddLog("Auto connect failed: " + err.Error())
		}
	}
	return nil
}

func (r *Runtime) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.cancelLyricSearch()
	r.connector.Disconnect()
}

func (r *Runtime) ConnectAMLL(url string) error {
	cfg := r.store.Config()
	cfg.AMLL.URL = url
	if err := r.connector.Connect(r.context(), cfg.AMLL.URL, cfg.AMLL.SendAudio); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		r.connector.Disconnect()
		return err
	}
	r.store.SetConfig(cfg)
	return nil
}

func (r *Runtime) DisconnectAMLL() error {
	r.connector.Disconnect()
	return nil
}

func (r *Runtime) SendSnapshot() error {
	return r.connector.SendSnapshot()
}

func (r *Runtime) ToggleSendAudio(enabled bool) error {
	cfg := r.store.Config()
	cfg.AMLL.SendAudio = enabled
	if err := r.SaveConfig(cfg); err != nil {
		return err
	}
	r.store.AddLog("Send audio set to " + boolText(enabled))
	current := r.store.AMLL()
	if current.Status == "connected" || current.Status == "connecting" {
		if err := r.connector.Connect(r.context(), cfg.AMLL.URL, cfg.AMLL.SendAudio); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) SelectMediaSession(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	cfg := r.store.Config()
	cfg.Media.SelectedSessionID = sessionID
	cfg.Media.AutoSelect = sessionID == ""
	if err := r.SaveConfig(cfg); err != nil {
		return err
	}
	if sessionID == "" {
		r.store.AddLog("SMTC selection set to auto")
	} else {
		r.store.AddLog("SMTC selection locked to " + sessionID)
	}
	return nil
}

func (r *Runtime) ControlMedia(command string, positionMs int64) error {
	return r.media.Control(command, positionMs)
}

func (r *Runtime) SaveConfig(cfg model.Config) error {
	if err := config.Save(cfg); err != nil {
		return err
	}
	r.store.SetConfig(cfg)
	return nil
}

func (r *Runtime) Snapshot() model.Snapshot {
	return r.store.Snapshot()
}

func (r *Runtime) RecentSongs(ctx context.Context, limit int) ([]song.Song, error) {
	if r.songs == nil {
		return nil, errors.New("song database is not available")
	}
	return r.songs.Recent(ctx, limit)
}

func (r *Runtime) SearchSongs(ctx context.Context, query string, limit int) ([]song.Song, error) {
	if r.songs == nil {
		return nil, errors.New("song database is not available")
	}
	return r.songs.Search(ctx, query, limit)
}

func (r *Runtime) GetSong(ctx context.Context, id int64) (song.Song, error) {
	if r.songs == nil {
		return song.Song{}, errors.New("song database is not available")
	}
	return r.songs.Get(ctx, id)
}

func (r *Runtime) CreateSong(ctx context.Context, input song.Input) (song.Song, error) {
	if r.songs == nil {
		return song.Song{}, errors.New("song database is not available")
	}
	created, err := r.songs.Create(ctx, input)
	if err != nil {
		return song.Song{}, err
	}
	r.store.AddLog("Song saved: " + created.Title)
	r.store.Notify("songs_changed", created.ID)
	return created, nil
}

func (r *Runtime) UpdateSong(ctx context.Context, id int64, input song.Input) (song.Song, error) {
	if r.songs == nil {
		return song.Song{}, errors.New("song database is not available")
	}
	updated, err := r.songs.Update(ctx, id, input)
	if err != nil {
		return song.Song{}, err
	}
	r.store.AddLog("Song updated: " + updated.Title)
	r.store.Notify("songs_changed", updated.ID)
	return updated, nil
}

func (r *Runtime) DeleteSong(ctx context.Context, id int64) error {
	if r.songs == nil {
		return errors.New("song database is not available")
	}
	if err := r.songs.Delete(ctx, id); err != nil {
		return err
	}
	r.store.AddLog("Song deleted")
	r.store.Notify("songs_changed", id)
	return nil
}

func (r *Runtime) SongLyrics(ctx context.Context, songID int64) ([]song.LyricSource, error) {
	if r.songs == nil {
		return nil, errors.New("song database is not available")
	}
	return r.songs.Lyrics(ctx, songID)
}

func (r *Runtime) SetSongLyric(ctx context.Context, songID int64, source string, rawLyric string, ttmlLyric string) error {
	if r.songs == nil {
		return errors.New("song database is not available")
	}
	if err := r.songs.SetLyric(ctx, songID, source, rawLyric, ttmlLyric); err != nil {
		return err
	}
	if current, err := r.songs.Get(ctx, songID); err == nil && r.isCurrentSong(current) {
		r.publishBestLyricForSong(ctx, current, r.store.Snapshot().Track)
	}
	r.store.Notify("songs_changed", songID)
	return nil
}

func (r *Runtime) UpdateTTMLDBIndex(ctx context.Context) error {
	if r.lyrics == nil {
		return errors.New("lyric search service is not available")
	}
	cfg := r.store.Config()
	updatedAt, count, err := r.lyrics.UpdateTTMLDBIndex(ctx, cfg.TTMLDB)
	if err != nil {
		return err
	}
	if !updatedAt.IsZero() {
		cfg.TTMLDB.LastUpdatedAt = model.FormatTime(updatedAt)
		if err := r.SaveConfig(cfg); err != nil {
			return err
		}
	}
	r.store.AddLog("TTML DB index updated: " + intText(count) + " entries")
	return nil
}

func (r *Runtime) startSongRecorder(ctx context.Context) {
	if r.songs == nil {
		r.store.AddLog("Song database unavailable")
		return
	}
	events, unsubscribe := r.store.Subscribe(128)
	go func() {
		defer unsubscribe()
		lastKey := ""
		lastKey = r.recordTrackIfChanged(ctx, r.store.Snapshot().Track, lastKey)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type != "track_changed" {
					continue
				}
				track, ok := event.Payload.(model.Track)
				if !ok {
					continue
				}
				lastKey = r.recordTrackIfChanged(ctx, track, lastKey)
			}
		}
	}()
}

func (r *Runtime) recordTrackIfChanged(ctx context.Context, track model.Track, lastKey string) string {
	if isIdleTrack(track) {
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return ""
	}
	input := song.InputFromTrack(track)
	if !input.Recordable() {
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return lastKey
	}
	key := song.UniqueKey(input)
	if key == lastKey {
		return lastKey
	}
	recorded, created, err := r.songs.RecordPlayback(ctx, input)
	if errors.Is(err, song.ErrUnrecordable) {
		return lastKey
	}
	if err != nil {
		r.store.AddLog("Song record failed: " + err.Error())
		return lastKey
	}
	if created {
		r.store.AddLog("Song recorded: " + recorded.Title)
	} else {
		r.store.AddLog("Song play count updated: " + recorded.Title)
	}
	r.store.Notify("songs_changed", recorded.ID)
	if r.publishBestLyricForSong(ctx, recorded, track) {
		r.cancelLyricSearch()
	} else {
		r.startLyricSearch(ctx, recorded, track)
	}
	return key
}

func (r *Runtime) publishBestLyricForSong(ctx context.Context, item song.Song, track model.Track) bool {
	if r.songs == nil || item.ID <= 0 {
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	lyrics, err := r.songs.Lyrics(ctx, item.ID)
	if err != nil {
		r.store.AddLog("Lyric load failed: " + err.Error())
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	selected, ok := selectBestLyric(lyrics, r.store.Config().Lyrics.SearchPriority)
	if !ok {
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	content := strings.TrimSpace(selected.TTMLLyric)
	if content == "" {
		content = strings.TrimSpace(selected.RawLyric)
	}
	document, err := lyric.Parse(content)
	if err != nil || !lyric.IsUsable(document) {
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	ttmlText := selected.TTMLLyric
	if strings.TrimSpace(ttmlText) == "" {
		ttmlText = lyric.GenerateTTML(document, false)
	}
	r.store.SetLyrics(model.CurrentLyrics{
		TrackID:   track.ID,
		Source:    selected.Source,
		Lines:     currentLyricLines(document),
		TTML:      ttmlText,
		UpdatedAt: model.Now(),
	})
	return true
}

func (r *Runtime) startLyricSearch(parent context.Context, item song.Song, track model.Track) {
	if r.lyrics == nil || r.songs == nil || item.ID <= 0 {
		return
	}
	r.cancelLyricSearch()
	ctx, cancel := context.WithCancel(parent)
	r.lyricMu.Lock()
	r.lyricSeq++
	seq := r.lyricSeq
	r.lyricCancel = cancel
	r.lyricMu.Unlock()

	query := lyric.TrackQuery{
		Title:      item.Title,
		Artist:     item.Artist,
		Album:      item.Album,
		DurationMs: item.DurationMs,
	}
	cfg := r.store.Config()
	r.store.AddLog("Searching lyrics: " + item.Title)

	go func() {
		defer func() {
			r.lyricMu.Lock()
			if r.lyricSeq == seq {
				r.lyricCancel = nil
			}
			r.lyricMu.Unlock()
		}()
		result := r.lyrics.SearchAll(ctx, query, cfg)
		if ctx.Err() != nil {
			return
		}
		saved := 0
		for _, providerResult := range result.Results {
			if err := r.songs.SetLyricWithMeta(ctx, item.ID, providerResult.Source, providerResult.SourceTrackID, providerResult.RawLyric, providerResult.TTMLLyric); err != nil {
				r.store.AddLog("Lyric save failed [" + providerResult.Source + "]: " + err.Error())
				continue
			}
			saved++
		}
		if result.TTMLDBUpdatedAt.After(timeFromConfig(cfg.TTMLDB.LastUpdatedAt)) {
			next := r.store.Config()
			next.TTMLDB.LastUpdatedAt = model.FormatTime(result.TTMLDBUpdatedAt)
			_ = r.SaveConfig(next)
		}
		if saved == 0 {
			r.store.AddLog("No usable lyrics found: " + item.Title)
			r.store.Notify("songs_changed", item.ID)
			return
		}
		selected, ok := lyric.SelectBestResult(result.Results, cfg.Lyrics.SearchPriority)
		if ok {
			if err := r.songs.ApplyLyricSource(ctx, item.ID, selected.Source); err != nil && !errors.Is(err, song.ErrNotFound) {
				r.store.AddLog("Lyric apply failed: " + err.Error())
			}
			r.store.AddLog("Lyric applied [" + selected.Source + "]: " + item.Title)
		}
		r.store.Notify("songs_changed", item.ID)
		current, err := r.songs.Get(ctx, item.ID)
		if err == nil && r.isCurrentSong(current) {
			r.publishBestLyricForSong(ctx, current, track)
		}
	}()
}

func (r *Runtime) cancelLyricSearch() {
	r.lyricMu.Lock()
	cancel := r.lyricCancel
	r.lyricCancel = nil
	r.lyricMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *Runtime) isCurrentSong(item song.Song) bool {
	current := song.InputFromTrack(r.store.Snapshot().Track)
	if !current.Recordable() {
		return false
	}
	return song.UniqueKey(current) == item.UniqueKey
}

func selectBestLyric(lyrics []song.LyricSource, priority []string) (song.LyricSource, bool) {
	bySource := map[string]song.LyricSource{}
	for _, item := range lyrics {
		if !item.Available {
			continue
		}
		if strings.TrimSpace(item.TTMLLyric) == "" && strings.TrimSpace(item.RawLyric) == "" {
			continue
		}
		bySource[item.Source] = item
	}
	for _, source := range priority {
		if item, ok := bySource[source]; ok {
			return item, true
		}
	}
	for _, source := range song.LyricSources {
		if item, ok := bySource[source]; ok {
			return item, true
		}
	}
	return song.LyricSource{}, false
}

func currentLyricLines(document lyric.Document) []model.CurrentLyricLine {
	previews := lyric.PreviewLines(document, len(document.Lines))
	out := make([]model.CurrentLyricLine, 0, len(previews))
	for _, line := range previews {
		out = append(out, model.CurrentLyricLine{
			StartTimeMs: line.StartTimeMs,
			EndTimeMs:   line.EndTimeMs,
			Text:        line.Text,
			Translation: line.Translation,
			Roman:       line.Roman,
		})
	}
	return out
}

func boolText(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func intText(v int) string {
	return strconv.Itoa(v)
}

func timeFromConfig(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func (r *Runtime) context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

func isIdleTrack(track model.Track) bool {
	id := strings.TrimSpace(track.ID)
	return id == "" || id == "idle"
}
