package lyric

import (
	"sort"
	"strings"
)

type Metadata struct {
	Key    string
	Values []string
}

type Document struct {
	Metadata []Metadata
	Lines    []Line
}

type Line struct {
	StartTimeMs     int64
	EndTimeMs       int64
	Words           []Word
	TranslatedLyric string
	RomanLyric      string
	IsBackground    bool
	IsDuet          bool
	IgnoreSync      bool
}

type Word struct {
	StartTimeMs int64
	EndTimeMs   int64
	Text        string
	RomanText   string
	Obscene     bool
	EmptyBeat   float64
}

type PreviewLine struct {
	StartTimeMs int64
	EndTimeMs   int64
	Text        string
	Translation string
	Roman       string
}

func (d Document) Normalized() Document {
	out := Document{
		Metadata: cloneMetadata(d.Metadata),
		Lines:    make([]Line, 0, len(d.Lines)),
	}
	for _, line := range d.Lines {
		line = line.normalized()
		if line.hasAnyContent() {
			out.Lines = append(out.Lines, line)
		}
	}
	sort.SliceStable(out.Lines, func(i, j int) bool {
		if out.Lines[i].StartTimeMs == out.Lines[j].StartTimeMs {
			return out.Lines[i].EndTimeMs < out.Lines[j].EndTimeMs
		}
		return out.Lines[i].StartTimeMs < out.Lines[j].StartTimeMs
	})
	return out
}

func (d Document) HasTranslation() bool {
	for _, line := range d.Lines {
		if cleanAuxiliaryText(line.TranslatedLyric) != "" {
			return true
		}
	}
	return false
}

func (d Document) HasTransliteration() bool {
	for _, line := range d.Lines {
		if strings.TrimSpace(line.RomanLyric) != "" {
			return true
		}
		for _, word := range line.Words {
			if strings.TrimSpace(word.RomanText) != "" {
				return true
			}
		}
	}
	return false
}

func (d Document) HasWordTimeline() bool {
	for _, line := range d.Lines {
		if line.HasWordTimeline() {
			return true
		}
	}
	return false
}

func (d Document) HasTimedLine() bool {
	for _, line := range d.Lines {
		if line.StartTimeMs > 0 || line.EndTimeMs > line.StartTimeMs {
			return true
		}
	}
	return false
}

func IsUsable(d Document) bool {
	d = d.Normalized()
	if len(d.Lines) == 0 || !d.HasTimedLine() {
		return false
	}
	for _, line := range d.Lines {
		if strings.TrimSpace(line.Text()) != "" {
			return true
		}
	}
	return false
}

func (l Line) Text() string {
	var b strings.Builder
	for _, word := range l.Words {
		b.WriteString(word.Text)
	}
	return strings.TrimSpace(b.String())
}

func (l Line) HasWordTimeline() bool {
	nonBlank := 0
	for _, word := range l.Words {
		if strings.TrimSpace(word.Text) == "" {
			continue
		}
		nonBlank++
		if nonBlank > 1 && word.EndTimeMs > word.StartTimeMs {
			return true
		}
	}
	return false
}

func PreviewLines(d Document, limit int) []PreviewLine {
	if limit <= 0 {
		limit = 8
	}
	d = d.Normalized()
	out := make([]PreviewLine, 0, minInt(limit, len(d.Lines)))
	for _, line := range d.Lines {
		text := strings.TrimSpace(line.Text())
		if text == "" {
			continue
		}
		out = append(out, PreviewLine{
			StartTimeMs: line.StartTimeMs,
			EndTimeMs:   line.EndTimeMs,
			Text:        text,
			Translation: cleanAuxiliaryText(line.TranslatedLyric),
			Roman:       strings.TrimSpace(line.RomanLyric),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func PreviewText(d Document, limit int) string {
	lines := PreviewLines(d, limit)
	parts := make([]string, 0, len(lines)*3)
	for _, line := range lines {
		parts = append(parts, line.Text)
		if line.Translation != "" {
			parts = append(parts, line.Translation)
		}
		if line.Roman != "" {
			parts = append(parts, line.Roman)
		}
	}
	return strings.Join(parts, "\n")
}

func CurrentLine(d Document, positionMs int64) (PreviewLine, bool) {
	lines := PreviewLines(d, len(d.Lines))
	if len(lines) == 0 {
		return PreviewLine{}, false
	}
	for _, line := range lines {
		if positionMs >= line.StartTimeMs && (line.EndTimeMs <= line.StartTimeMs || positionMs < line.EndTimeMs) {
			return line, true
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if positionMs >= lines[i].StartTimeMs {
			return lines[i], true
		}
	}
	return lines[0], true
}

func (l Line) normalized() Line {
	l.TranslatedLyric = cleanAuxiliaryText(l.TranslatedLyric)
	l.RomanLyric = strings.TrimSpace(l.RomanLyric)
	words := make([]Word, 0, len(l.Words))
	for _, word := range l.Words {
		word.RomanText = strings.TrimSpace(word.RomanText)
		if word.EndTimeMs < word.StartTimeMs {
			word.EndTimeMs = word.StartTimeMs
		}
		words = append(words, word)
	}
	l.Words = words
	if l.StartTimeMs < 0 {
		l.StartTimeMs = 0
	}
	if l.EndTimeMs < l.StartTimeMs {
		l.EndTimeMs = l.StartTimeMs
	}
	if len(l.Words) > 0 {
		start := l.StartTimeMs
		end := l.EndTimeMs
		haveTimedWord := false
		for _, word := range l.Words {
			if strings.TrimSpace(word.Text) == "" {
				continue
			}
			if word.StartTimeMs > 0 || word.EndTimeMs > word.StartTimeMs {
				if !haveTimedWord || word.StartTimeMs < start {
					start = word.StartTimeMs
				}
				if word.EndTimeMs > end {
					end = word.EndTimeMs
				}
				haveTimedWord = true
			}
		}
		if haveTimedWord {
			l.StartTimeMs = start
			l.EndTimeMs = end
		}
	}
	return l
}

func (l Line) hasAnyContent() bool {
	if strings.TrimSpace(l.Text()) != "" {
		return true
	}
	return cleanAuxiliaryText(l.TranslatedLyric) != "" || strings.TrimSpace(l.RomanLyric) != ""
}

func cloneMetadata(metadata []Metadata) []Metadata {
	out := make([]Metadata, 0, len(metadata))
	for _, item := range metadata {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		values := make([]string, 0, len(item.Values))
		for _, value := range item.Values {
			value = strings.TrimSpace(value)
			if value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			out = append(out, Metadata{Key: key, Values: values})
		}
	}
	return out
}

func cleanAuxiliaryText(value string) string {
	value = strings.TrimSpace(value)
	if value == "//" {
		return ""
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
