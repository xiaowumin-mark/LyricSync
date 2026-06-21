package lyric

import (
	"encoding/base64"
	"strings"
	"testing"

	musicapi "github.com/xiaowumin-mark/AMLX-MUSIC-API"
	ttml "github.com/xiaowumin-mark/amll-ttml"

	"lyricsync/pkg/model"
)

func TestFromMusicAPILyricExportsTTMLAndAMLX(t *testing.T) {
	track := model.Track{
		ID:       "track-1",
		Title:    "Song",
		Artist:   "Artist",
		Album:    "Album",
		Duration: 12000,
	}
	apiLyric := &musicapi.Lyric{
		Lines: []musicapi.LyricLine{
			{Time: 0, Duration: 3000, Text: "hello"},
			{Time: 3000, Duration: 3000, Text: "world"},
		},
		Translation: []musicapi.LyricLine{
			{Time: 0, Text: "你好"},
			{Time: 3000, Text: "世界"},
		},
	}

	doc, err := FromMusicAPILyric(track, "test", &musicapi.Song{ID: "provider-song"}, apiLyric)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "ttml" {
		t.Fatalf("expected ttml format, got %q", doc.Format)
	}
	if doc.ProviderTrackID != "provider-song" {
		t.Fatalf("expected provider track id, got %q", doc.ProviderTrackID)
	}
	if len(doc.Lines) != 2 || doc.Lines[0].Text != "hello" || doc.Lines[0].Translation != "你好" {
		t.Fatalf("unexpected model lines: %#v", doc.Lines)
	}
	if !strings.Contains(doc.TTML, "hello") {
		t.Fatalf("expected TTML output to contain lyric text: %s", doc.TTML)
	}

	binaryData, err := base64.StdEncoding.DecodeString(doc.AMLXBase64)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ttml.DecodeBinary(binaryData)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.LyricLines) != 2 {
		t.Fatalf("expected 2 decoded lines, got %d", len(decoded.LyricLines))
	}
}

func TestCacheRoundTrip(t *testing.T) {
	cache, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	track := model.Track{Title: "Song", Artist: "Artist", Album: "Album"}
	key := CacheKey(track)
	doc := model.LyricDocument{
		ID:          "lyric",
		TrackID:     "track",
		Source:      "test",
		Format:      "ttml",
		LastUpdated: model.Now(),
		Lines: []model.LyricLine{
			{StartMs: 0, EndMs: 1000, Text: "line"},
		},
	}
	if err := cache.Save(key, doc); err != nil {
		t.Fatal(err)
	}
	got, ok, err := cache.Load(key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Lines[0].Text != "line" {
		t.Fatalf("unexpected cached document: %#v", got)
	}
}
