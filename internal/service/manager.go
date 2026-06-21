package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"lyricsync/internal/ai"
	"lyricsync/internal/amllclient"
	"lyricsync/internal/api"
	"lyricsync/internal/audio"
	"lyricsync/internal/config"
	"lyricsync/internal/core"
	"lyricsync/internal/lyric"
	"lyricsync/internal/smtc"
	"lyricsync/internal/volume"
	"lyricsync/pkg/model"
)

type Manager struct {
	State      *core.State
	config     *config.Store
	api        *api.Server
	amll       *amllclient.Connector
	audio      audio.Provider
	smtc       smtc.Provider
	control    *smtc.Controller
	volume     *volume.Controller
	lyrics     *lyric.Manager
	aiCache    map[string]model.AITaskResult
	cancel     context.CancelFunc
	started    bool
	configPath string
	mu         sync.Mutex
	aiMu       sync.Mutex
	aiRunMu    sync.Mutex
}

func NewManager() (*Manager, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}

	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	state := core.NewState(cfg)
	return &Manager{
		State:      state,
		config:     store,
		api:        api.NewServer(state),
		amll:       amllclient.NewConnector(state),
		audio:      audio.NewProvider(state),
		smtc:       smtc.NewProvider(state),
		control:    smtc.NewController(state),
		volume:     volume.NewController(state),
		lyrics:     lyric.NewManager(state),
		aiCache:    map[string]model.AITaskResult{},
		configPath: store.Path(),
	}, nil
}

func (m *Manager) Start(parent context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.started = true

	m.State.SetService("app", "running", "LyricSync services started")
	m.State.AddLog("info", "app", "configuration loaded from "+m.configPath)
	m.volume.Refresh()

	m.lyrics.Start(ctx)
	if err := m.smtc.Start(ctx); err != nil {
		m.State.AddLog("error", "smtc", err.Error())
	}
	if err := m.audio.Start(ctx); err != nil {
		m.State.AddLog("error", "audio", err.Error())
	}
	m.amll.SetCommandHandlers(m.ControlPlayback, m.SetVolume)
	if cfg := m.State.Config().AMLL; cfg.Enabled && cfg.AutoConnect {
		if err := m.amll.Connect(ctx, cfg.URL, cfg.SendAudio); err != nil {
			m.State.AddLog("warn", "amll-client", err.Error())
		}
	}
	if err := m.api.Start(ctx, m.State.Config().Server); err != nil {
		m.State.SetService("api", "error", err.Error())
		return err
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	cancel := m.cancel
	m.started = false
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var errs []error
	m.amll.Disconnect()
	if err := m.api.Stop(ctx); err != nil {
		errs = append(errs, err)
	}
	m.State.SetService("app", "stopped", "LyricSync services stopped")
	m.State.AddLog("info", "app", "services stopped")
	return errors.Join(errs...)
}

func (m *Manager) SaveConfig(cfg model.AppConfig) error {
	cfg = normalizeConfig(cfg)
	if err := m.config.Save(cfg); err != nil {
		return err
	}
	m.State.SetConfig(cfg)
	m.State.AddLog("info", "config", "configuration saved")
	return nil
}

func (m *Manager) SelectSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session: id is required")
	}
	snapshot := m.State.Snapshot()
	var selected model.Session
	for _, session := range snapshot.Sessions {
		if session.ID == sessionID || session.AppID == sessionID {
			selected = session
			break
		}
	}
	if selected.ID == "" {
		return fmt.Errorf("session: %s not found", sessionID)
	}

	cfg := m.State.Config()
	cfg.Session.AutoSelect = false
	cfg.Session.Preferred = prioritizeSession(selected.ID, cfg.Session.Preferred)
	if err := m.SaveConfig(cfg); err != nil {
		return err
	}
	m.State.AddLog("info", "session", "selected SMTC session: "+selected.Name)
	return nil
}

func (m *Manager) ConfigPath() string {
	return m.configPath
}

func (m *Manager) SetWindowController(window api.WindowController) {
	m.api.SetWindowController(window)
}

func (m *Manager) SearchLyrics(ctx context.Context, title, artist, album string) (model.LyricDocument, error) {
	return m.lyrics.Search(ctx, title, artist, album)
}

func (m *Manager) SearchLyricCandidates(ctx context.Context, title, artist, album string) ([]model.LyricSearchCandidate, error) {
	return m.lyrics.SearchCandidates(ctx, title, artist, album)
}

func (m *Manager) ApplyLyricCandidate(ctx context.Context, candidate model.LyricSearchCandidate) (model.LyricDocument, error) {
	return m.lyrics.ApplyCandidate(ctx, candidate)
}

func (m *Manager) ImportLyrics(ctx context.Context, path string) (model.LyricDocument, error) {
	return m.lyrics.ImportFile(ctx, path)
}

func (m *Manager) OffsetLyrics(offsetMs int64) (model.LyricDocument, error) {
	return m.lyrics.OffsetCurrent(offsetMs)
}

func (m *Manager) AlignLyrics(lineIndex int, positionMs int64) (model.LyricDocument, error) {
	return m.lyrics.AlignCurrent(lineIndex, positionMs)
}

func (m *Manager) SuggestLyricAlignment(lineIndex int) (model.LyricCalibrationSuggestion, error) {
	return m.lyrics.SuggestAlignment(lineIndex)
}

func (m *Manager) AutoAlignLyrics(lineIndex int) (model.LyricCalibrationResult, error) {
	return m.lyrics.AutoAlignCurrent(lineIndex)
}

func (m *Manager) RunAITask(ctx context.Context, request model.AITaskRequest) (model.AITaskResult, error) {
	start := time.Now()
	if !m.aiRunMu.TryLock() {
		m.State.SetService("ai", "queued", "waiting for current AI task")
		m.State.AddLog("info", "ai", "queued "+request.Task+" task")
		m.aiRunMu.Lock()
	}
	defer m.aiRunMu.Unlock()

	doc := m.State.Snapshot().Lyrics
	if len(doc.Lines) == 0 {
		return model.AITaskResult{}, fmt.Errorf("ai: no current lyrics")
	}

	cacheKey := aiTaskCacheKey(m.State.Config().AI, request, doc)
	m.aiMu.Lock()
	if cached, ok := m.aiCache[cacheKey]; ok {
		m.aiMu.Unlock()
		m.State.SetLyrics(cached.UpdatedLyrics)
		m.State.SetService("ai", "running", "cached task restored")
		m.State.AddLog("info", "ai", fmt.Sprintf("cache hit for %s on %d lyric lines", request.Task, len(doc.Lines)))
		return cached, nil
	}
	m.aiMu.Unlock()

	m.State.SetService("ai", "running", "processing "+request.Task)
	m.State.AddLog("info", "ai", fmt.Sprintf("starting %s for %d lyric lines", request.Task, len(doc.Lines)))

	client := ai.NewClient(m.State.Config().AI)
	processed, err := client.Process(ctx, request, doc)
	if err != nil {
		m.State.SetService("ai", "error", err.Error())
		m.State.AddLog("warn", "ai", err.Error())
		return model.AITaskResult{}, err
	}
	rebuilt, err := lyric.RebuildDocument(processed)
	if err != nil {
		m.State.SetService("ai", "error", err.Error())
		m.State.AddLog("warn", "ai", err.Error())
		return model.AITaskResult{}, err
	}

	m.State.SetLyrics(rebuilt)
	m.State.SetService("ai", "running", "last task completed")
	m.State.AddLog("info", "ai", fmt.Sprintf("completed %s for %d lyric lines", request.Task, len(rebuilt.Lines)))
	result := model.AITaskResult{
		Task:           request.Task,
		TargetLanguage: request.TargetLanguage,
		LineCount:      len(rebuilt.Lines),
		DurationMs:     time.Since(start).Milliseconds(),
		UpdatedLyrics:  rebuilt,
	}
	m.aiMu.Lock()
	m.aiCache[cacheKey] = result
	if len(m.aiCache) > 32 {
		m.aiCache = map[string]model.AITaskResult{cacheKey: result}
	}
	m.aiMu.Unlock()
	return result, nil
}

func (m *Manager) ControlPlayback(command string, positionMs int64) error {
	return m.control.Command(command, positionMs)
}

func (m *Manager) SetVolume(level float64) error {
	return m.volume.Set(level)
}

func (m *Manager) ConnectAMLL(ctx context.Context, url string) error {
	cfg := m.State.Config()
	cfg.AMLL.Enabled = true
	cfg.AMLL.URL = url
	if cfg.AMLL.URL == "" {
		cfg.AMLL.URL = model.DefaultConfig().AMLL.URL
	}
	if err := m.SaveConfig(cfg); err != nil {
		return err
	}
	return m.amll.Connect(ctx, cfg.AMLL.URL, cfg.AMLL.SendAudio)
}

func (m *Manager) DisconnectAMLL() error {
	cfg := m.State.Config()
	cfg.AMLL.Enabled = false
	if err := m.SaveConfig(cfg); err != nil {
		return err
	}
	m.amll.Disconnect()
	return nil
}

func (m *Manager) SendLyricsToAMLL() error {
	return m.amll.SendLyrics()
}

func normalizeConfig(cfg model.AppConfig) model.AppConfig {
	defaults := model.DefaultConfig()
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		cfg.Server.Port = defaults.Server.Port
	}
	if cfg.Audio.SampleRate <= 0 {
		cfg.Audio.SampleRate = defaults.Audio.SampleRate
	}
	if cfg.Audio.Channels <= 0 {
		cfg.Audio.Channels = defaults.Audio.Channels
	}
	if cfg.Audio.FrameDurationMs < 10 {
		cfg.Audio.FrameDurationMs = defaults.Audio.FrameDurationMs
	}
	if cfg.Audio.Mode == "" {
		cfg.Audio.Mode = defaults.Audio.Mode
	}
	if cfg.Audio.Format == "" {
		cfg.Audio.Format = defaults.Audio.Format
	}
	if cfg.AI.Timeout <= 0 {
		cfg.AI.Timeout = defaults.AI.Timeout
	}
	if cfg.AI.BaseURL == "" {
		cfg.AI.BaseURL = defaults.AI.BaseURL
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = defaults.AI.Model
	}
	if len(cfg.Lyrics.Sources) == 0 {
		cfg.Lyrics.Sources = defaults.Lyrics.Sources
	}
	if cfg.AMLL.URL == "" {
		cfg.AMLL.URL = defaults.AMLL.URL
	}
	return cfg
}

func prioritizeSession(sessionID string, preferred []string) []string {
	next := []string{sessionID}
	for _, item := range preferred {
		if item != "" && item != sessionID {
			next = append(next, item)
		}
	}
	return next
}

func aiTaskCacheKey(cfg model.AIConfig, request model.AITaskRequest, doc model.LyricDocument) string {
	type cacheInput struct {
		Model   string              `json:"model"`
		Request model.AITaskRequest `json:"request"`
		Lines   []model.LyricLine   `json:"lines"`
	}
	data, _ := json.Marshal(cacheInput{
		Model:   cfg.Model,
		Request: request,
		Lines:   doc.Lines,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) Restart(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Stop(timeoutCtx); err != nil {
		return fmt.Errorf("stop services: %w", err)
	}
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("start services: %w", err)
	}
	return nil
}
