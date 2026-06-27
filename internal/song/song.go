package song

import (
	"strings"
	"time"
	"unicode"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

const (
	SourceTTMLDB  = model.LyricSourceTTMLDB
	SourceQQ      = model.LyricSourceQQ
	SourceKugou   = model.LyricSourceKugou
	SourceNetease = model.LyricSourceNetease
)

var LyricSources = []string{SourceTTMLDB, SourceQQ, SourceKugou, SourceNetease}

type Song struct {
	ID                 int64
	UniqueKey          string
	Title              string
	Artist             string
	Album              string
	DurationMs         int64
	CoverHash          string
	FirstPlayedAt      time.Time
	LastPlayedAt       time.Time
	PlayCount          int
	AppliedLyricSource string
	AppliedLyricID     int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type LyricSource struct {
	ID                 int64
	SongID             int64
	Source             string
	SourceTrackID      string
	RawLyric           string
	TTMLLyric          string
	Available          bool
	SoftwareCleaned    bool
	AICleaned          bool
	HasTranslation     bool
	HasTransliteration bool
	UpdatedAt          time.Time
}

type Input struct {
	Title      string
	Artist     string
	Album      string
	DurationMs int64
	CoverHash  string
}

func InputFromTrack(track model.Track) Input {
	return Input{
		Title:      track.Title,
		Artist:     track.Artist,
		Album:      track.Album,
		DurationMs: track.Duration,
		CoverHash:  track.CoverHash,
	}
}

func (in Input) Clean() Input {
	in.Title = strings.TrimSpace(in.Title)
	in.Artist = strings.TrimSpace(in.Artist)
	in.Album = strings.TrimSpace(in.Album)
	in.CoverHash = strings.TrimSpace(in.CoverHash)
	if in.DurationMs < 0 {
		in.DurationMs = 0
	}
	return in
}

func (in Input) Recordable() bool {
	in = in.Clean()
	if in.Title == "" {
		return false
	}
	title := strings.ToLower(in.Title)
	return title != "waiting for playback" && title != "waiting for selected smtc session"
}

func UniqueKey(in Input) string {
	in = in.Clean()
	title := normalizeTitle(in.Title)
	artist := normalizeArtist(in.Artist)
	if title == "" {
		title = "unknown-title"
	}
	if artist == "" {
		artist = "unknown-artist"
	}
	return title + "|" + artist
}

func normalizeTitle(value string) string {
	value = normalizeText(value)
	for {
		next := stripVersionSuffix(value)
		if next == value {
			return value
		}
		value = next
	}
}

func normalizeArtist(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		" feat. ", ",",
		" ft. ", ",",
		" featuring ", ",",
		" and ", ",",
		" & ", ",",
		" x ", ",",
		"、", ",",
		"，", ",",
		";", ",",
		"；", ",",
		"/", ",",
		"\\", ",",
	)
	value = replacer.Replace(value)
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = normalizeText(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return strings.Join(out, ",")
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func stripVersionSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	pairs := [][2]string{{"(", ")"}, {"[", "]"}, {"（", "）"}, {"【", "】"}}
	for _, pair := range pairs {
		if !strings.HasSuffix(value, pair[1]) {
			continue
		}
		start := strings.LastIndex(value, pair[0])
		if start < 0 {
			continue
		}
		inside := strings.TrimSpace(value[start+len(pair[0]) : len(value)-len(pair[1])])
		if isVersionTag(inside) {
			return strings.TrimSpace(value[:start])
		}
	}
	return value
}

func isVersionTag(value string) bool {
	value = strings.ToLower(value)
	keywords := []string{
		"version", "ver.", "remaster", "remastered", "remix", "mix", "edit",
		"live", "cover", "instrumental", "伴奏", "纯音乐", "现场", "重制", "混音",
	}
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}
