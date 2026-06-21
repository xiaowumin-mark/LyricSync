package lyric

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lyricsync/pkg/model"
)

func TestLocalProviderSearchesLRCFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Artist - Song.lrc")
	if err := os.WriteFile(path, []byte("[00:00.00]hello\n[00:03.00]world\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	provider := NewLocalProvider([]string{dir})
	doc, err := provider.Search(context.Background(), model.Track{
		ID:     "track",
		Title:  "Song",
		Artist: "Artist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != "local:Artist - Song.lrc" {
		t.Fatalf("unexpected source: %q", doc.Source)
	}
	if len(doc.Lines) != 2 || doc.Lines[0].Text != "hello" {
		t.Fatalf("unexpected lines: %#v", doc.Lines)
	}
	if doc.TTML == "" || doc.AMLXBase64 == "" {
		t.Fatalf("expected TTML and AMLX output")
	}
}
