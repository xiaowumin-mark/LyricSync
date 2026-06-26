package config

import (
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
