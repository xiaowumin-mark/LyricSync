package lyric

import (
	"encoding/base64"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	ttml "github.com/xiaowumin-mark/amll-ttml"

	"lyricsync/pkg/model"
)

func ImportFile(track model.Track, path string) (model.LyricDocument, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ttml" && ext != ".lrc" {
		return model.LyricDocument{}, fmt.Errorf("lyric: unsupported import file %s", filepath.Base(path))
	}
	return loadLocalLyric(track, path)
}

func OffsetDocument(doc model.LyricDocument, offsetMs int64) (model.LyricDocument, error) {
	if offsetMs == 0 {
		return doc, nil
	}
	if strings.TrimSpace(doc.TTML) != "" {
		parsed, err := ttml.ParseLyric(doc.TTML)
		if err == nil {
			shiftTTML(&parsed, offsetMs)
			rawTTML := ttml.ExportTTMLText(parsed, false)
			next, err := FromTTMLText(model.Track{ID: doc.TrackID}, doc.Source, rawTTML)
			if err == nil {
				next.ID = doc.ID
				next.ProviderTrackID = doc.ProviderTrackID
				next.Language = doc.Language
				next.Source = doc.Source
				next.LastUpdated = model.Now()
				return next, nil
			}
		}
	}

	next := doc
	next.Lines = shiftLines(doc.Lines, offsetMs)
	return RebuildDocument(next)
}

func AlignDocument(doc model.LyricDocument, lineIndex int, positionMs int64) (model.LyricDocument, int64, error) {
	if lineIndex < 0 || lineIndex >= len(doc.Lines) {
		return model.LyricDocument{}, 0, fmt.Errorf("lyric: line index %d out of range", lineIndex)
	}
	if positionMs < 0 {
		positionMs = 0
	}
	offsetMs := positionMs - doc.Lines[lineIndex].StartMs
	next, err := OffsetDocument(doc, offsetMs)
	if err != nil {
		return model.LyricDocument{}, 0, err
	}
	return next, offsetMs, nil
}

func RebuildDocument(doc model.LyricDocument) (model.LyricDocument, error) {
	if len(doc.Lines) == 0 {
		doc.TTML = ""
		doc.AMLXBase64 = ""
		doc.LastUpdated = model.Now()
		return doc, nil
	}

	lyric := ttml.TTMLLyric{
		Metadata: []ttml.TTMLMetadata{
			{Key: "source", Value: []string{"LyricSync"}},
		},
		LyricLines: make([]ttml.LyricLine, 0, len(doc.Lines)),
	}
	for _, modelLine := range doc.Lines {
		text := strings.TrimSpace(modelLine.Text)
		if text == "" {
			continue
		}
		start := float64(maxInt64(modelLine.StartMs, 0))
		end := float64(maxInt64(modelLine.EndMs, modelLine.StartMs+1))
		line := ttml.NewLyricLine()
		line.StartTime = start
		line.EndTime = end
		line.TranslatedLyric = strings.TrimSpace(modelLine.Translation)
		line.RomanLyric = strings.TrimSpace(modelLine.Romanization)
		word := ttml.NewLyricWord()
		word.StartTime = start
		word.EndTime = end
		word.Word = text
		line.Words = []ttml.LyricWord{word}
		lyric.LyricLines = append(lyric.LyricLines, line)
	}

	rawTTML := ttml.ExportTTMLText(lyric, false)
	binaryData, err := ttml.EncodeBinary(lyric)
	if err != nil {
		return model.LyricDocument{}, fmt.Errorf("lyric: encode AMLX binary: %w", err)
	}

	doc.Format = "ttml"
	doc.Translated = documentHasTranslations(doc)
	doc.TTML = rawTTML
	doc.AMLXBase64 = base64.StdEncoding.EncodeToString(binaryData)
	doc.LastUpdated = model.Now()
	return doc, nil
}

func shiftTTML(lyric *ttml.TTMLLyric, offsetMs int64) {
	offset := float64(offsetMs)
	for lineIndex := range lyric.LyricLines {
		line := &lyric.LyricLines[lineIndex]
		line.StartTime = clampMsFloat(line.StartTime + offset)
		line.EndTime = clampMsFloat(line.EndTime + offset)
		if line.EndTime <= line.StartTime {
			line.EndTime = line.StartTime + 1
		}
		for wordIndex := range line.Words {
			word := &line.Words[wordIndex]
			word.StartTime = clampMsFloat(word.StartTime + offset)
			word.EndTime = clampMsFloat(word.EndTime + offset)
			if word.EndTime <= word.StartTime {
				word.EndTime = word.StartTime + 1
			}
		}
	}
}

func shiftLines(lines []model.LyricLine, offsetMs int64) []model.LyricLine {
	next := make([]model.LyricLine, 0, len(lines))
	for _, line := range lines {
		line.StartMs = maxInt64(line.StartMs+offsetMs, 0)
		line.EndMs = maxInt64(line.EndMs+offsetMs, line.StartMs+1)
		next = append(next, line)
	}
	return next
}

func clampMsFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return math.Round(value)
}

func documentHasTranslations(doc model.LyricDocument) bool {
	for _, line := range doc.Lines {
		if strings.TrimSpace(line.Translation) != "" {
			return true
		}
	}
	return false
}
