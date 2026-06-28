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
	SourceCustom  = model.LyricSourceCustom
)

var LyricSources = []string{SourceTTMLDB, SourceQQ, SourceKugou, SourceNetease, SourceCustom}

var PlatformLyricSources = []string{SourceQQ, SourceKugou, SourceNetease}

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
	FixedLyricSource   string
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
	DelayMs            int64
	Available          bool
	SoftwareCleaned    bool
	AICleaned          bool
	HasTranslation     bool
	HasTransliteration bool
	UpdatedAt          time.Time
}

type Input struct {
	Title            string
	Artist           string
	Album            string
	DurationMs       int64
	CoverHash        string
	FixedLyricSource string
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
	in = in.cleanBase()
	in = normalizeCombinedMetadata(in, primaryMetadataSeparators)
	return in
}

func (in Input) cleanBase() Input {
	in.Title = strings.TrimSpace(in.Title)
	in.Artist = strings.TrimSpace(in.Artist)
	in.Album = strings.TrimSpace(in.Album)
	in.CoverHash = strings.TrimSpace(in.CoverHash)
	in.FixedLyricSource = normalizeFixedLyricSource(in.FixedLyricSource)
	if in.DurationMs < 0 {
		in.DurationMs = 0
	}
	return in
}

func (in Input) Recordable() bool {
	in = in.Clean()
	if in.Title == "" || in.Artist == "" {
		return false
	}
	title := strings.ToLower(in.Title)
	if isPlaceholderTitle(title) {
		return false
	}
	artist := strings.ToLower(in.Artist)
	return !isPlaceholderTitle(artist)
}

func UniqueKey(in Input) string {
	in = in.Clean()
	return uniqueKeyFromFields(in.Title, in.Artist)
}

func uniqueKeyWithoutMetadataSplit(in Input) string {
	in = in.cleanBase()
	return uniqueKeyFromFields(in.Title, in.Artist)
}

func uniqueKeyFromFields(titleValue string, artistValue string) string {
	title := normalizeTitle(titleValue)
	artist := normalizeArtist(artistValue)
	if title == "" {
		title = "unknown-title"
	}
	if artist == "" {
		artist = "unknown-artist"
	}
	return title + "|" + artist
}

type inputKeyCandidate struct {
	input Input
	key   string
}

func inputKeyCandidates(in Input) []inputKeyCandidate {
	base := in.cleanBase()
	primary := normalizeCombinedMetadata(base, primaryMetadataSeparators)
	fallback := normalizeCombinedMetadata(base, append(primaryMetadataSeparators, fallbackMetadataSeparators...))

	var candidates []inputKeyCandidate
	seen := map[string]struct{}{}
	add := func(item Input, key string) {
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, inputKeyCandidate{input: item.Clean(), key: key})
	}

	add(primary, UniqueKey(primary))
	add(fallback, UniqueKey(fallback))
	add(primary, uniqueKeyWithoutMetadataSplit(base))
	for _, item := range legacyCombinedMetadataCandidates(base) {
		add(primary, uniqueKeyWithoutMetadataSplit(item))
	}
	return candidates
}

func legacyCombinedMetadataCandidates(in Input) []Input {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Artist) == "" || strings.TrimSpace(in.Album) == "" {
		return nil
	}
	combined := []string{
		in.Artist + " — " + in.Album,
		in.Artist + " - " + in.Album,
	}
	result := make([]Input, 0, len(combined))
	for _, artist := range combined {
		item := in.cleanBase()
		item.Artist = artist
		item.Album = ""
		result = append(result, item)
	}
	return result
}

func normalizeFixedLyricSource(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, source := range LyricSources {
		if value == source {
			return value
		}
	}
	return ""
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

var (
	primaryMetadataSeparators  = []string{" — ", "—", " ― ", "―", " – ", "–", " | ", "｜", " / ", "／"}
	fallbackMetadataSeparators = []string{" - "}
)

func normalizeCombinedMetadata(in Input, separators []string) Input {
	if in.Artist != "" {
		if left, right, ok := splitCombinedField(in.Artist, separators); ok {
			in.Artist = left
			if in.Album == "" {
				in.Album = right
			}
		}
	}
	if in.Artist == "" && in.Album != "" {
		if left, right, ok := splitCombinedField(in.Album, separators); ok {
			in.Artist = left
			in.Album = right
		}
	}
	if in.Artist == "" && in.Title != "" {
		if left, right, ok := splitCombinedField(in.Title, separators); ok && looksLikeArtist(left) && looksLikeSongTitle(right) {
			in.Artist = left
			in.Title = right
		}
	}
	return in
}

func splitCombinedField(value string, separators []string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	for _, sep := range separators {
		index := strings.Index(value, sep)
		if index <= 0 {
			continue
		}
		left := strings.TrimSpace(value[:index])
		right := strings.TrimSpace(value[index+len(sep):])
		if !looksLikeArtist(left) || !looksLikeSongTitle(right) {
			continue
		}
		return left, right, true
	}
	return "", "", false
}

func looksLikeArtist(value string) bool {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	return len(runes) >= 1 && len(runes) <= 80 && !isPlaceholderTitle(strings.ToLower(value))
}

func looksLikeSongTitle(value string) bool {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	return len(runes) >= 1 && len(runes) <= 120 && !isPlaceholderTitle(strings.ToLower(value))
}

func isPlaceholderTitle(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	known := map[string]struct{}{
		"waiting for playback":              {},
		"waiting for selected smtc session": {},
		"no media":                          {},
		"no media playing":                  {},
		"nothing playing":                   {},
		"not playing":                       {},
		"unknown":                           {},
		"unknown title":                     {},
		"loading":                           {},
		"advertisement":                     {},
		"ad":                                {},
		"等待播放":                              {},
		"暂无播放":                              {},
		"没有正在播放的媒体":                         {},
		"未知":                                {},
		"未知歌曲":                              {},
		"加载中":                               {},
	}
	if _, ok := known[value]; ok {
		return true
	}
	return strings.Contains(value, "waiting for") ||
		strings.Contains(value, "nothing is playing") ||
		strings.Contains(value, "no song") ||
		strings.Contains(value, "no track")
}
