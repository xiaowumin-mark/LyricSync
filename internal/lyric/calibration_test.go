package lyric

import (
	"testing"

	"lyricsync/pkg/model"
)

func TestSuggestAlignmentUsesLocalEnergyPeak(t *testing.T) {
	doc := model.LyricDocument{
		ID:      "lyric",
		TrackID: "track",
		Lines: []model.LyricLine{
			{StartMs: 10000, EndMs: 13000, Text: "first line"},
			{StartMs: 16000, EndMs: 19000, Text: "second line"},
		},
	}
	history := []model.AudioEnergyPoint{
		{Sequence: 1, TrackID: "track", PositionMs: 11600, RMS: 0.05, Peak: 0.10},
		{Sequence: 2, TrackID: "track", PositionMs: 11800, RMS: 0.08, Peak: 0.12},
		{Sequence: 3, TrackID: "track", PositionMs: 12000, RMS: 0.80, Peak: 0.90},
		{Sequence: 4, TrackID: "track", PositionMs: 12200, RMS: 0.45, Peak: 0.55},
		{Sequence: 5, TrackID: "other", PositionMs: 12100, RMS: 0.95, Peak: 0.98},
	}

	suggestion, err := SuggestAlignment(doc, history, 0, 12050, "track")
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.LineIndex != 0 {
		t.Fatalf("unexpected line index: %#v", suggestion)
	}
	if suggestion.SuggestedPositionMs != 12000 {
		t.Fatalf("expected peak at 12000 ms, got %#v", suggestion)
	}
	if suggestion.OffsetMs != 2000 {
		t.Fatalf("expected 2000 ms offset, got %#v", suggestion)
	}
	if suggestion.Confidence <= 0.5 {
		t.Fatalf("expected useful confidence, got %#v", suggestion)
	}
}

func TestSuggestAlignmentCanChooseCurrentLine(t *testing.T) {
	doc := model.LyricDocument{
		ID:      "lyric",
		TrackID: "track",
		Lines: []model.LyricLine{
			{StartMs: 1000, EndMs: 2000, Text: "first"},
			{StartMs: 3000, EndMs: 5000, Text: "second"},
		},
	}
	history := []model.AudioEnergyPoint{
		{Sequence: 1, TrackID: "track", PositionMs: 3500, RMS: 0.12, Peak: 0.18},
		{Sequence: 2, TrackID: "track", PositionMs: 3650, RMS: 0.70, Peak: 0.80},
	}

	suggestion, err := SuggestAlignment(doc, history, -1, 3600, "track")
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.LineIndex != 1 {
		t.Fatalf("expected current line to be selected, got %#v", suggestion)
	}
}

func TestSuggestAlignmentRequiresAudioHistory(t *testing.T) {
	doc := model.LyricDocument{
		ID:      "lyric",
		TrackID: "track",
		Lines: []model.LyricLine{
			{StartMs: 1000, EndMs: 2000, Text: "first"},
		},
	}

	if _, err := SuggestAlignment(doc, nil, 0, 1200, "track"); err == nil {
		t.Fatal("expected missing audio history error")
	}
}
