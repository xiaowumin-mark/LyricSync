package lyric

import (
	"testing"

	musicapi "github.com/xiaowumin-mark/AMLX-MUSIC-API"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestDocumentFromAMLXLyricMergesAuxiliaryLines(t *testing.T) {
	doc := documentFromAMLXLyric(&musicapi.Lyric{
		Raw: "[00:01.00]hello",
		Lines: []musicapi.LyricLine{{
			Time:     1000,
			Duration: 2000,
			Text:     "hello",
			Syllables: []musicapi.LyricSyllable{{
				Time:     1000,
				Duration: 900,
				Text:     "hel",
			}, {
				Time:     1900,
				Duration: 1100,
				Text:     "lo",
			}},
		}},
		Translation:  []musicapi.LyricLine{{Time: 1000, Text: "//"}},
		Romanization: []musicapi.LyricLine{{Time: 1000, Text: "ha lo"}},
	}, nil)

	if !IsUsable(doc) {
		t.Fatalf("expected AMLX lyric to be usable: %#v", doc)
	}
	if len(doc.Lines) != 1 {
		t.Fatalf("expected one line, got %#v", doc.Lines)
	}
	if doc.Lines[0].TranslatedLyric != "" {
		t.Fatalf("expected QQ // translation to be empty, got %q", doc.Lines[0].TranslatedLyric)
	}
	if doc.Lines[0].RomanLyric != "ha lo" {
		t.Fatalf("expected romanization to merge, got %q", doc.Lines[0].RomanLyric)
	}
	if !doc.HasWordTimeline() {
		t.Fatalf("expected word timeline from AMLX syllables")
	}
}

func TestSelectBestResultFollowsPriority(t *testing.T) {
	doc := Document{Lines: []Line{{
		StartTimeMs: 1000,
		EndTimeMs:   2000,
		Words:       []Word{{StartTimeMs: 1000, EndTimeMs: 2000, Text: "hello"}},
	}}}.Normalized()
	ttml := GenerateTTML(doc, false)
	results := []ProviderResult{
		{Source: model.LyricSourceQQ, TTMLLyric: ttml, Document: doc, Score: 100},
		{Source: model.LyricSourceTTMLDB, TTMLLyric: ttml, Document: doc, Score: 50},
	}

	selected, ok := SelectBestResult(results, []string{model.LyricSourceTTMLDB, model.LyricSourceQQ})
	if !ok {
		t.Fatal("expected selectable result")
	}
	if selected.Source != model.LyricSourceTTMLDB {
		t.Fatalf("expected priority to win over score, got %s", selected.Source)
	}
}

func TestParseTTMLDBIndexMetadata(t *testing.T) {
	content := `{"metadata":[["musicName",["明明 (Live)"]],["artists",["李宇春","丁肆Dicey"]],["album",["有歌2024"]],["ncmMusicId",["2642164541"]],["qqMusicId",["000pF84f1Mqkf7"]]],"rawLyricFile":"1746678978875-108002475-0a0fb081.ttml"}`
	entries, err := parseTTMLDBIndex(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %#v", entries)
	}
	entry := entries[0]
	if entry.Metadata.Titles[0] != "明明 (Live)" || entry.Metadata.Artists[0] != "丁肆Dicey" {
		t.Fatalf("unexpected metadata: %#v", entry.Metadata)
	}
	if entry.Timestamp != 1746678978875 {
		t.Fatalf("unexpected timestamp: %d", entry.Timestamp)
	}
	if score := entry.matchScore(TrackQuery{Title: "明明", Artist: "李宇春"}); score < 30 {
		t.Fatalf("expected matching score, got %d", score)
	}
}
