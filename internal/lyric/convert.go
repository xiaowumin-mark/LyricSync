package lyric

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"

	musicapi "github.com/xiaowumin-mark/AMLX-MUSIC-API"
	ttml "github.com/xiaowumin-mark/amll-ttml"

	"lyricsync/pkg/model"
)

func FromTTMLText(track model.Track, source string, rawTTML string) (model.LyricDocument, error) {
	parsed, err := ttml.ParseLyric(rawTTML)
	if err != nil {
		return model.LyricDocument{}, err
	}
	binaryData, err := ttml.EncodeBinary(parsed)
	if err != nil {
		return model.LyricDocument{}, fmt.Errorf("lyric: encode AMLX binary: %w", err)
	}
	return model.LyricDocument{
		ID:          "lyric-" + CacheKey(track),
		TrackID:     track.ID,
		Source:      source,
		Format:      "ttml",
		Language:    "und",
		Translated:  hasTranslations(parsed),
		Lines:       toModelLines(parsed),
		TTML:        rawTTML,
		AMLXBase64:  base64.StdEncoding.EncodeToString(binaryData),
		LastUpdated: model.Now(),
	}, nil
}

func FromLRCText(track model.Track, source string, rawLRC string) (model.LyricDocument, error) {
	lines := musicapi.LRCParse(rawLRC)
	if len(lines) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no timed LRC lines")
	}
	apiLyric := &musicapi.Lyric{
		Raw:   rawLRC,
		Lines: lines,
	}
	return FromMusicAPILyric(track, source, nil, apiLyric)
}

func FromMusicAPILyric(track model.Track, providerName string, song *musicapi.Song, apiLyric *musicapi.Lyric) (model.LyricDocument, error) {
	if apiLyric == nil || len(apiLyric.Lines) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: empty lyric from %s", providerName)
	}

	ttmlLyric := toTTMLLyric(apiLyric)
	if len(ttmlLyric.LyricLines) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no timed lyric lines from %s", providerName)
	}

	rawTTML := ttml.ExportTTMLText(ttmlLyric, false)
	doc, err := FromTTMLText(track, providerName, rawTTML)
	if err != nil {
		return model.LyricDocument{}, fmt.Errorf("lyric: exported TTML validation failed: %w", err)
	}

	if song != nil {
		doc.ProviderTrackID = song.ID
	}
	return doc, nil
}

func DemoDocument() model.LyricDocument {
	apiLyric := &musicapi.Lyric{
		Lines: []musicapi.LyricLine{
			{Time: 0, Duration: 6000, Text: "LyricSync is watching local playback"},
			{Time: 6000, Duration: 6000, Text: "Lyrics are normalized through amll-ttml"},
			{Time: 12000, Duration: 6000, Text: "WebSocket syncs lyrics, progress and audio features"},
			{Time: 18000, Duration: 6000, Text: "External services can render dynamic backgrounds from audio data"},
		},
		Translation: []musicapi.LyricLine{
			{Time: 0, Text: "LyricSync monitors the active media session"},
			{Time: 6000, Text: "The internal lyric model uses AMLL TTML"},
			{Time: 12000, Text: "Realtime clients receive synchronized state"},
			{Time: 18000, Text: "Audio features are available for dynamic backgrounds"},
		},
	}
	doc, err := FromMusicAPILyric(model.Track{ID: "demo-track", Title: "LyricSync Demo", Artist: "Local Session"}, "local-demo", nil, apiLyric)
	if err != nil {
		return model.LyricDocument{
			ID:          "demo-lyric",
			TrackID:     "demo-track",
			Source:      "local-demo",
			Format:      "ttml",
			Language:    "und",
			LastUpdated: model.Now(),
		}
	}
	return doc
}

func toTTMLLyric(apiLyric *musicapi.Lyric) ttml.TTMLLyric {
	lines := append([]musicapi.LyricLine(nil), apiLyric.Lines...)
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})

	var lyricLines []ttml.LyricLine
	for i, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}

		start := line.Time
		end := line.Time + line.Duration
		if end <= start {
			end = inferLineEnd(lines, i)
		}

		ttmlLine := ttml.NewLyricLine()
		ttmlLine.StartTime = float64(maxInt64(start, 0))
		ttmlLine.EndTime = float64(maxInt64(end, start+1))
		ttmlLine.TranslatedLyric = textAt(apiLyric.Translation, line.Time)
		ttmlLine.RomanLyric = textAt(apiLyric.Romanization, line.Time)
		ttmlLine.Words = wordsForLine(line, ttmlLine.StartTime, ttmlLine.EndTime)
		lyricLines = append(lyricLines, ttmlLine)
	}

	return ttml.TTMLLyric{
		Metadata: []ttml.TTMLMetadata{
			{Key: "source", Value: []string{"LyricSync"}},
		},
		LyricLines: lyricLines,
	}
}

func wordsForLine(line musicapi.LyricLine, start, end float64) []ttml.LyricWord {
	if len(line.Syllables) == 0 {
		word := ttml.NewLyricWord()
		word.StartTime = start
		word.EndTime = end
		word.Word = line.Text
		return []ttml.LyricWord{word}
	}

	words := make([]ttml.LyricWord, 0, len(line.Syllables))
	for _, syllable := range line.Syllables {
		if strings.TrimSpace(syllable.Text) == "" {
			continue
		}
		wordStart := maxInt64(syllable.Time, int64(start))
		wordEnd := syllable.Time + syllable.Duration
		if wordEnd <= wordStart {
			wordEnd = int64(end)
		}
		word := ttml.NewLyricWord()
		word.StartTime = float64(wordStart)
		word.EndTime = float64(maxInt64(wordEnd, wordStart+1))
		word.Word = syllable.Text
		words = append(words, word)
	}
	if len(words) == 0 {
		word := ttml.NewLyricWord()
		word.StartTime = start
		word.EndTime = end
		word.Word = line.Text
		return []ttml.LyricWord{word}
	}
	return words
}

func inferLineEnd(lines []musicapi.LyricLine, index int) int64 {
	current := lines[index].Time
	for i := index + 1; i < len(lines); i++ {
		if lines[i].Time > current {
			return lines[i].Time
		}
	}
	return current + 5000
}

func textAt(lines []musicapi.LyricLine, ms int64) string {
	for _, line := range lines {
		if absInt64(line.Time-ms) <= 350 {
			return strings.TrimSpace(line.Text)
		}
	}
	return ""
}

func toModelLines(lyric ttml.TTMLLyric) []model.LyricLine {
	lines := make([]model.LyricLine, 0, len(lyric.LyricLines))
	for _, line := range lyric.LyricLines {
		text := strings.TrimSpace(lineText(line))
		if text == "" {
			continue
		}
		lines = append(lines, model.LyricLine{
			StartMs:      int64(math.Round(line.StartTime)),
			EndMs:        int64(math.Round(line.EndTime)),
			Text:         text,
			Translation:  line.TranslatedLyric,
			Romanization: line.RomanLyric,
		})
	}
	return lines
}

func lineText(line ttml.LyricLine) string {
	var builder strings.Builder
	for _, word := range line.Words {
		builder.WriteString(word.Word)
	}
	return builder.String()
}

func hasTranslations(lyric ttml.TTMLLyric) bool {
	for _, line := range lyric.LyricLines {
		if strings.TrimSpace(line.TranslatedLyric) != "" {
			return true
		}
	}
	return false
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
