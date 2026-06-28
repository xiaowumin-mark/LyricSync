package lyric

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const ttmlDBIndexFileName = "raw-lyrics-index.jsonl"

type ttmlDBProvider struct {
	client   *http.Client
	cacheDir string
	cfg      model.TTMLDBConfig

	entries       []ttmlDBEntry
	lastUpdatedAt time.Time
	entryCount    int
}

type ttmlDBEntry struct {
	Metadata     ttmlDBMetadata `json:"metadata"`
	RawLyricFile string         `json:"rawLyricFile"`
	Timestamp    int64          `json:"-"`
}

type ttmlDBMetadata struct {
	Titles   []string
	Artists  []string
	Albums   []string
	NCMIDs   []string
	QQIDs    []string
	AppleIDs []string
	Spotify  []string
}

func newTTMLDBProvider(client *http.Client, cacheDir string, cfg model.TTMLDBConfig) *ttmlDBProvider {
	if cfg.IndexURL == "" {
		cfg.IndexURL = "https://amlldb.bikonoo.com/metadata/raw-lyrics-index.jsonl"
	}
	if cfg.UpdateIntervalHours <= 0 {
		cfg.UpdateIntervalHours = 24
	}
	return &ttmlDBProvider{
		client:   client,
		cacheDir: strings.TrimSpace(cacheDir),
		cfg:      cfg,
	}
}

func (p *ttmlDBProvider) Source() string {
	return model.LyricSourceTTMLDB
}

func (p *ttmlDBProvider) Search(ctx context.Context, query TrackQuery) (ProviderResult, error) {
	if err := p.ensureIndex(ctx, false); err != nil {
		return ProviderResult{}, err
	}
	if len(p.entries) == 0 {
		return ProviderResult{}, fmt.Errorf("ttml-db index is empty")
	}
	best, score, ok := p.bestEntry(query)
	if !ok {
		return ProviderResult{}, fmt.Errorf("ttml-db lyric not found")
	}
	rawURL := p.rawLyricsURL(best.RawLyricFile)
	ttmlText, err := getText(ctx, p.client, rawURL, nil, nil)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("ttml-db lyric download: %w", err)
	}
	doc, err := ParseTTML(ttmlText)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("ttml-db parse: %w", err)
	}
	result, err := NewProviderResult(model.LyricSourceTTMLDB, best.RawLyricFile, ttmlText, doc)
	if err != nil {
		return ProviderResult{}, err
	}
	result.Title = firstString(best.Metadata.Titles)
	result.Artist = strings.Join(best.Metadata.Artists, ", ")
	result.Album = firstString(best.Metadata.Albums)
	result.Score = score
	return result, nil
}

func (p *ttmlDBProvider) UpdateIndex(ctx context.Context) (time.Time, int, error) {
	if err := p.ensureIndex(ctx, true); err != nil {
		return time.Time{}, 0, err
	}
	return p.lastUpdatedAt, p.entryCount, nil
}

func (p *ttmlDBProvider) LastUpdatedAt() time.Time {
	return p.lastUpdatedAt
}

func (p *ttmlDBProvider) EntryCount() int {
	return p.entryCount
}

func (p *ttmlDBProvider) ensureIndex(ctx context.Context, force bool) error {
	if p.cacheDir == "" {
		return fmt.Errorf("ttml-db cache dir is empty")
	}
	path := p.indexPath()
	info, statErr := os.Stat(path)
	hasCache := statErr == nil && !info.IsDir()
	shouldDownload := force || !hasCache
	if !shouldDownload && p.cfg.AutoUpdateIndex {
		last := p.lastConfiguredUpdate()
		if last.IsZero() && hasCache {
			last = info.ModTime()
		}
		interval := time.Duration(p.cfg.UpdateIntervalHours) * time.Hour
		shouldDownload = interval > 0 && time.Since(last) >= interval
	}
	if shouldDownload {
		if err := p.downloadIndex(ctx, path); err != nil {
			if !hasCache {
				return err
			}
		}
	}
	entries, err := loadTTMLDBIndex(path)
	if err != nil {
		return err
	}
	p.entries = entries
	p.entryCount = len(entries)
	if p.lastUpdatedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			p.lastUpdatedAt = info.ModTime()
		}
	}
	return nil
}

func (p *ttmlDBProvider) downloadIndex(ctx context.Context, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text, err := getText(ctx, p.client, p.cfg.IndexURL, nil, nil)
	if err != nil {
		return fmt.Errorf("ttml-db index download: %w", err)
	}
	entries, err := parseTTMLDBIndex(text)
	if err != nil {
		return err
	}
	if len(entries) == 0 && strings.TrimSpace(text) != "" {
		return fmt.Errorf("ttml-db index has no readable entries")
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return err
	}
	p.lastUpdatedAt = time.Now().UTC()
	p.entryCount = len(entries)
	return nil
}

func (p *ttmlDBProvider) bestEntry(query TrackQuery) (ttmlDBEntry, int, bool) {
	query = query.Clean()
	type scored struct {
		entry ttmlDBEntry
		score int
	}
	bestByKey := map[string]scored{}
	for i := len(p.entries) - 1; i >= 0; i-- {
		entry := p.entries[i]
		score := entry.matchScore(query)
		if score < 74 {
			continue
		}
		key := entry.dedupKey()
		current, ok := bestByKey[key]
		if !ok || score > current.score || (score == current.score && entry.Timestamp > current.entry.Timestamp) {
			bestByKey[key] = scored{entry: entry, score: score}
		}
	}
	if len(bestByKey) == 0 {
		return ttmlDBEntry{}, 0, false
	}
	items := make([]scored, 0, len(bestByKey))
	for _, item := range bestByKey {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].entry.Timestamp > items[j].entry.Timestamp
		}
		return items[i].score > items[j].score
	})
	return items[0].entry, items[0].score, true
}

func (p *ttmlDBProvider) indexPath() string {
	return filepath.Join(p.cacheDir, ttmlDBIndexFileName)
}

func (p *ttmlDBProvider) lastConfiguredUpdate() time.Time {
	value := strings.TrimSpace(p.cfg.LastUpdatedAt)
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func (p *ttmlDBProvider) rawLyricsURL(file string) string {
	file = strings.TrimSpace(file)
	indexURL := strings.TrimSpace(p.cfg.IndexURL)
	if base, ok := strings.CutSuffix(indexURL, "/metadata/raw-lyrics-index.jsonl"); ok {
		return base + "/raw-lyrics/" + url.PathEscape(file)
	}
	parsed, err := url.Parse(indexURL)
	if err != nil {
		return "https://amlldb.bikonoo.com/raw-lyrics/" + url.PathEscape(file)
	}
	dir := strings.TrimRight(filepath.ToSlash(filepath.Dir(parsed.Path)), "/")
	if strings.HasSuffix(dir, "/metadata") {
		dir = strings.TrimSuffix(dir, "/metadata")
	}
	parsed.Path = dir + "/raw-lyrics/" + file
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func loadTTMLDBIndex(path string) ([]ttmlDBEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseTTMLDBIndex(string(data))
}

func parseTTMLDBIndex(content string) ([]ttmlDBEntry, error) {
	var entries []ttmlDBEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ttmlDBEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entry.RawLyricFile = strings.TrimSpace(entry.RawLyricFile)
		if entry.RawLyricFile == "" || len(entry.Metadata.Titles) == 0 {
			continue
		}
		entry.Timestamp = rawLyricTimestamp(entry.RawLyricFile)
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (m *ttmlDBMetadata) UnmarshalJSON(data []byte) error {
	var pairs [][]json.RawMessage
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}
	for _, pair := range pairs {
		if len(pair) != 2 {
			continue
		}
		var key string
		var values []string
		if err := json.Unmarshal(pair[0], &key); err != nil {
			continue
		}
		if err := json.Unmarshal(pair[1], &values); err != nil {
			continue
		}
		values = cleanStringSlice(values)
		switch key {
		case "musicName":
			m.Titles = values
		case "artists":
			m.Artists = values
		case "album":
			m.Albums = values
		case "ncmMusicId":
			m.NCMIDs = values
		case "qqMusicId":
			m.QQIDs = values
		case "appleMusicId":
			m.AppleIDs = values
		case "spotifyId":
			m.Spotify = values
		}
	}
	return nil
}

func (e ttmlDBEntry) matchScore(query TrackQuery) int {
	best := 0
	artist := strings.Join(e.Metadata.Artists, ", ")
	albums := e.Metadata.Albums
	if len(albums) == 0 {
		albums = []string{""}
	}
	for _, title := range e.Metadata.Titles {
		for _, album := range albums {
			score := MatchScore(query, title, artist, album, 0)
			if score > best {
				best = score
			}
		}
	}
	return best
}

func (e ttmlDBEntry) dedupKey() string {
	for _, values := range [][]string{e.Metadata.NCMIDs, e.Metadata.QQIDs, e.Metadata.AppleIDs, e.Metadata.Spotify} {
		if len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0])
		}
	}
	return e.RawLyricFile
}

func rawLyricTimestamp(file string) int64 {
	prefix, _, _ := strings.Cut(file, "-")
	value, _ := strconv.ParseInt(prefix, 10, 64)
	return value
}

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func firstString(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
