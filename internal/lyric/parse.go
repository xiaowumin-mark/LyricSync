package lyric

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var lrcTimeTagPattern = regexp.MustCompile(`\[(\d{1,3}:\d{2}(?:[.:]\d{1,3})?)\]`)
var lrcMetadataPattern = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9_-]*):(.*)\]$`)

func Parse(input string) (Document, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Document{}, fmt.Errorf("lyric content is empty")
	}
	if looksLikeTTML(input) {
		return ParseTTML(input)
	}
	if lrcTimeTagPattern.MatchString(input) {
		return ParseLRC(input)
	}
	return ParsePlainText(input), nil
}

func ParseLRC(input string) (Document, error) {
	var doc Document
	for _, rawLine := range strings.Split(input, "\n") {
		rawLine = strings.TrimRight(rawLine, "\r")
		matches := lrcTimeTagPattern.FindAllStringSubmatchIndex(rawLine, -1)
		if len(matches) == 0 {
			if meta := parseLRCMetadata(rawLine); meta.Key != "" {
				doc.Metadata = append(doc.Metadata, meta)
			}
			continue
		}
		text := strings.TrimSpace(lrcTimeTagPattern.ReplaceAllString(rawLine, ""))
		if text == "" {
			continue
		}
		for _, match := range matches {
			start, err := ParseTimestamp(rawLine[match[2]:match[3]])
			if err != nil {
				return Document{}, err
			}
			doc.Lines = append(doc.Lines, Line{
				StartTimeMs: start,
				EndTimeMs:   start,
				Words: []Word{{
					StartTimeMs: start,
					EndTimeMs:   start,
					Text:        text,
				}},
			})
		}
	}
	sort.SliceStable(doc.Lines, func(i, j int) bool {
		return doc.Lines[i].StartTimeMs < doc.Lines[j].StartTimeMs
	})
	for i := range doc.Lines {
		end := doc.Lines[i].StartTimeMs + 5000
		if i+1 < len(doc.Lines) && doc.Lines[i+1].StartTimeMs > doc.Lines[i].StartTimeMs {
			end = doc.Lines[i+1].StartTimeMs
		}
		doc.Lines[i].EndTimeMs = end
		for j := range doc.Lines[i].Words {
			doc.Lines[i].Words[j].EndTimeMs = end
		}
	}
	return doc.Normalized(), nil
}

func ParsePlainText(input string) Document {
	lines := strings.Split(input, "\n")
	doc := Document{Lines: make([]Line, 0, len(lines))}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "//" {
			continue
		}
		doc.Lines = append(doc.Lines, Line{
			Words: []Word{{Text: line}},
		})
	}
	return doc.Normalized()
}

func looksLikeTTML(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(lower, "<") &&
		(strings.Contains(lower, "<tt") || strings.Contains(lower, "<p") || strings.Contains(lower, "ttml"))
}

func parseLRCMetadata(line string) Metadata {
	match := lrcMetadataPattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return Metadata{}
	}
	key := strings.TrimSpace(match[1])
	value := strings.TrimSpace(match[2])
	if key == "" || value == "" {
		return Metadata{}
	}
	return Metadata{Key: key, Values: []string{value}}
}
