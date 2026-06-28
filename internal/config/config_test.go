package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestMergePreservesDisabledAudioSending(t *testing.T) {
	got := merge(Default(), model.Config{
		AMLL: model.AMLLConfig{
			URL:       "ws://localhost:1234",
			SendAudio: false,
		},
	})
	if got.AMLL.SendAudio {
		t.Fatal("expected sendAudio=false to be preserved")
	}
}

func TestMergeAppliesDefaultURL(t *testing.T) {
	got := merge(Default(), model.Config{})
	if got.AMLL.URL != Default().AMLL.URL {
		t.Fatalf("expected default URL, got %q", got.AMLL.URL)
	}
}

func TestMergeAppliesFullSettingsDefaults(t *testing.T) {
	got := merge(Default(), model.Config{})
	if len(got.Lyrics.SearchPriority) != 5 {
		t.Fatalf("expected full lyrics priority, got %#v", got.Lyrics.SearchPriority)
	}
	if got.Lyrics.SearchPriority[0] != model.LyricSourceTTMLDB {
		t.Fatalf("expected TTML DB first, got %#v", got.Lyrics.SearchPriority)
	}
	if got.Lyrics.CleanStrategy != model.LyricsCleanSoftware {
		t.Fatalf("expected software clean strategy, got %q", got.Lyrics.CleanStrategy)
	}
	if got.AI.BaseURL == "" {
		t.Fatal("expected default AI base URL")
	}
	if got.AI.TimeoutSeconds != 120 {
		t.Fatalf("expected default AI timeout 120, got %d", got.AI.TimeoutSeconds)
	}
	if got.App.CloseBehavior != model.CloseBehaviorExit {
		t.Fatalf("expected exit close behavior, got %q", got.App.CloseBehavior)
	}
	if !got.TTMLDB.AutoUpdateIndex || got.TTMLDB.UpdateIntervalHours != 24 || got.TTMLDB.IndexURL == "" {
		t.Fatalf("expected default TTML DB config, got %#v", got.TTMLDB)
	}
}

func TestMergePreservesDisabledTTMLDBAutoUpdate(t *testing.T) {
	defaults := Default()
	got := merge(defaults, model.Config{
		TTMLDB: model.TTMLDBConfig{
			AutoUpdateIndex:     false,
			UpdateIntervalHours: defaults.TTMLDB.UpdateIntervalHours,
			IndexURL:            defaults.TTMLDB.IndexURL,
		},
	})
	if got.TTMLDB.AutoUpdateIndex {
		t.Fatal("expected TTML DB auto update=false to be preserved")
	}
}

func TestMergeNormalizesAITimeout(t *testing.T) {
	got := merge(Default(), model.Config{
		AI: model.AIConfig{
			BaseURL:        "https://example.test/v1",
			TimeoutSeconds: 5,
		},
	})
	if got.AI.TimeoutSeconds != 30 {
		t.Fatalf("low timeout = %d, want 30", got.AI.TimeoutSeconds)
	}
	got = merge(Default(), model.Config{
		AI: model.AIConfig{
			BaseURL:        "https://example.test/v1",
			TimeoutSeconds: 999,
		},
	})
	if got.AI.TimeoutSeconds != 600 {
		t.Fatalf("high timeout = %d, want 600", got.AI.TimeoutSeconds)
	}
}

func TestMergeNormalizesLyricPriority(t *testing.T) {
	got := merge(Default(), model.Config{
		Lyrics: model.LyricsConfig{
			SearchPriority: []string{
				model.LyricSourceQQ,
				model.LyricSourceQQ,
				"unknown",
				model.LyricSourceNetease,
			},
		},
	})
	want := []string{
		model.LyricSourceQQ,
		model.LyricSourceNetease,
		model.LyricSourceTTMLDB,
		model.LyricSourceKugou,
		model.LyricSourceCustom,
	}
	for i := range want {
		if got.Lyrics.SearchPriority[i] != want[i] {
			t.Fatalf("priority[%d] = %q, want %q; full=%#v", i, got.Lyrics.SearchPriority[i], want[i], got.Lyrics.SearchPriority)
		}
	}
}

func TestPathUsesFullConfigDirectory(t *testing.T) {
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != configDirName {
		t.Fatalf("expected config directory %q, got %q", configDirName, filepath.Dir(path))
	}
	if filepath.Base(path) != configFileName {
		t.Fatalf("expected config file %q, got %q", configFileName, filepath.Base(path))
	}
}

func TestLoadMigratesLegacyConfig(t *testing.T) {
	tmp := t.TempDir()
	setUserConfigDir(t, tmp)

	legacy := filepath.Join(tmp, legacyConfigDirName, configFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{
		"amll": {"url": "ws://legacy.local"},
		"ttmlDb": {"updateIntervalHours": 12, "indexUrl": "https://example.com/index.jsonl"}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AMLL.URL != "ws://legacy.local" {
		t.Fatalf("expected legacy URL, got %q", cfg.AMLL.URL)
	}
	if !cfg.AMLL.SendAudio {
		t.Fatal("expected missing sendAudio to default to true")
	}
	if !cfg.TTMLDB.AutoUpdateIndex {
		t.Fatal("expected missing TTML DB autoUpdateIndex to default to true")
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected migrated config at %s: %v", path, err)
	}
}

func TestLoadPreservesExplicitDisabledBooleans(t *testing.T) {
	tmp := t.TempDir()
	setUserConfigDir(t, tmp)

	path := filepath.Join(tmp, configDirName, configFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
		"amll": {"url": "ws://local", "sendAudio": false},
		"ttmlDb": {"autoUpdateIndex": false, "updateIntervalHours": 24, "indexUrl": "https://example.com/index.jsonl"}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AMLL.SendAudio {
		t.Fatal("expected explicit sendAudio=false to be preserved")
	}
	if cfg.TTMLDB.AutoUpdateIndex {
		t.Fatal("expected explicit TTML DB autoUpdateIndex=false to be preserved")
	}
}

func setUserConfigDir(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", path)
		return
	}
	t.Setenv("XDG_CONFIG_HOME", path)
	t.Setenv("HOME", path)
}
