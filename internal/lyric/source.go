package lyric

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type TrackQuery struct {
	Title      string
	Artist     string
	Album      string
	DurationMs int64
}

type Provider interface {
	Source() string
	Search(ctx context.Context, query TrackQuery) (ProviderResult, error)
}

type ProviderResult struct {
	Source        string
	SourceTrackID string
	Title         string
	Artist        string
	Album         string
	DurationMs    int64
	RawLyric      string
	TTMLLyric     string
	Document      Document
	Score         int
}

type ProviderError struct {
	Source string
	Err    error
}

type SearchAllResult struct {
	Results          []ProviderResult
	Errors           []ProviderError
	TTMLDBUpdatedAt  time.Time
	TTMLDBEntryCount int
}

type TimedTextLine struct {
	StartTimeMs int64
	DurationMs  int64
	Text        string
	Syllables   []TimedSyllable
}

type TimedSyllable struct {
	StartTimeMs int64
	DurationMs  int64
	Text        string
}

func (q TrackQuery) Clean() TrackQuery {
	q.Title = strings.TrimSpace(q.Title)
	q.Artist = strings.TrimSpace(q.Artist)
	q.Album = strings.TrimSpace(q.Album)
	q.DurationMs = NormalizeDurationMs(q.DurationMs)
	return q
}

func (q TrackQuery) Keyword() string {
	q = q.Clean()
	if q.Artist == "" {
		return q.Title
	}
	return strings.TrimSpace(q.Title + " " + q.Artist)
}

func NormalizeDurationMs(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value < 1000 {
		return value * 1000
	}
	return value
}

func NewProviderResult(source, trackID, rawLyric string, doc Document) (ProviderResult, error) {
	source = strings.TrimSpace(source)
	trackID = strings.TrimSpace(trackID)
	rawLyric = strings.TrimSpace(rawLyric)
	doc = doc.Normalized()
	if !IsUsable(doc) {
		return ProviderResult{}, fmt.Errorf("%s lyric is not usable", source)
	}
	return ProviderResult{
		Source:        source,
		SourceTrackID: trackID,
		RawLyric:      rawLyric,
		TTMLLyric:     GenerateTTML(doc, false),
		Document:      doc,
	}, nil
}

func DocumentFromTimedLines(lines []TimedTextLine, translations []TimedTextLine, romanizations []TimedTextLine, metadata []Metadata) Document {
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].StartTimeMs < lines[j].StartTimeMs
	})
	doc := Document{Metadata: metadata, Lines: make([]Line, 0, len(lines))}
	for i, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" || text == "//" {
			continue
		}
		start := maxInt64(line.StartTimeMs, 0)
		end := start + maxInt64(line.DurationMs, 0)
		if end <= start {
			if i+1 < len(lines) && lines[i+1].StartTimeMs > start {
				end = lines[i+1].StartTimeMs
			} else {
				end = start + 5000
			}
		}
		out := Line{
			StartTimeMs:     start,
			EndTimeMs:       end,
			TranslatedLyric: matchingAuxText(translations, start),
			RomanLyric:      matchingAuxText(romanizations, start),
		}
		if len(line.Syllables) > 0 {
			for _, syllable := range line.Syllables {
				wordText := syllable.Text
				if wordText == "" {
					continue
				}
				wordStart := maxInt64(syllable.StartTimeMs, start)
				wordEnd := wordStart + maxInt64(syllable.DurationMs, 0)
				if wordEnd <= wordStart {
					wordEnd = end
				}
				out.Words = append(out.Words, Word{
					StartTimeMs: wordStart,
					EndTimeMs:   wordEnd,
					Text:        wordText,
				})
			}
		}
		if len(out.Words) == 0 {
			out.Words = []Word{{StartTimeMs: start, EndTimeMs: end, Text: text}}
		}
		doc.Lines = append(doc.Lines, out)
	}
	return doc.Normalized()
}

func ParseSyncedLRC(mainText string, translationText string, romanText string, metadata []Metadata) (Document, error) {
	mainDoc, err := ParseLRC(mainText)
	if err != nil {
		return Document{}, err
	}
	transDoc := parseOptionalLRC(translationText)
	romanDoc := parseOptionalLRC(romanText)
	lines := timedLinesFromDocument(mainDoc)
	translations := timedLinesFromDocument(transDoc)
	romanizations := timedLinesFromDocument(romanDoc)
	return DocumentFromTimedLines(lines, translations, romanizations, metadata), nil
}

func SelectBestResult(results []ProviderResult, priority []string) (ProviderResult, bool) {
	bySource := map[string]ProviderResult{}
	for _, result := range results {
		if result.Source == "" || strings.TrimSpace(result.TTMLLyric) == "" || !IsUsable(result.Document) {
			continue
		}
		if current, ok := bySource[result.Source]; !ok || result.Score > current.Score {
			bySource[result.Source] = result
		}
	}
	for _, source := range priority {
		if result, ok := bySource[source]; ok {
			return result, true
		}
	}
	keys := make([]string, 0, len(bySource))
	for source := range bySource {
		keys = append(keys, source)
	}
	sort.Strings(keys)
	for _, source := range keys {
		return bySource[source], true
	}
	return ProviderResult{}, false
}

func MatchScore(query TrackQuery, title string, artist string, album string, durationMs int64) int {
	query = query.Clean()
	titleScore := textMatchScore(query.Title, title)
	if titleScore == 0 {
		return 0
	}
	score := titleScore
	if query.Artist != "" {
		score += textMatchScore(query.Artist, artist) / 2
	}
	if query.Album != "" {
		score += textMatchScore(query.Album, album) / 4
	}
	durationMs = NormalizeDurationMs(durationMs)
	if query.DurationMs > 0 && durationMs > 0 {
		diff := math.Abs(float64(query.DurationMs - durationMs))
		switch {
		case diff <= 2000:
			score += 12
		case diff <= 5000:
			score += 8
		case diff <= 12000:
			score += 4
		}
	}
	return score
}

func ProviderRawText(mainText string, translationText string, romanText string) string {
	parts := []string{}
	if strings.TrimSpace(mainText) != "" {
		parts = append(parts, strings.TrimSpace(mainText))
	}
	if strings.TrimSpace(translationText) != "" {
		parts = append(parts, "[translation]\n"+strings.TrimSpace(translationText))
	}
	if strings.TrimSpace(romanText) != "" {
		parts = append(parts, "[romanization]\n"+strings.TrimSpace(romanText))
	}
	return strings.Join(parts, "\n\n")
}

func parseOptionalLRC(input string) Document {
	input = strings.TrimSpace(input)
	if input == "" {
		return Document{}
	}
	doc, err := ParseLRC(input)
	if err != nil {
		return Document{}
	}
	return doc
}

func timedLinesFromDocument(doc Document) []TimedTextLine {
	doc = doc.Normalized()
	lines := make([]TimedTextLine, 0, len(doc.Lines))
	for _, line := range doc.Lines {
		text := strings.TrimSpace(line.Text())
		if text == "" {
			continue
		}
		lines = append(lines, TimedTextLine{
			StartTimeMs: line.StartTimeMs,
			DurationMs:  maxInt64(line.EndTimeMs-line.StartTimeMs, 0),
			Text:        text,
		})
	}
	return lines
}

func matchingAuxText(lines []TimedTextLine, start int64) string {
	best := ""
	bestDiff := int64(250)
	for _, line := range lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		diff := absInt64(line.StartTimeMs - start)
		if diff <= bestDiff {
			best = line.Text
			bestDiff = diff
		}
	}
	return cleanAuxiliaryText(best)
}

func textMatchScore(query string, candidate string) int {
	query = normalizeMatchText(query)
	candidate = normalizeMatchText(candidate)
	if query == "" || candidate == "" {
		return 0
	}
	if query == candidate {
		return 60
	}
	if strings.Contains(candidate, query) || strings.Contains(query, candidate) {
		return 44
	}
	queryTokens := splitMatchTokens(query)
	candidateTokens := splitMatchTokens(candidate)
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}
	matches := 0
	for _, q := range queryTokens {
		for _, c := range candidateTokens {
			if q == c || strings.Contains(c, q) || strings.Contains(q, c) {
				matches++
				break
			}
		}
	}
	return int(float64(matches) / float64(len(queryTokens)) * 34)
}

func normalizeMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("feat.", " ", "ft.", " ", "featuring", " ", "&", " ", "/", " ", "\\", " ", "-", " ")
	value = replacer.Replace(value)
	var b strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) {
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

var matchTokenPattern = regexp.MustCompile(`\s+`)

func splitMatchTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := matchTokenPattern.Split(value, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func metadataForResult(title string, artist string, album string, source string, sourceID string) []Metadata {
	metadata := []Metadata{}
	add := func(key string, values ...string) {
		cleaned := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				cleaned = append(cleaned, value)
			}
		}
		if len(cleaned) > 0 {
			metadata = append(metadata, Metadata{Key: key, Values: cleaned})
		}
	}
	add("musicName", title)
	add("artists", splitArtists(artist)...)
	add("album", album)
	switch source {
	case "ttml-db":
		add("ttmlDbRawLyricFile", sourceID)
	case "qq":
		add("qqMusicId", sourceID)
	case "netease":
		add("ncmMusicId", sourceID)
	case "kugou":
		add("kugouMusicId", sourceID)
	}
	return metadata
}

func splitArtists(value string) []string {
	replacer := strings.NewReplacer(" feat. ", ",", " ft. ", ",", " featuring ", ",", " & ", ",", " / ", ",", "/", ",", ";", ",")
	value = replacer.Replace(value)
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
