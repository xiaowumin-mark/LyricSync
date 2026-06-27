package app

import (
	"context"
	"errors"
	"strings"

	"github.com/xiaowumin-mark/LyricSync/internal/amll"
	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/media"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/song"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

type Runtime struct {
	store     *state.Store
	media     *media.Service
	connector *amll.Connector
	songs     *song.Repository
	ctx       context.Context
	cancel    context.CancelFunc
}

func New(store *state.Store, songs *song.Repository) *Runtime {
	mediaSvc := media.New(store)
	connector := amll.NewConnector(store)
	connector.SetCommandHandlers(mediaSvc.Control, mediaSvc.SetVolume)
	return &Runtime{
		store:     store,
		media:     mediaSvc,
		connector: connector,
		songs:     songs,
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
	r.store.Notify("songs_changed", songID)
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
		return ""
	}
	input := song.InputFromTrack(track)
	if !input.Recordable() {
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
	return key
}

func boolText(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
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
