package main

import (
	"context"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"lyricsync/internal/service"
	"lyricsync/pkg/model"
)

type App struct {
	ctx     context.Context
	manager *service.Manager
	cancel  func()
}

func NewApp() *App {
	manager, err := service.NewManager()
	if err != nil {
		panic(err)
	}
	return &App{manager: manager}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetWindowController(runtimeWindowController{ctx: ctx, app: a})
	if err := a.manager.Start(ctx); err != nil {
		a.manager.State.AddLog("error", "app", err.Error())
	}
	if a.manager.State.Config().UI.StartHidden {
		go func() {
			time.Sleep(300 * time.Millisecond)
			wailsruntime.WindowHide(ctx)
			a.manager.State.AddLog("info", "window", "window hidden after startup")
		}()
	}

	events, cancel := a.manager.State.Subscribe(128)
	a.cancel = cancel
	go func() {
		for event := range events {
			wailsruntime.EventsEmit(ctx, "lyricsync:event", event)
			if event.Type != "audio_frame" {
				wailsruntime.EventsEmit(ctx, "lyricsync:state", a.manager.State.Snapshot())
			}
		}
	}()
}

func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.manager.Stop(stopCtx)
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.manager.State.Config().UI.HideOnClose {
		wailsruntime.WindowHide(ctx)
		a.manager.State.AddLog("info", "window", "close requested; window hidden to background")
		return true
	}
	return false
}

func (a *App) GetState() model.AppSnapshot {
	return a.manager.State.Snapshot()
}

func (a *App) GetConfig() model.AppConfig {
	return a.manager.State.Config()
}

func (a *App) SaveConfig(cfg model.AppConfig) error {
	return a.manager.SaveConfig(cfg)
}

func (a *App) SelectSession(sessionID string) error {
	return a.manager.SelectSession(sessionID)
}

func (a *App) RestartServices() error {
	return a.manager.Restart(a.ctx)
}

func (a *App) SearchLyrics(title, artist, album string) (model.LyricDocument, error) {
	return a.manager.SearchLyrics(a.ctx, title, artist, album)
}

func (a *App) SearchLyricCandidates(title, artist, album string) ([]model.LyricSearchCandidate, error) {
	return a.manager.SearchLyricCandidates(a.ctx, title, artist, album)
}

func (a *App) ApplyLyricCandidate(candidate model.LyricSearchCandidate) (model.LyricDocument, error) {
	return a.manager.ApplyLyricCandidate(a.ctx, candidate)
}

func (a *App) ImportLyrics(path string) (model.LyricDocument, error) {
	return a.manager.ImportLyrics(a.ctx, path)
}

func (a *App) OffsetLyrics(offsetMs int64) (model.LyricDocument, error) {
	return a.manager.OffsetLyrics(offsetMs)
}

func (a *App) AlignLyrics(lineIndex int, positionMs int64) (model.LyricDocument, error) {
	return a.manager.AlignLyrics(lineIndex, positionMs)
}

func (a *App) SuggestLyricAlignment(lineIndex int) (model.LyricCalibrationSuggestion, error) {
	return a.manager.SuggestLyricAlignment(lineIndex)
}

func (a *App) AutoAlignLyrics(lineIndex int) (model.LyricCalibrationResult, error) {
	return a.manager.AutoAlignLyrics(lineIndex)
}

func (a *App) RunAITask(request model.AITaskRequest) (model.AITaskResult, error) {
	return a.manager.RunAITask(a.ctx, request)
}

func (a *App) ControlPlayback(command string, positionMs int64) error {
	return a.manager.ControlPlayback(command, positionMs)
}

func (a *App) SetVolume(level float64) error {
	return a.manager.SetVolume(level)
}

func (a *App) ConnectAMLL(url string) error {
	return a.manager.ConnectAMLL(a.ctx, url)
}

func (a *App) DisconnectAMLL() error {
	return a.manager.DisconnectAMLL()
}

func (a *App) SendLyricsToAMLL() error {
	return a.manager.SendLyricsToAMLL()
}

func (a *App) ConfigPath() string {
	return a.manager.ConfigPath()
}

func (a *App) MinimizeWindow() {
	if a.manager.State.Config().UI.MinimizeToTray {
		wailsruntime.WindowHide(a.ctx)
		a.manager.State.AddLog("info", "window", "minimize requested; window hidden to background")
		return
	}
	wailsruntime.WindowMinimise(a.ctx)
	a.manager.State.AddLog("info", "window", "window minimized to taskbar")
}

func (a *App) HideWindow() {
	wailsruntime.WindowHide(a.ctx)
	a.manager.State.AddLog("info", "window", "window hidden to background")
}

func (a *App) ShowWindow() {
	wailsruntime.WindowShow(a.ctx)
	a.manager.State.AddLog("info", "window", "window shown")
}

func (a *App) Quit() {
	a.shutdown(a.ctx)
	wailsruntime.Quit(a.ctx)
}

type runtimeWindowController struct {
	ctx context.Context
	app *App
}

func (c runtimeWindowController) ShowWindow() {
	wailsruntime.WindowShow(c.ctx)
	if c.app != nil {
		c.app.manager.State.AddLog("info", "window", "window shown through HTTP API")
	}
}

func (c runtimeWindowController) HideWindow() {
	wailsruntime.WindowHide(c.ctx)
	if c.app != nil {
		c.app.manager.State.AddLog("info", "window", "window hidden through HTTP API")
	}
}

func (c runtimeWindowController) MinimizeWindow() {
	if c.app != nil {
		c.app.MinimizeWindow()
		return
	}
	wailsruntime.WindowMinimise(c.ctx)
}
