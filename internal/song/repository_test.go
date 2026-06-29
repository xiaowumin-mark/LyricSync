package song

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/lyric"
	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestUniqueKeyUsesStableTitleAndArtist(t *testing.T) {
	a := UniqueKey(Input{
		Title:      "Song Title (Remastered)",
		Artist:     "Artist feat. Guest",
		Album:      "Album",
		DurationMs: 183400,
	})
	b := UniqueKey(Input{
		Title:      " song   title ",
		Artist:     "artist, guest",
		Album:      "Other",
		DurationMs: 0,
	})
	if a != b {
		t.Fatalf("expected matching unique keys, got %q and %q", a, b)
	}
}

func TestRecordPlaybackDeduplicatesSongs(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	first, created, err := repo.RecordPlayback(ctx, InputFromTrack(model.Track{
		Title:     "Song",
		Artist:    "Artist",
		Album:     "Album",
		Duration:  180000,
		CoverHash: "cover-a",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !created || first.PlayCount != 1 {
		t.Fatalf("expected first playback to create count=1, got created=%v song=%#v", created, first)
	}

	second, created, err := repo.RecordPlayback(ctx, InputFromTrack(model.Track{
		Title:     "Song",
		Artist:    "Artist",
		Album:     "Album",
		Duration:  181900,
		CoverHash: "cover-b",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected duplicate playback to update existing row")
	}
	if second.ID != first.ID || second.PlayCount != 2 {
		t.Fatalf("expected same song with play_count=2, got %#v", second)
	}

	all, err := repo.Search(ctx, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one stored song, got %d", len(all))
	}
}

func TestRecordPlaybackDeduplicatesWhenDurationArrivesLater(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	first, created, err := repo.RecordPlayback(ctx, Input{
		Title:  "Saikai",
		Artist: "Mili",
		Album:  "Saikai - Single",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first playback to create a song")
	}

	second, created, err := repo.RecordPlayback(ctx, Input{
		Title:      "Saikai",
		Artist:     "Mili",
		Album:      "Saikai - Single",
		DurationMs: 323000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created || second.ID != first.ID {
		t.Fatalf("expected same row after duration update, created=%v first=%#v second=%#v", created, first, second)
	}
	if second.PlayCount != 2 || second.DurationMs != 323000 {
		t.Fatalf("expected play count and duration update, got %#v", second)
	}
}

func TestRecordPlaybackDeduplicatesArtistAlbumCombinedMetadata(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	first, created, err := repo.RecordPlayback(ctx, Input{
		Title:  "Song",
		Artist: "Artist — Album",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first playback to create a song")
	}
	if first.Artist != "Artist" || first.Album != "Album" {
		t.Fatalf("expected combined artist field to be split before storing, got %#v", first)
	}

	second, created, err := repo.RecordPlayback(ctx, Input{
		Title:  "Song",
		Artist: "Artist",
		Album:  "Album",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created || second.ID != first.ID || second.PlayCount != 2 {
		t.Fatalf("expected normalized playback to hit same row, created=%v first=%#v second=%#v", created, first, second)
	}

	all, err := repo.Search(ctx, "Song", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one stored song, got %#v", all)
	}
}

func TestGetByInputMatchesLegacyCombinedArtistKey(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()
	now := formatDBTime(time.Now().UTC())

	_, err := repo.db.ExecContext(ctx, `
		INSERT INTO songs (
			unique_key, title, artist, album, duration_ms, first_played_at,
			last_played_at, play_count, created_at, updated_at
		) VALUES (?, 'Song', 'Artist — Album', '', 0, ?, ?, 1, ?, ?)
	`, uniqueKeyWithoutMetadataSplit(Input{Title: "Song", Artist: "Artist — Album"}), now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByInput(ctx, Input{Title: "Song", Artist: "Artist", Album: "Album"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Song" {
		t.Fatalf("expected legacy row, got %#v", got)
	}
}

func TestRecordPlaybackRejectsPlaceholderTrack(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	if _, _, err := repo.RecordPlayback(ctx, Input{Title: "Waiting for playback"}); err != ErrUnrecordable {
		t.Fatalf("expected placeholder title to be unrecordable, got %v", err)
	}
	if _, _, err := repo.RecordPlayback(ctx, Input{Title: "Song Without Artist"}); err != ErrUnrecordable {
		t.Fatalf("expected missing artist to be unrecordable, got %v", err)
	}
}

func TestInputCleanSplitsCombinedMetadata(t *testing.T) {
	got := Input{Title: "Artist — Song"}.Clean()
	if got.Artist != "Artist" || got.Title != "Song" {
		t.Fatalf("expected artist-title split, got %#v", got)
	}
	got = Input{Title: "Song", Artist: "Artist — Album"}.Clean()
	if got.Artist != "Artist" || got.Album != "Album" {
		t.Fatalf("expected artist-album split, got %#v", got)
	}
	got = Input{Title: "Song", Artist: "Artist - Album"}.Clean()
	if got.Artist != "Artist - Album" || got.Album != "" {
		t.Fatalf("plain hyphen should not split during primary clean, got %#v", got)
	}
}

func TestNormalizeSongKeysMergesLegacyDurationDuplicates(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()
	now := formatDBTime(time.Now().UTC())

	_, err := repo.db.ExecContext(ctx, `
		INSERT INTO songs (
			unique_key, title, artist, album, duration_ms, first_played_at,
			last_played_at, play_count, created_at, updated_at
		) VALUES
			('saikai|mili|d:0', 'Saikai', 'Mili', 'Saikai - Single', 0, ?, ?, 1, ?, ?),
			('saikai|mili|d:325000', 'Saikai', 'Mili', 'Saikai - Single', 323000, ?, ?, 2, ?, ?)
	`, now, now, now, now, now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.normalizeSongKeys(ctx); err != nil {
		t.Fatal(err)
	}
	all, err := repo.Search(ctx, "Saikai", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected duplicates to merge, got %#v", all)
	}
	if all[0].PlayCount != 3 || all[0].DurationMs != 323000 {
		t.Fatalf("expected merged stats and duration, got %#v", all[0])
	}
}

func TestNormalizeSongKeysMergesCombinedArtistDuplicates(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()
	now := formatDBTime(time.Now().UTC())

	_, err := repo.db.ExecContext(ctx, `
		INSERT INTO songs (
			unique_key, title, artist, album, duration_ms, first_played_at,
			last_played_at, play_count, created_at, updated_at
		) VALUES
			(?, 'Song', 'Artist — Album', '', 0, ?, ?, 1, ?, ?),
			(?, 'Song', 'Artist', 'Album', 200000, ?, ?, 2, ?, ?)
	`, uniqueKeyWithoutMetadataSplit(Input{Title: "Song", Artist: "Artist — Album"}), now, now, now, now,
		UniqueKey(Input{Title: "Song", Artist: "Artist", Album: "Album"}), now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.normalizeSongKeys(ctx); err != nil {
		t.Fatal(err)
	}
	all, err := repo.Search(ctx, "Song", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected duplicates to merge, got %#v", all)
	}
	if all[0].Artist != "Artist" || all[0].Album != "Album" || all[0].PlayCount != 3 {
		t.Fatalf("expected merged normalized metadata, got %#v", all[0])
	}
}

func TestSongCRUDSearchAndDelete(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, Input{Title: "晴天", Artist: "周杰伦", Album: "叶惠美", DurationMs: 269000})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.PlayCount != 0 {
		t.Fatalf("unexpected created song: %#v", created)
	}

	updated, err := repo.Update(ctx, created.ID, Input{Title: "晴天", Artist: "周杰伦", Album: "叶惠美", DurationMs: 270000})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DurationMs != 270000 {
		t.Fatalf("expected updated duration, got %#v", updated)
	}

	found, err := repo.Search(ctx, "周杰伦", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != created.ID {
		t.Fatalf("expected search to find created song, got %#v", found)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, created.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestLyricsSlotsAndUpdate(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	s, err := repo.Create(ctx, Input{Title: "Song", Artist: "Artist"})
	if err != nil {
		t.Fatal(err)
	}
	lyrics, err := repo.Lyrics(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lyrics) != len(LyricSources) {
		t.Fatalf("expected lyric slots for all sources, got %#v", lyrics)
	}

	if err := repo.SetLyric(ctx, s.ID, SourceQQ, "[00:01.00]hello", ""); err != nil {
		t.Fatal(err)
	}
	lyrics, err = repo.Lyrics(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	var qq LyricSource
	for _, lyric := range lyrics {
		if lyric.Source == SourceQQ {
			qq = lyric
		}
	}
	if !qq.Available || !strings.Contains(qq.RawLyric, "hello") {
		t.Fatalf("expected QQ lyric to be available, got %#v", qq)
	}
	if strings.TrimSpace(qq.TTMLLyric) == "" {
		t.Fatalf("expected normalized TTML to be generated, got %#v", qq)
	}
	if _, err := lyric.ParseTTML(qq.TTMLLyric); err != nil {
		t.Fatalf("generated TTML should parse: %v", err)
	}
}

func TestSetLyricPreservesAICleanedTTMLFromRawRefresh(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	s, err := repo.Create(ctx, Input{Title: "Song", Artist: "Artist"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetLyric(ctx, s.ID, SourceQQ, "[00:01.00]raw line", ""); err != nil {
		t.Fatal(err)
	}
	aiTTML := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="00:00:01.000" end="00:00:03.000">ai line</p></div></body></tt>`
	if err := repo.SetLyricWithFlags(ctx, s.ID, SourceQQ, "track-1", "[00:01.00]raw line", aiTTML, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetLyric(ctx, s.ID, SourceQQ, "[00:01.00]raw refreshed", ""); err != nil {
		t.Fatal(err)
	}
	lyrics, err := repo.Lyrics(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range lyrics {
		if item.Source != SourceQQ {
			continue
		}
		if !item.AICleaned {
			t.Fatalf("expected AI flag to remain set, got %#v", item)
		}
		if !strings.Contains(item.RawLyric, "raw refreshed") {
			t.Fatalf("expected raw lyric refresh, got %#v", item.RawLyric)
		}
		if !strings.Contains(item.TTMLLyric, "ai line") {
			t.Fatalf("expected AI TTML to be preserved, got %s", item.TTMLLyric)
		}
		return
	}
	t.Fatal("QQ lyric source not found")
}

func TestSetLyricDelay(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	s, err := repo.Create(ctx, Input{Title: "Song", Artist: "Artist"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetLyricDelay(ctx, s.ID, SourceCustom, -240); err != nil {
		t.Fatal(err)
	}
	lyrics, err := repo.Lyrics(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range lyrics {
		if item.Source == SourceCustom {
			if item.DelayMs != -240 {
				t.Fatalf("delay = %d, want -240", item.DelayMs)
			}
			return
		}
	}
	t.Fatal("custom lyric slot not found")
}

func TestFixedLyricSourceAndGetByInput(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, Input{Title: "Song", Artist: "Artist", FixedLyricSource: SourceCustom})
	if err != nil {
		t.Fatal(err)
	}
	if created.FixedLyricSource != SourceCustom {
		t.Fatalf("expected fixed source to persist, got %#v", created)
	}
	got, err := repo.GetByInput(ctx, Input{Title: "Song", Artist: "Artist"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected GetByInput to find created song, got %#v", got)
	}
}

func openTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := Open(filepath.Join(t.TempDir(), "lyricsync-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}
