package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"lyricsync/pkg/model"
)

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{
		path: filepath.Join(base, "LyricSync", "config.json"),
	}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (model.AppConfig, error) {
	defaults := model.DefaultConfig()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}

	var cfg model.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, err
	}
	return mergeDefaults(defaults, cfg), nil
}

func (s *Store) Save(cfg model.AppConfig) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func mergeDefaults(defaults, cfg model.AppConfig) model.AppConfig {
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaults.Server.Port
	}
	if cfg.Audio.Mode == "" {
		cfg.Audio.Mode = defaults.Audio.Mode
	}
	if cfg.Audio.Format == "" {
		cfg.Audio.Format = defaults.Audio.Format
	}
	if cfg.Audio.SampleRate == 0 {
		cfg.Audio.SampleRate = defaults.Audio.SampleRate
	}
	if cfg.Audio.Channels == 0 {
		cfg.Audio.Channels = defaults.Audio.Channels
	}
	if cfg.Audio.FrameDurationMs == 0 {
		cfg.Audio.FrameDurationMs = defaults.Audio.FrameDurationMs
	}
	if cfg.Lyrics.Sources == nil {
		cfg.Lyrics.Sources = defaults.Lyrics.Sources
	}
	if cfg.AMLL.URL == "" {
		cfg.AMLL.URL = defaults.AMLL.URL
		cfg.AMLL.SendAudio = defaults.AMLL.SendAudio
	}
	if cfg.AI.BaseURL == "" {
		cfg.AI.BaseURL = defaults.AI.BaseURL
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = defaults.AI.Model
	}
	if cfg.AI.Timeout == 0 {
		cfg.AI.Timeout = defaults.AI.Timeout
	}
	return cfg
}
