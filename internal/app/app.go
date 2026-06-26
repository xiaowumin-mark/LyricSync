package app

import (
	"context"
	"strings"

	"github.com/xiaowumin-mark/LyricSync/internal/amll"
	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/media"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
)

type Runtime struct {
	store     *state.Store
	media     *media.Service
	connector *amll.Connector
	ctx       context.Context
	cancel    context.CancelFunc
}

func New(store *state.Store) *Runtime {
	mediaSvc := media.New(store)
	connector := amll.NewConnector(store)
	connector.SetCommandHandlers(mediaSvc.Control, mediaSvc.SetVolume)
	return &Runtime{
		store:     store,
		media:     mediaSvc,
		connector: connector,
	}
}

func (r *Runtime) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	r.ctx = ctx
	r.cancel = cancel
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
