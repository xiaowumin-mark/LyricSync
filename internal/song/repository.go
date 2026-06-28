package song

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/lyric"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound      = errors.New("song: not found")
	ErrInvalidInput  = errors.New("song: invalid input")
	ErrUnrecordable  = errors.New("song: track is not recordable")
	ErrDuplicateSong = errors.New("song: duplicate song")
)

type Repository struct {
	db *sql.DB
}

func Open(path string) (*Repository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("song database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	repo := &Repository{db: db}
	if err := repo.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) RecordPlayback(ctx context.Context, input Input) (Song, bool, error) {
	if r == nil || r.db == nil {
		return Song{}, false, fmt.Errorf("song database is not available")
	}
	input = input.Clean()
	if !input.Recordable() {
		return Song{}, false, ErrUnrecordable
	}
	key := UniqueKey(input)
	candidates := inputKeyCandidates(input)
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Song{}, false, err
	}
	defer rollback(tx)

	id, err := songIDByCandidatesTx(ctx, tx, candidates)
	created := false
	if errors.Is(err, ErrNotFound) {
		result, execErr := tx.ExecContext(ctx, `
			INSERT INTO songs (
				unique_key, title, artist, album, duration_ms, cover_hash,
				first_played_at, last_played_at, play_count, fixed_lyric_source, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)
		`, key, input.Title, input.Artist, input.Album, input.DurationMs, input.CoverHash, formatDBTime(now), formatDBTime(now), input.FixedLyricSource, formatDBTime(now), formatDBTime(now))
		if execErr != nil {
			return Song{}, false, execErr
		}
		id, err = result.LastInsertId()
		if err != nil {
			return Song{}, false, err
		}
		created = true
	} else if err != nil {
		return Song{}, false, err
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE songs
			SET unique_key = ?, title = ?, artist = ?, album = ?, duration_ms = ?, cover_hash = ?,
				last_played_at = ?, play_count = play_count + 1, updated_at = ?
			WHERE id = ?
		`, key, input.Title, input.Artist, input.Album, input.DurationMs, input.CoverHash, formatDBTime(now), formatDBTime(now), id); err != nil {
			return Song{}, false, err
		}
	}
	if err := ensureLyricSlotsTx(ctx, tx, id); err != nil {
		return Song{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Song{}, false, err
	}
	got, err := r.Get(ctx, id)
	return got, created, err
}

func (r *Repository) Create(ctx context.Context, input Input) (Song, error) {
	if r == nil || r.db == nil {
		return Song{}, fmt.Errorf("song database is not available")
	}
	input = input.Clean()
	if input.Title == "" {
		return Song{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	key := UniqueKey(input)
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Song{}, err
	}
	defer rollback(tx)

	id, err := songIDByKeyTx(ctx, tx, key)
	if errors.Is(err, ErrNotFound) {
		result, execErr := tx.ExecContext(ctx, `
			INSERT INTO songs (
				unique_key, title, artist, album, duration_ms, cover_hash,
				first_played_at, last_played_at, play_count, fixed_lyric_source, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)
		`, key, input.Title, input.Artist, input.Album, input.DurationMs, input.CoverHash, formatDBTime(now), formatDBTime(now), input.FixedLyricSource, formatDBTime(now), formatDBTime(now))
		if execErr != nil {
			return Song{}, execErr
		}
		id, err = result.LastInsertId()
		if err != nil {
			return Song{}, err
		}
	} else if err != nil {
		return Song{}, err
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE songs
			SET title = ?, artist = ?, album = ?, duration_ms = ?, cover_hash = ?, fixed_lyric_source = ?, updated_at = ?
			WHERE id = ?
		`, input.Title, input.Artist, input.Album, input.DurationMs, input.CoverHash, input.FixedLyricSource, formatDBTime(now), id); err != nil {
			return Song{}, err
		}
	}
	if err := ensureLyricSlotsTx(ctx, tx, id); err != nil {
		return Song{}, err
	}
	if err := tx.Commit(); err != nil {
		return Song{}, err
	}
	return r.Get(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, input Input) (Song, error) {
	if r == nil || r.db == nil {
		return Song{}, fmt.Errorf("song database is not available")
	}
	if id <= 0 {
		return Song{}, ErrNotFound
	}
	input = input.Clean()
	if input.Title == "" {
		return Song{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	key := UniqueKey(input)
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE songs
		SET unique_key = ?, title = ?, artist = ?, album = ?, duration_ms = ?, cover_hash = ?, fixed_lyric_source = ?, updated_at = ?
		WHERE id = ?
	`, key, input.Title, input.Artist, input.Album, input.DurationMs, input.CoverHash, input.FixedLyricSource, formatDBTime(now), id)
	if err != nil {
		if isConstraintError(err) {
			return Song{}, ErrDuplicateSong
		}
		return Song{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Song{}, err
	}
	if affected == 0 {
		return Song{}, ErrNotFound
	}
	return r.Get(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("song database is not available")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM songs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id int64) (Song, error) {
	if r == nil || r.db == nil {
		return Song{}, fmt.Errorf("song database is not available")
	}
	row := r.db.QueryRowContext(ctx, selectSongSQL()+` WHERE id = ?`, id)
	return scanSong(row)
}

func (r *Repository) GetByInput(ctx context.Context, input Input) (Song, error) {
	if r == nil || r.db == nil {
		return Song{}, fmt.Errorf("song database is not available")
	}
	input = input.Clean()
	if input.Title == "" {
		return Song{}, ErrNotFound
	}
	for _, candidate := range inputKeyCandidates(input) {
		row := r.db.QueryRowContext(ctx, selectSongSQL()+` WHERE unique_key = ?`, candidate.key)
		got, err := scanSong(row)
		if err == nil {
			return got, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Song{}, err
		}
	}
	return Song{}, ErrNotFound
}

func (r *Repository) Recent(ctx context.Context, limit int) ([]Song, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, selectSongSQL()+` ORDER BY last_played_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSongs(rows)
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]Song, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("song database is not available")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query = strings.TrimSpace(query)
	if query == "" {
		rows, err := r.db.QueryContext(ctx, selectSongSQL()+` ORDER BY last_played_at DESC, id DESC LIMIT ?`, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanSongs(rows)
	}
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := r.db.QueryContext(ctx, selectSongSQL()+`
		WHERE lower(title) LIKE ? OR lower(artist) LIKE ? OR lower(album) LIKE ?
		ORDER BY last_played_at DESC, id DESC LIMIT ?
	`, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSongs(rows)
}

func (r *Repository) Lyrics(ctx context.Context, songID int64) ([]LyricSource, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("song database is not available")
	}
	if err := r.ensureLyricSlots(ctx, songID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, song_id, source, source_track_id, raw_lyric, ttml_lyric, delay_ms, available,
			software_cleaned, ai_cleaned, has_translation, has_transliteration, updated_at
		FROM lyric_sources
		WHERE song_id = ?
		ORDER BY CASE source
			WHEN 'ttml-db' THEN 0
			WHEN 'qq' THEN 1
			WHEN 'kugou' THEN 2
			WHEN 'netease' THEN 3
			WHEN 'custom' THEN 4
			ELSE 9
		END
	`, songID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []LyricSource
	for rows.Next() {
		lyric, err := scanLyric(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, lyric)
	}
	return result, rows.Err()
}

func (r *Repository) SetLyric(ctx context.Context, songID int64, source string, rawLyric string, ttmlLyric string) error {
	return r.SetLyricWithMeta(ctx, songID, source, "", rawLyric, ttmlLyric)
}

func (r *Repository) SetLyricWithMeta(ctx context.Context, songID int64, source string, sourceTrackID string, rawLyric string, ttmlLyric string) error {
	return r.SetLyricWithFlags(ctx, songID, source, sourceTrackID, rawLyric, ttmlLyric, false)
}

func (r *Repository) SetLyricWithFlags(ctx context.Context, songID int64, source string, sourceTrackID string, rawLyric string, ttmlLyric string, aiCleaned bool) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("song database is not available")
	}
	if !validSource(source) {
		return fmt.Errorf("%w: invalid lyric source %q", ErrInvalidInput, source)
	}
	sourceTrackID = strings.TrimSpace(sourceTrackID)
	if _, err := r.Get(ctx, songID); err != nil {
		return err
	}
	normalized, err := lyric.NormalizeContent(rawLyric, ttmlLyric)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO lyric_sources (
			song_id, source, source_track_id, raw_lyric, ttml_lyric, available,
			ai_cleaned, has_translation, has_transliteration, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(song_id, source) DO UPDATE SET
			source_track_id = CASE
				WHEN excluded.source_track_id != '' THEN excluded.source_track_id
				ELSE lyric_sources.source_track_id
			END,
			raw_lyric = excluded.raw_lyric,
			ttml_lyric = excluded.ttml_lyric,
			available = excluded.available,
			ai_cleaned = max(lyric_sources.ai_cleaned, excluded.ai_cleaned),
			has_translation = excluded.has_translation,
			has_transliteration = excluded.has_transliteration,
			updated_at = excluded.updated_at
	`, songID, source, sourceTrackID, rawLyric, normalized.TTML, boolInt(normalized.Available), boolInt(aiCleaned),
		boolInt(normalized.HasTranslation), boolInt(normalized.HasTransliteration), formatDBTime(now))
	return err
}

func (r *Repository) SetLyricDelay(ctx context.Context, songID int64, source string, delayMs int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("song database is not available")
	}
	if !validSource(source) {
		return fmt.Errorf("%w: invalid lyric source %q", ErrInvalidInput, source)
	}
	if _, err := r.Get(ctx, songID); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO lyric_sources (song_id, source, delay_ms, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(song_id, source) DO UPDATE SET
			delay_ms = excluded.delay_ms,
			updated_at = excluded.updated_at
	`, songID, source, delayMs, formatDBTime(now))
	return err
}

func (r *Repository) ApplyLyricSource(ctx context.Context, songID int64, source string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("song database is not available")
	}
	if !validSource(source) {
		return fmt.Errorf("%w: invalid lyric source %q", ErrInvalidInput, source)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE songs
		SET applied_lyric_source = ?,
			applied_lyric_id = (
				SELECT id FROM lyric_sources
				WHERE song_id = songs.id AND source = ? AND available = 1
				LIMIT 1
			),
			updated_at = ?
		WHERE id = ? AND EXISTS (
			SELECT 1 FROM lyric_sources
			WHERE song_id = songs.id AND source = ? AND available = 1
		)
	`, source, source, formatDBTime(time.Now().UTC()), songID, source)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ensureLyricSlots(ctx context.Context, songID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err := ensureLyricSlotsTx(ctx, tx, songID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS songs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			unique_key TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			artist TEXT NOT NULL DEFAULT '',
			album TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			cover_hash TEXT NOT NULL DEFAULT '',
			first_played_at TEXT NOT NULL,
			last_played_at TEXT NOT NULL,
			play_count INTEGER NOT NULL DEFAULT 0,
			fixed_lyric_source TEXT NOT NULL DEFAULT '',
			applied_lyric_source TEXT NOT NULL DEFAULT '',
			applied_lyric_id INTEGER,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_songs_last_played_at ON songs(last_played_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_songs_title ON songs(title)`,
		`CREATE TABLE IF NOT EXISTS lyric_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			song_id INTEGER NOT NULL,
			source TEXT NOT NULL,
			source_track_id TEXT NOT NULL DEFAULT '',
			raw_lyric TEXT NOT NULL DEFAULT '',
			ttml_lyric TEXT NOT NULL DEFAULT '',
			delay_ms INTEGER NOT NULL DEFAULT 0,
			available INTEGER NOT NULL DEFAULT 0,
			software_cleaned INTEGER NOT NULL DEFAULT 0,
			ai_cleaned INTEGER NOT NULL DEFAULT 0,
			has_translation INTEGER NOT NULL DEFAULT 0,
			has_transliteration INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE,
			UNIQUE(song_id, source)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lyric_sources_song_id ON lyric_sources(song_id)`,
		`PRAGMA user_version = 1`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := r.ensureSchema(ctx); err != nil {
		return err
	}
	if err := r.normalizeSongKeys(ctx); err != nil {
		return err
	}
	if err := r.ensureAllLyricSlots(ctx); err != nil {
		return err
	}
	return r.normalizeStoredLyrics(ctx)
}

func (r *Repository) ensureSchema(ctx context.Context) error {
	if err := r.ensureColumn(ctx, "songs", "fixed_lyric_source", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "lyric_sources", "delay_ms", `INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := r.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}

func (r *Repository) ensureAllLyricSlots(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM songs`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	for _, id := range ids {
		if err := ensureLyricSlotsTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) normalizeStoredLyrics(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, raw_lyric, ttml_lyric
		FROM lyric_sources
		WHERE raw_lyric != '' OR ttml_lyric != ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id   int64
		raw  string
		ttml string
	}
	var rowsToUpdate []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.raw, &item.ttml); err != nil {
			return err
		}
		rowsToUpdate = append(rowsToUpdate, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range rowsToUpdate {
		normalized, err := lyric.NormalizeContent(item.raw, item.ttml)
		if err != nil {
			continue
		}
		if _, err := r.db.ExecContext(ctx, `
			UPDATE lyric_sources
			SET ttml_lyric = ?, available = ?, has_translation = ?, has_transliteration = ?
			WHERE id = ?
		`, normalized.TTML, boolInt(normalized.Available), boolInt(normalized.HasTranslation), boolInt(normalized.HasTransliteration), item.id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) normalizeSongKeys(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, unique_key, title, artist, album, duration_ms, cover_hash,
			first_played_at, last_played_at, play_count, fixed_lyric_source
		FROM songs
		ORDER BY last_played_at DESC, id DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id            int64
		key           string
		title         string
		artist        string
		album         string
		durationMs    int64
		coverHash     string
		firstPlayed   time.Time
		lastPlayed    time.Time
		playCount     int
		fixedSource   string
		normalizedKey string
	}
	groups := map[string][]row{}
	for rows.Next() {
		var item row
		var firstPlayed, lastPlayed string
		if err := rows.Scan(
			&item.id, &item.key, &item.title, &item.artist, &item.album, &item.durationMs,
			&item.coverHash, &firstPlayed, &lastPlayed, &item.playCount, &item.fixedSource,
		); err != nil {
			return err
		}
		item.firstPlayed = parseDBTime(firstPlayed)
		item.lastPlayed = parseDBTime(lastPlayed)
		candidates := inputKeyCandidates(Input{
			Title:  item.title,
			Artist: item.artist,
			Album:  item.album,
		})
		if len(candidates) == 0 {
			item.normalizedKey = UniqueKey(Input{Title: item.title, Artist: item.artist})
		} else {
			item.normalizedKey = candidates[0].key
		}
		groups[item.normalizedKey] = append(groups[item.normalizedKey], item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	now := formatDBTime(time.Now().UTC())
	for key, items := range groups {
		if len(items) == 0 {
			continue
		}
		target := items[0]
		firstPlayed := target.firstPlayed
		lastPlayed := target.lastPlayed
		playCount := 0
		duration := target.durationMs
		coverHash := target.coverHash
		fixedSource := target.fixedSource
		normalizedInput := normalizedStoredInput(target.title, target.artist, target.album)

		for _, item := range items {
			if firstPlayed.IsZero() || (!item.firstPlayed.IsZero() && item.firstPlayed.Before(firstPlayed)) {
				firstPlayed = item.firstPlayed
			}
			if item.lastPlayed.After(lastPlayed) {
				lastPlayed = item.lastPlayed
			}
			playCount += item.playCount
			if duration <= 0 && item.durationMs > 0 {
				duration = item.durationMs
			}
			if strings.TrimSpace(coverHash) == "" && strings.TrimSpace(item.coverHash) != "" {
				coverHash = item.coverHash
			}
			if strings.TrimSpace(fixedSource) == "" && strings.TrimSpace(item.fixedSource) != "" {
				fixedSource = item.fixedSource
			}
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE songs
			SET unique_key = ?, title = ?, artist = ?, album = ?, duration_ms = ?, cover_hash = ?, first_played_at = ?,
				last_played_at = ?, play_count = ?, fixed_lyric_source = ?, updated_at = ?
			WHERE id = ?
		`, key, normalizedInput.Title, normalizedInput.Artist, normalizedInput.Album, duration, coverHash,
			formatDBTime(firstPlayed), formatDBTime(lastPlayed), playCount, fixedSource, now, target.id); err != nil {
			return err
		}

		for _, duplicate := range items[1:] {
			if err := mergeLyricSourcesTx(ctx, tx, target.id, duplicate.id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM songs WHERE id = ?`, duplicate.id); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func normalizedStoredInput(title string, artist string, album string) Input {
	candidates := inputKeyCandidates(Input{
		Title:  title,
		Artist: artist,
		Album:  album,
	})
	if len(candidates) > 0 {
		return candidates[0].input
	}
	return Input{Title: title, Artist: artist, Album: album}.Clean()
}

func mergeLyricSourcesTx(ctx context.Context, tx *sql.Tx, targetID, duplicateID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT source, source_track_id, raw_lyric, ttml_lyric, delay_ms, available,
			software_cleaned, ai_cleaned, has_translation, has_transliteration, updated_at
		FROM lyric_sources
		WHERE song_id = ?
	`, duplicateID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type lyricRow struct {
		source             string
		sourceTrackID      string
		rawLyric           string
		ttmlLyric          string
		delayMs            int64
		available          int
		softwareCleaned    int
		aiCleaned          int
		hasTranslation     int
		hasTransliteration int
		updatedAt          string
	}
	for rows.Next() {
		var item lyricRow
		if err := rows.Scan(
			&item.source, &item.sourceTrackID, &item.rawLyric, &item.ttmlLyric, &item.delayMs, &item.available,
			&item.softwareCleaned, &item.aiCleaned, &item.hasTranslation, &item.hasTransliteration, &item.updatedAt,
		); err != nil {
			return err
		}
		if strings.TrimSpace(item.rawLyric) == "" && strings.TrimSpace(item.ttmlLyric) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lyric_sources (
				song_id, source, source_track_id, raw_lyric, ttml_lyric, delay_ms, available,
				software_cleaned, ai_cleaned, has_translation, has_transliteration, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(song_id, source) DO UPDATE SET
				source_track_id = CASE
					WHEN lyric_sources.source_track_id = '' THEN excluded.source_track_id
					ELSE lyric_sources.source_track_id
				END,
				raw_lyric = CASE
					WHEN lyric_sources.raw_lyric = '' THEN excluded.raw_lyric
					ELSE lyric_sources.raw_lyric
				END,
				ttml_lyric = CASE
					WHEN lyric_sources.ttml_lyric = '' THEN excluded.ttml_lyric
					ELSE lyric_sources.ttml_lyric
				END,
				delay_ms = CASE
					WHEN lyric_sources.delay_ms = 0 THEN excluded.delay_ms
					ELSE lyric_sources.delay_ms
				END,
				available = max(lyric_sources.available, excluded.available),
				software_cleaned = max(lyric_sources.software_cleaned, excluded.software_cleaned),
				ai_cleaned = max(lyric_sources.ai_cleaned, excluded.ai_cleaned),
				has_translation = max(lyric_sources.has_translation, excluded.has_translation),
				has_transliteration = max(lyric_sources.has_transliteration, excluded.has_transliteration),
				updated_at = CASE
					WHEN lyric_sources.updated_at > excluded.updated_at THEN lyric_sources.updated_at
					ELSE excluded.updated_at
				END
		`, targetID, item.source, item.sourceTrackID, item.rawLyric, item.ttmlLyric, item.delayMs, item.available,
			item.softwareCleaned, item.aiCleaned, item.hasTranslation, item.hasTransliteration, item.updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func ensureLyricSlotsTx(ctx context.Context, tx *sql.Tx, songID int64) error {
	now := formatDBTime(time.Now().UTC())
	for _, source := range LyricSources {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO lyric_sources (song_id, source, updated_at)
			VALUES (?, ?, ?)
		`, songID, source, now); err != nil {
			return err
		}
	}
	return nil
}

func songIDByKeyTx(ctx context.Context, tx *sql.Tx, key string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM songs WHERE unique_key = ?`, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func songIDByCandidatesTx(ctx context.Context, tx *sql.Tx, candidates []inputKeyCandidate) (int64, error) {
	for _, candidate := range candidates {
		id, err := songIDByKeyTx(ctx, tx, candidate.key)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return 0, err
		}
	}
	return 0, ErrNotFound
}

func selectSongSQL() string {
	return `SELECT id, unique_key, title, artist, album, duration_ms, cover_hash,
		first_played_at, last_played_at, play_count, fixed_lyric_source, applied_lyric_source,
		applied_lyric_id, created_at, updated_at FROM songs`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSong(row rowScanner) (Song, error) {
	var s Song
	var firstPlayed, lastPlayed, created, updated string
	var appliedID sql.NullInt64
	err := row.Scan(
		&s.ID, &s.UniqueKey, &s.Title, &s.Artist, &s.Album, &s.DurationMs, &s.CoverHash,
		&firstPlayed, &lastPlayed, &s.PlayCount, &s.FixedLyricSource, &s.AppliedLyricSource,
		&appliedID, &created, &updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Song{}, ErrNotFound
	}
	if err != nil {
		return Song{}, err
	}
	if appliedID.Valid {
		s.AppliedLyricID = appliedID.Int64
	}
	s.FirstPlayedAt = parseDBTime(firstPlayed)
	s.LastPlayedAt = parseDBTime(lastPlayed)
	s.CreatedAt = parseDBTime(created)
	s.UpdatedAt = parseDBTime(updated)
	return s, nil
}

func scanSongs(rows *sql.Rows) ([]Song, error) {
	var result []Song
	for rows.Next() {
		s, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func scanLyric(rows *sql.Rows) (LyricSource, error) {
	var l LyricSource
	var updated string
	var available, softwareCleaned, aiCleaned, hasTranslation, hasTransliteration int
	err := rows.Scan(
		&l.ID, &l.SongID, &l.Source, &l.SourceTrackID, &l.RawLyric, &l.TTMLLyric, &l.DelayMs, &available,
		&softwareCleaned, &aiCleaned, &hasTranslation, &hasTransliteration, &updated,
	)
	if err != nil {
		return LyricSource{}, err
	}
	l.Available = available != 0
	l.SoftwareCleaned = softwareCleaned != 0
	l.AICleaned = aiCleaned != 0
	l.HasTranslation = hasTranslation != 0
	l.HasTransliteration = hasTransliteration != 0
	l.UpdatedAt = parseDBTime(updated)
	return l, nil
}

func validSource(source string) bool {
	for _, allowed := range LyricSources {
		if source == allowed {
			return true
		}
	}
	return false
}

func formatDBTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseDBTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func rollback(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "constraint") || strings.Contains(text, "unique")
}
