package lyric

import (
	"math"
	"strings"

	amllttml "github.com/xiaowumin-mark/amll-ttml"
)

func ParseTTML(input string) (Document, error) {
	parsed, err := amllttml.ParseLyric(input)
	if err != nil {
		return Document{}, err
	}
	return documentFromAMLLTTML(parsed), nil
}

func GenerateTTML(doc Document, pretty bool) string {
	return amllttml.ExportTTMLText(amllTTMLFromDocument(doc), pretty)
}

func CompressTTML(input string) (string, error) {
	parsed, err := amllttml.ParseLyric(input)
	if err != nil {
		return "", err
	}
	return amllttml.ExportTTMLText(amllTTMLFromDocument(documentFromAMLLTTML(parsed)), false), nil
}

func documentFromAMLLTTML(input amllttml.TTMLLyric) Document {
	doc := Document{
		Metadata: make([]Metadata, 0, len(input.Metadata)),
		Lines:    make([]Line, 0, len(input.LyricLines)),
	}
	for _, meta := range input.Metadata {
		key := strings.TrimSpace(meta.Key)
		if key == "" {
			continue
		}
		values := make([]string, 0, len(meta.Value))
		for _, value := range meta.Value {
			value = strings.TrimSpace(value)
			if value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			doc.Metadata = append(doc.Metadata, Metadata{Key: key, Values: values})
		}
	}
	for _, line := range input.LyricLines {
		out := Line{
			StartTimeMs:     safeMillis(line.StartTime),
			EndTimeMs:       safeMillis(line.EndTime),
			TranslatedLyric: cleanAuxiliaryText(line.TranslatedLyric),
			RomanLyric:      strings.TrimSpace(line.RomanLyric),
			IsBackground:    line.IsBG,
			IsDuet:          line.IsDuet,
			IgnoreSync:      line.IgnoreSync,
			Words:           make([]Word, 0, len(line.Words)),
		}
		for _, word := range line.Words {
			if isFormattingWhitespace(word.Word) {
				continue
			}
			out.Words = append(out.Words, Word{
				StartTimeMs: safeMillis(word.StartTime),
				EndTimeMs:   safeMillis(word.EndTime),
				Text:        word.Word,
				RomanText:   strings.TrimSpace(word.RomanWord),
				Obscene:     word.Obscene,
				EmptyBeat:   word.EmptyBeat,
			})
		}
		doc.Lines = append(doc.Lines, out)
	}
	return doc.Normalized()
}

func amllTTMLFromDocument(doc Document) amllttml.TTMLLyric {
	doc = doc.Normalized()
	out := amllttml.TTMLLyric{
		Metadata:   make([]amllttml.TTMLMetadata, 0, len(doc.Metadata)),
		LyricLines: make([]amllttml.LyricLine, 0, len(doc.Lines)),
	}
	for _, meta := range doc.Metadata {
		key := strings.TrimSpace(meta.Key)
		if key == "" {
			continue
		}
		values := make([]string, 0, len(meta.Values))
		for _, value := range meta.Values {
			value = strings.TrimSpace(value)
			if value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			out.Metadata = append(out.Metadata, amllttml.TTMLMetadata{Key: key, Value: values})
		}
	}
	for _, line := range doc.Lines {
		outLine := amllttml.NewLyricLine()
		outLine.StartTime = float64(maxInt64(line.StartTimeMs, 0))
		outLine.EndTime = float64(maxInt64(line.EndTimeMs, line.StartTimeMs))
		outLine.TranslatedLyric = cleanAuxiliaryText(line.TranslatedLyric)
		outLine.RomanLyric = strings.TrimSpace(line.RomanLyric)
		outLine.IsBG = line.IsBackground
		outLine.IsDuet = line.IsDuet
		outLine.IgnoreSync = line.IgnoreSync
		outLine.Words = make([]amllttml.LyricWord, 0, len(line.Words))
		for _, word := range line.Words {
			outWord := amllttml.NewLyricWord()
			outWord.StartTime = float64(maxInt64(word.StartTimeMs, 0))
			outWord.EndTime = float64(maxInt64(word.EndTimeMs, word.StartTimeMs))
			outWord.Word = word.Text
			outWord.RomanWord = strings.TrimSpace(word.RomanText)
			outWord.Obscene = word.Obscene
			outWord.EmptyBeat = word.EmptyBeat
			outLine.Words = append(outLine.Words, outWord)
		}
		if len(outLine.Words) == 0 {
			outLine.Words = append(outLine.Words, amllttml.LyricWord{
				StartTime: outLine.StartTime,
				EndTime:   outLine.EndTime,
				Word:      line.Text(),
			})
		}
		out.LyricLines = append(out.LyricLines, outLine)
	}
	return out
}

func safeMillis(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return int64(math.Round(value))
}

func isFormattingWhitespace(value string) bool {
	return strings.TrimSpace(value) == "" && strings.ContainsAny(value, "\r\n\t")
}
