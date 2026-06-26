package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func Default() model.Config {
	return model.Config{
		AMLL: model.AMLLConfig{
			URL:         "ws://127.0.0.1:11444",
			AutoConnect: false,
			SendAudio:   true,
		},
		Media: model.MediaConfig{
			AutoSelect: true,
		},
	}
}

func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "LyricSyncBasic", "config.json"), nil
}

func Load() (model.Config, error) {
	defaults := Default()
	path, err := Path()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, err
	}
	return merge(defaults, cfg), nil
}

func Save(cfg model.Config) error {
	cfg = merge(Default(), cfg)
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func merge(defaults, cfg model.Config) model.Config {
	if cfg.AMLL.URL == "" {
		cfg.AMLL.URL = defaults.AMLL.URL
	}
	return cfg
}
