package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const (
	configDirName       = "LyricSync"
	legacyConfigDirName = "LyricSyncBasic"
	configFileName      = "config.json"
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
		Lyrics: model.LyricsConfig{
			SearchPriority: []string{
				model.LyricSourceTTMLDB,
				model.LyricSourceQQ,
				model.LyricSourceKugou,
				model.LyricSourceNetease,
				model.LyricSourceCustom,
			},
			CleanStrategy: model.LyricsCleanSoftware,
		},
		AI: model.AIConfig{
			BaseURL:          "https://api.openai.com/v1",
			TimeoutSeconds:   120,
			ApplyWaitSeconds: 3,
		},
		App: model.AppConfig{
			CloseBehavior: model.CloseBehaviorExit,
		},
		TTMLDB: model.TTMLDBConfig{
			AutoUpdateIndex:     true,
			UpdateIntervalHours: 24,
			IndexURL:            "https://amlldb.bikonoo.com/metadata/raw-lyrics-index.jsonl",
		},
	}
}

func Path() (string, error) {
	return configPath(configDirName)
}

func legacyPath() (string, error) {
	return configPath(legacyConfigDirName)
}

func configPath(dirName string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName, configFileName), nil
}

func Load() (model.Config, error) {
	defaults := Default()
	path, err := Path()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy, legacyErr := loadLegacy(defaults)
		if legacyErr != nil {
			return defaults, legacyErr
		}
		if legacy != nil {
			_ = Save(*legacy)
			return *legacy, nil
		}
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	return decode(data, defaults)
}

func loadLegacy(defaults model.Config) (*model.Config, error) {
	path, err := legacyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg, err := decode(data, defaults)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func decode(data []byte, defaults model.Config) (model.Config, error) {
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, err
	}
	cfg = merge(defaults, cfg)
	if !hasNestedField(data, "amll", "sendAudio") {
		cfg.AMLL.SendAudio = defaults.AMLL.SendAudio
	}
	if !hasNestedField(data, "ttmlDb", "autoUpdateIndex") {
		cfg.TTMLDB.AutoUpdateIndex = defaults.TTMLDB.AutoUpdateIndex
	}
	if !hasNestedField(data, "ai", "applyWaitSeconds") {
		cfg.AI.ApplyWaitSeconds = defaults.AI.ApplyWaitSeconds
	}
	return cfg, nil
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
	aiWasEmpty := cfg.AI.BaseURL == "" && cfg.AI.APIKey == "" && cfg.AI.Model == "" && len(cfg.AI.Models) == 0 && !cfg.AI.DeepThinking && cfg.AI.TimeoutSeconds == 0 && cfg.AI.ApplyWaitSeconds == 0
	if cfg.AMLL.URL == "" {
		cfg.AMLL.URL = defaults.AMLL.URL
	}
	if len(cfg.Lyrics.SearchPriority) == 0 {
		cfg.Lyrics.SearchPriority = append([]string(nil), defaults.Lyrics.SearchPriority...)
	} else {
		cfg.Lyrics.SearchPriority = normalizePriority(cfg.Lyrics.SearchPriority, defaults.Lyrics.SearchPriority)
	}
	if cfg.Lyrics.CleanStrategy == "" {
		cfg.Lyrics.CleanStrategy = defaults.Lyrics.CleanStrategy
	}
	if cfg.AI.BaseURL == "" {
		cfg.AI.BaseURL = defaults.AI.BaseURL
	}
	if cfg.AI.TimeoutSeconds <= 0 {
		cfg.AI.TimeoutSeconds = defaults.AI.TimeoutSeconds
	}
	if cfg.AI.TimeoutSeconds < 30 {
		cfg.AI.TimeoutSeconds = 30
	}
	if cfg.AI.TimeoutSeconds > 600 {
		cfg.AI.TimeoutSeconds = 600
	}
	if cfg.AI.ApplyWaitSeconds < 0 {
		cfg.AI.ApplyWaitSeconds = 0
	}
	if cfg.AI.ApplyWaitSeconds > 15 {
		cfg.AI.ApplyWaitSeconds = 15
	}
	if aiWasEmpty && cfg.AI.ApplyWaitSeconds == 0 {
		cfg.AI.ApplyWaitSeconds = defaults.AI.ApplyWaitSeconds
	}
	cfg.AI.Models = normalizeModels(cfg.AI.Models)
	if cfg.App.CloseBehavior == "" {
		cfg.App.CloseBehavior = defaults.App.CloseBehavior
	}
	if cfg.TTMLDB.IndexURL == "" && cfg.TTMLDB.UpdateIntervalHours == 0 && cfg.TTMLDB.LastUpdatedAt == "" && !cfg.TTMLDB.AutoUpdateIndex {
		cfg.TTMLDB = defaults.TTMLDB
	} else {
		if cfg.TTMLDB.IndexURL == "" {
			cfg.TTMLDB.IndexURL = defaults.TTMLDB.IndexURL
		}
		if cfg.TTMLDB.UpdateIntervalHours <= 0 {
			cfg.TTMLDB.UpdateIntervalHours = defaults.TTMLDB.UpdateIntervalHours
		}
	}
	return cfg
}

func normalizeModels(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizePriority(values []string, defaults []string) []string {
	allowed := map[string]struct{}{
		model.LyricSourceTTMLDB:  {},
		model.LyricSourceQQ:      {},
		model.LyricSourceKugou:   {},
		model.LyricSourceNetease: {},
		model.LyricSourceCustom:  {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(defaults))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range defaults {
		if _, ok := seen[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func hasNestedField(data []byte, objectName string, fieldName string) bool {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false
	}
	rawObject, ok := root[objectName]
	if !ok {
		return false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawObject, &object); err != nil {
		return false
	}
	_, ok = object[fieldName]
	return ok
}
