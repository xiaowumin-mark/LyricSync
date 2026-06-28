package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	aipkg "github.com/xiaowumin-mark/LyricSync/internal/ai"
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
	lyricRev    uint64
	lyricKey    string
	aiCancel    context.CancelFunc
	aiLimiter   chan struct{}
}

const lyricSearchDebounce = 250 * time.Millisecond

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
		aiLimiter: make(chan struct{}, 1),
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
	r.connector.Disconnect()
	if r.media != nil {
		r.media.Stop()
	}
	r.cancelLyricSearch()
	r.cancelAIProcessing()
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

func (r *Runtime) CurrentSong(ctx context.Context) (song.Song, error) {
	if r.songs == nil {
		return song.Song{}, errors.New("song database is not available")
	}
	input := song.InputFromTrack(r.store.Snapshot().Track)
	if !input.Recordable() {
		return song.Song{}, song.ErrUnrecordable
	}
	return r.songs.GetByInput(ctx, input)
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
	r.refreshLyricsForCurrentSong(ctx, created)
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
	r.refreshLyricsForCurrentSong(ctx, updated)
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

func (r *Runtime) SetSongLyricDelay(ctx context.Context, songID int64, source string, delayMs int64) error {
	if r.songs == nil {
		return errors.New("song database is not available")
	}
	if err := r.songs.SetLyricDelay(ctx, songID, source, delayMs); err != nil {
		return err
	}
	if current, err := r.songs.Get(ctx, songID); err == nil && r.isCurrentSong(current) {
		r.publishBestLyricForSong(ctx, current, r.store.Snapshot().Track)
	}
	r.store.Notify("songs_changed", songID)
	return nil
}

func (r *Runtime) refreshLyricsForCurrentSong(ctx context.Context, item song.Song) {
	if item.ID <= 0 || !r.isCurrentSong(item) {
		return
	}
	r.publishBestLyricForSong(ctx, item, r.store.Snapshot().Track)
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

func (r *Runtime) FetchAIModels(ctx context.Context) ([]string, error) {
	cfg := r.store.Config()
	models, err := aipkg.ListModels(ctx, cfg.AI)
	if err != nil {
		return nil, err
	}
	next := r.store.Config()
	next.AI.Models = models
	if strings.TrimSpace(next.AI.Model) == "" && len(models) > 0 {
		next.AI.Model = models[0]
	}
	if err := r.SaveConfig(next); err != nil {
		return nil, err
	}
	r.store.AddLog("AI models updated: " + intText(len(models)))
	return models, nil
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
		r.advanceLyricRevision("")
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return ""
	}
	input := song.InputFromTrack(track)
	if !input.Recordable() {
		r.advanceLyricRevision("")
		r.store.SetLyrics(model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return lastKey
	}
	key := song.UniqueKey(input)
	if key == lastKey {
		return lastKey
	}
	revision := r.advanceLyricRevision(key)
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
	if r.publishBestLyricForRevision(ctx, revision, key, recorded, track) {
		r.cancelLyricSearch()
		r.startAIEnhancement(ctx, revision, key, recorded, track)
	} else {
		r.startLyricSearch(ctx, revision, key, recorded, track)
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
	selected, ok := selectBestLyric(lyrics, lyricPriorityForSong(item, r.store.Config().Lyrics.SearchPriority))
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
		DelayMs:   selected.DelayMs,
		UpdatedAt: model.Now(),
	})
	return true
}

func (r *Runtime) publishBestLyricForRevision(ctx context.Context, revision uint64, key string, item song.Song, track model.Track) bool {
	if !r.lyricRevisionActive(revision, key, track) {
		return false
	}
	if r.songs == nil || item.ID <= 0 {
		r.setLyricsForRevision(revision, key, track, model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	lyrics, err := r.songs.Lyrics(ctx, item.ID)
	if err != nil {
		r.store.AddLog("Lyric load failed: " + err.Error())
		r.setLyricsForRevision(revision, key, track, model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	selected, ok := selectBestLyric(lyrics, lyricPriorityForSong(item, r.store.Config().Lyrics.SearchPriority))
	if !ok {
		r.setLyricsForRevision(revision, key, track, model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	content := strings.TrimSpace(selected.TTMLLyric)
	if content == "" {
		content = strings.TrimSpace(selected.RawLyric)
	}
	document, err := lyric.Parse(content)
	if err != nil || !lyric.IsUsable(document) {
		r.setLyricsForRevision(revision, key, track, model.CurrentLyrics{TrackID: track.ID, UpdatedAt: model.Now()})
		return false
	}
	ttmlText := selected.TTMLLyric
	if strings.TrimSpace(ttmlText) == "" {
		ttmlText = lyric.GenerateTTML(document, false)
	}
	return r.setLyricsForRevision(revision, key, track, model.CurrentLyrics{
		TrackID:   track.ID,
		Source:    selected.Source,
		Lines:     currentLyricLines(document),
		TTML:      ttmlText,
		DelayMs:   selected.DelayMs,
		UpdatedAt: model.Now(),
	})
}

func (r *Runtime) startLyricSearch(parent context.Context, revision uint64, key string, item song.Song, track model.Track) {
	if r.lyrics == nil || r.songs == nil || item.ID <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r.lyricMu.Lock()
	r.lyricCancel = cancel
	r.lyricMu.Unlock()

	query := lyric.TrackQuery{
		Title:      item.Title,
		Artist:     item.Artist,
		Album:      item.Album,
		DurationMs: item.DurationMs,
	}
	cfg := r.store.Config()
	r.store.AddLog("Queued lyric search: " + item.Title)

	go func() {
		defer func() {
			r.lyricMu.Lock()
			if r.lyricRev == revision && r.lyricKey == key {
				r.lyricCancel = nil
			}
			r.lyricMu.Unlock()
		}()
		timer := time.NewTimer(lyricSearchDebounce)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if !r.lyricRevisionActive(revision, key, track) {
			return
		}
		r.store.AddLog("Searching lyrics: " + item.Title)
		result := r.lyrics.SearchAll(ctx, query, cfg)
		if ctx.Err() != nil || !r.lyricRevisionActive(revision, key, track) {
			return
		}
		saved := 0
		for _, providerResult := range result.Results {
			if ctx.Err() != nil || !r.lyricRevisionActive(revision, key, track) {
				return
			}
			if err := r.songs.SetLyricWithMeta(ctx, item.ID, providerResult.Source, providerResult.SourceTrackID, providerResult.RawLyric, providerResult.TTMLLyric); err != nil {
				r.store.AddLog("Lyric save failed [" + providerResult.Source + "]: " + err.Error())
				continue
			}
			saved++
		}
		if !r.lyricRevisionActive(revision, key, track) {
			return
		}
		if result.TTMLDBUpdatedAt.After(timeFromConfig(cfg.TTMLDB.LastUpdatedAt)) {
			next := r.store.Config()
			next.TTMLDB.LastUpdatedAt = model.FormatTime(result.TTMLDBUpdatedAt)
			_ = r.SaveConfig(next)
		}
		if saved == 0 {
			r.store.AddLog("No usable lyrics found: " + item.Title)
			if r.lyricRevisionActive(revision, key, track) {
				r.store.Notify("songs_changed", item.ID)
			}
			return
		}
		selected, ok := lyric.SelectBestResult(result.Results, lyricPriorityForSong(item, cfg.Lyrics.SearchPriority))
		if ok {
			if err := r.songs.ApplyLyricSource(ctx, item.ID, selected.Source); err != nil && !errors.Is(err, song.ErrNotFound) {
				r.store.AddLog("Lyric apply failed: " + err.Error())
			}
			r.store.AddLog("Lyric applied [" + selected.Source + "]: " + item.Title)
		}
		if !r.lyricRevisionActive(revision, key, track) {
			return
		}
		r.store.Notify("songs_changed", item.ID)
		current, err := r.songs.Get(ctx, item.ID)
		if err == nil && r.isCurrentSong(current) {
			r.publishBestLyricForRevision(ctx, revision, key, current, track)
			r.startAIEnhancement(ctx, revision, key, current, track)
		}
	}()
}

func (r *Runtime) startAIEnhancement(parent context.Context, revision uint64, key string, item song.Song, track model.Track) {
	if r.songs == nil || item.ID <= 0 || !r.lyricRevisionActive(revision, key, track) {
		return
	}
	cfg := r.store.Config()
	if !aiConfigured(cfg) {
		return
	}
	lyrics, err := r.songs.Lyrics(parent, item.ID)
	if err != nil {
		r.store.AddLog("AI lyric load failed: " + err.Error())
		return
	}
	selected, ok := selectBestLyric(lyrics, lyricPriorityForSong(item, cfg.Lyrics.SearchPriority))
	if !ok || selected.AICleaned {
		return
	}
	if !aiEnhancementAllowedSource(selected.Source) {
		return
	}
	content := strings.TrimSpace(selected.TTMLLyric)
	if content == "" {
		content = strings.TrimSpace(selected.RawLyric)
	}
	document, err := lyric.Parse(content)
	if err != nil || !lyric.IsUsable(document) {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r.lyricMu.Lock()
	if r.lyricRev == revision && r.lyricKey == key {
		if r.aiCancel != nil {
			r.aiCancel()
		}
		r.aiCancel = cancel
	}
	r.lyricMu.Unlock()
	go func() {
		defer func() {
			r.lyricMu.Lock()
			if r.lyricRev == revision && r.lyricKey == key {
				r.aiCancel = nil
			}
			r.lyricMu.Unlock()
			cancel()
		}()
		if !r.lyricRevisionActive(revision, key, track) {
			return
		}
		client, err := aipkg.NewClient(cfg.AI)
		if err != nil {
			if !errors.Is(err, aipkg.ErrDisabled) {
				r.store.AddLog("AI client unavailable: " + err.Error())
			}
			return
		}
		r.store.AddLog("AI processing lyrics: " + item.Title)
		var enhanced lyric.Document
		var changed bool
		err = r.runAIExclusive(ctx, func(taskCtx context.Context) error {
			var enhanceErr error
			enhanced, changed, enhanceErr = client.EnhanceLyrics(taskCtx, document, cfg.Lyrics)
			return enhanceErr
		})
		if err != nil {
			if ctx.Err() == nil {
				r.store.AddLog("AI lyric processing failed: " + err.Error())
			}
			return
		}
		if !changed || !lyric.IsUsable(enhanced) || !r.lyricRevisionActive(revision, key, track) {
			return
		}
		ttmlText := lyric.GenerateTTML(enhanced, false)
		if err := r.songs.SetLyricWithFlags(ctx, item.ID, selected.Source, selected.SourceTrackID, selected.RawLyric, ttmlText, true); err != nil {
			if ctx.Err() == nil {
				r.store.AddLog("AI lyric save failed: " + err.Error())
			}
			return
		}
		if err := r.songs.ApplyLyricSource(ctx, item.ID, selected.Source); err != nil && !errors.Is(err, song.ErrNotFound) {
			r.store.AddLog("AI lyric apply failed: " + err.Error())
		}
		if !r.lyricRevisionActive(revision, key, track) {
			return
		}
		r.store.AddLog("AI lyrics applied [" + selected.Source + "]: " + item.Title)
		r.store.Notify("songs_changed", item.ID)
		current, err := r.songs.Get(ctx, item.ID)
		if err == nil && r.isCurrentSong(current) {
			r.publishBestLyricForRevision(ctx, revision, key, current, track)
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

func (r *Runtime) cancelAIProcessing() {
	r.lyricMu.Lock()
	cancel := r.aiCancel
	r.aiCancel = nil
	r.lyricMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *Runtime) advanceLyricRevision(key string) uint64 {
	r.lyricMu.Lock()
	cancel := r.lyricCancel
	aiCancel := r.aiCancel
	r.lyricCancel = nil
	r.aiCancel = nil
	r.lyricRev++
	r.lyricKey = key
	revision := r.lyricRev
	r.lyricMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if aiCancel != nil {
		aiCancel()
	}
	return revision
}

func (r *Runtime) lyricRevisionActive(revision uint64, key string, track model.Track) bool {
	r.lyricMu.Lock()
	active := r.lyricRev == revision && r.lyricKey == key
	r.lyricMu.Unlock()
	if !active {
		return false
	}
	if strings.TrimSpace(track.ID) == "" {
		return false
	}
	snapshotTrack := r.store.Snapshot().Track
	current := song.InputFromTrack(snapshotTrack)
	if !current.Recordable() {
		return false
	}
	if strings.TrimSpace(track.ID) != "" && snapshotTrack.ID != track.ID {
		return false
	}
	return song.UniqueKey(current) == key
}

func (r *Runtime) setLyricsForRevision(revision uint64, key string, track model.Track, lyrics model.CurrentLyrics) bool {
	if !r.lyricRevisionActive(revision, key, track) {
		return false
	}
	r.store.SetLyrics(lyrics)
	return true
}

func (r *Runtime) runAIExclusive(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	limiter := r.aiLimiter
	if limiter == nil {
		limiter = make(chan struct{}, 1)
		r.aiLimiter = limiter
	}
	select {
	case limiter <- struct{}{}:
		defer func() { <-limiter }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return fn(ctx)
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

func lyricPriorityForSong(item song.Song, configured []string) []string {
	if strings.TrimSpace(item.FixedLyricSource) != "" {
		return []string{item.FixedLyricSource}
	}
	priority := make([]string, 0, len(configured)+len(song.LyricSources))
	seen := map[string]struct{}{}
	add := func(source string) {
		source = strings.TrimSpace(source)
		if source == "" {
			return
		}
		if _, ok := seen[source]; ok {
			return
		}
		for _, allowed := range song.LyricSources {
			if source == allowed {
				seen[source] = struct{}{}
				priority = append(priority, source)
				return
			}
		}
	}
	for _, source := range configured {
		add(source)
	}
	for _, source := range []string{song.SourceTTMLDB, song.SourceQQ, song.SourceKugou, song.SourceNetease, song.SourceCustom} {
		add(source)
	}
	return priority
}

func aiConfigured(cfg model.Config) bool {
	if strings.TrimSpace(cfg.AI.BaseURL) == "" || strings.TrimSpace(cfg.AI.APIKey) == "" || strings.TrimSpace(cfg.AI.Model) == "" {
		return false
	}
	return cfg.Lyrics.CleanStrategy == model.LyricsCleanAI || cfg.Lyrics.AITranslate || cfg.Lyrics.AITransliterate
}

func aiEnhancementAllowedSource(source string) bool {
	for _, platformSource := range song.PlatformLyricSources {
		if source == platformSource {
			return true
		}
	}
	return false
}

func currentLyricLines(document lyric.Document) []model.CurrentLyricLine {
	document = document.Normalized()
	out := make([]model.CurrentLyricLine, 0, len(document.Lines))
	for _, line := range document.Lines {
		text := strings.TrimSpace(line.Text())
		if text == "" {
			continue
		}
		out = append(out, model.CurrentLyricLine{
			StartTimeMs: line.StartTimeMs,
			EndTimeMs:   line.EndTimeMs,
			Text:        text,
			Translation: strings.TrimSpace(line.TranslatedLyric),
			Roman:       strings.TrimSpace(line.RomanLyric),
			Background:  line.IsBackground,
			Duet:        line.IsDuet,
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
