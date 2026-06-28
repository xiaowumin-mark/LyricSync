package lyric

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	platformLineTimestampPattern = regexp.MustCompile(`^\[(\d+),(\d+)\]`)
	krcSyllablePattern           = regexp.MustCompile(`<(\d+),(\d+),\d+>([^<]*)`)
	krcLanguagePattern           = regexp.MustCompile(`\[language:([A-Za-z0-9+/=]+)\]`)
	yrcSyllablePattern           = regexp.MustCompile(`\((\d+),(\d+),\d+\)`)
	qrcTokenPattern              = regexp.MustCompile(`(.*?)\((\d+),(\d+)\)`)
)

func parsePlatformRaw(input string) (Document, bool, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Document{}, false, nil
	}
	if doc, ok, err := parseKRC(input); ok || err != nil {
		return doc, ok, err
	}
	if doc, ok, err := parseYRC(input); ok || err != nil {
		return doc, ok, err
	}
	if doc, ok, err := parseQRC(input); ok || err != nil {
		return doc, ok, err
	}
	return Document{}, false, nil
}

type krcAuxiliaryData struct {
	translations  []string
	romanizations [][]string
}

type krcLanguagePayload struct {
	Content []krcLanguageEntry `json:"content"`
}

type krcLanguageEntry struct {
	LyricContent [][]string `json:"lyricContent"`
	Type         int        `json:"type"`
}

func parseKRC(input string) (Document, bool, error) {
	aux := extractKRCAuxiliaryData(input)
	metadata := []Metadata{}
	lines := []Line{}
	sawKRC := krcLanguagePattern.MatchString(input) || krcSyllablePattern.MatchString(input)
	auxLineIndex := 0
	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" {
			continue
		}
		if isKRCIgnoredLine(line) {
			continue
		}
		if meta := parseLRCMetadata(line); meta.Key != "" {
			metadata = append(metadata, meta)
			continue
		}
		start, duration, content, ok := splitPlatformTimedLine(line)
		if !ok {
			continue
		}
		words := parseKRCWords(content, start)
		if len(words) == 0 {
			continue
		}
		out := Line{
			StartTimeMs: start,
			EndTimeMs:   lineEnd(start, duration, words),
			Words:       words,
		}
		if auxLineIndex < len(aux.translations) {
			out.TranslatedLyric = cleanAuxiliaryText(aux.translations[auxLineIndex])
		}
		if auxLineIndex < len(aux.romanizations) {
			applyRomanizationToWords(out.Words, aux.romanizations[auxLineIndex])
			out.RomanLyric = joinAuxiliaryParts(aux.romanizations[auxLineIndex])
		}
		lines = append(lines, out)
		auxLineIndex++
	}
	if len(lines) == 0 {
		return Document{}, sawKRC, nil
	}
	return Document{Metadata: metadata, Lines: lines}.Normalized(), true, nil
}

func isKRCIgnoredLine(line string) bool {
	prefixes := []string{
		"[language:", "[id:", "[hash:", "[total:", "[qq:", "[offset:", "[sign:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func extractKRCAuxiliaryData(input string) krcAuxiliaryData {
	match := krcLanguagePattern.FindStringSubmatch(input)
	if match == nil {
		return krcAuxiliaryData{}
	}
	decoded, err := base64.StdEncoding.DecodeString(match[1])
	if err != nil {
		return krcAuxiliaryData{}
	}
	var payload krcLanguagePayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return krcAuxiliaryData{}
	}
	out := krcAuxiliaryData{}
	for _, entry := range payload.Content {
		switch entry.Type {
		case 1:
			out.translations = make([]string, 0, len(entry.LyricContent))
			for _, parts := range entry.LyricContent {
				out.translations = append(out.translations, strings.Join(parts, ""))
			}
		case 0:
			out.romanizations = entry.LyricContent
		}
	}
	return out
}

func parseKRCWords(content string, lineStart int64) []Word {
	matches := krcSyllablePattern.FindAllStringSubmatch(content, -1)
	words := make([]Word, 0, len(matches))
	for _, match := range matches {
		offset, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		duration, err := strconv.ParseInt(match[2], 10, 64)
		if err != nil {
			continue
		}
		text := strings.Trim(match[3], "\r\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		start := maxInt64(lineStart+offset, 0)
		words = append(words, Word{
			StartTimeMs: start,
			EndTimeMs:   start + maxInt64(duration, 0),
			Text:        text,
		})
	}
	return words
}

func parseYRC(input string) (Document, bool, error) {
	metadata := []Metadata{}
	lines := []Line{}
	sawYRC := false
	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, `{"t":`) {
			if parsed := parseYRCMetadata(line); parsed.Key != "" {
				metadata = append(metadata, parsed)
			}
			continue
		}
		start, duration, content, ok := splitPlatformTimedLine(line)
		if !ok || !yrcSyllablePattern.MatchString(content) {
			continue
		}
		sawYRC = true
		words := parseYRCWords(content)
		if len(words) == 0 {
			continue
		}
		lines = append(lines, Line{
			StartTimeMs: start,
			EndTimeMs:   lineEnd(start, duration, words),
			Words:       words,
		})
	}
	if len(lines) == 0 {
		return Document{}, sawYRC, nil
	}
	return Document{Metadata: metadata, Lines: lines}.Normalized(), true, nil
}

func parseYRCWords(content string) []Word {
	matches := yrcSyllablePattern.FindAllStringSubmatchIndex(content, -1)
	words := make([]Word, 0, len(matches))
	for i, match := range matches {
		textStart := match[1]
		textEnd := len(content)
		if i+1 < len(matches) {
			textEnd = matches[i+1][0]
		}
		if textStart > textEnd {
			continue
		}
		start, err := strconv.ParseInt(content[match[2]:match[3]], 10, 64)
		if err != nil {
			continue
		}
		duration, err := strconv.ParseInt(content[match[4]:match[5]], 10, 64)
		if err != nil {
			continue
		}
		text := strings.Trim(content[textStart:textEnd], "\r\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		words = append(words, Word{
			StartTimeMs: maxInt64(start, 0),
			EndTimeMs:   maxInt64(start, 0) + maxInt64(duration, 0),
			Text:        text,
		})
	}
	return words
}

func parseYRCMetadata(line string) Metadata {
	var value struct {
		Content []struct {
			Text string `json:"tx"`
		} `json:"c"`
	}
	if err := json.Unmarshal([]byte(line), &value); err != nil || len(value.Content) < 2 {
		return Metadata{}
	}
	key := strings.TrimSpace(value.Content[0].Text)
	key = strings.TrimRight(key, ":： \t")
	if key == "" {
		return Metadata{}
	}
	parts := make([]string, 0, len(value.Content)-1)
	for _, item := range value.Content[1:] {
		text := strings.TrimSpace(item.Text)
		if text != "" && text != "/" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return Metadata{}
	}
	return Metadata{Key: key, Values: []string{strings.Join(parts, ", ")}}
}

func parseQRC(input string) (Document, bool, error) {
	metadata := []Metadata{}
	lines := []Line{}
	sawQRC := false
	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "[kana:") {
			continue
		}
		if meta := parseLRCMetadata(line); meta.Key != "" {
			metadata = append(metadata, meta)
			continue
		}
		start, duration, content, ok := splitPlatformTimedLine(line)
		if !ok || !qrcTokenPattern.MatchString(content) {
			continue
		}
		sawQRC = true
		words := parseQRCWords(content)
		if len(words) == 0 {
			continue
		}
		background := wrappedAsBackgroundLine(wordsText(words))
		if background {
			stripBackgroundWrapping(words)
		}
		lines = append(lines, Line{
			StartTimeMs:  start,
			EndTimeMs:    lineEnd(start, duration, words),
			Words:        words,
			IsBackground: background,
		})
	}
	if len(lines) == 0 {
		return Document{}, sawQRC, nil
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].StartTimeMs < lines[j].StartTimeMs
	})
	return Document{Metadata: metadata, Lines: lines}.Normalized(), true, nil
}

func parseQRCWords(content string) []Word {
	matches := qrcTokenPattern.FindAllStringSubmatchIndex(content, -1)
	words := make([]Word, 0, len(matches))
	for _, match := range matches {
		text := strings.Trim(content[match[2]:match[3]], "\r\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		start, err := strconv.ParseInt(content[match[4]:match[5]], 10, 64)
		if err != nil {
			continue
		}
		duration, err := strconv.ParseInt(content[match[6]:match[7]], 10, 64)
		if err != nil {
			continue
		}
		words = append(words, Word{
			StartTimeMs: maxInt64(start, 0),
			EndTimeMs:   maxInt64(start, 0) + maxInt64(duration, 0),
			Text:        text,
		})
	}
	return words
}

func splitPlatformTimedLine(line string) (int64, int64, string, bool) {
	match := platformLineTimestampPattern.FindStringSubmatchIndex(line)
	if match == nil {
		return 0, 0, "", false
	}
	start, err := strconv.ParseInt(line[match[2]:match[3]], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	duration, err := strconv.ParseInt(line[match[4]:match[5]], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	return maxInt64(start, 0), maxInt64(duration, 0), strings.TrimSpace(line[match[1]:]), true
}

func lineEnd(start int64, duration int64, words []Word) int64 {
	end := start + maxInt64(duration, 0)
	for _, word := range words {
		if word.EndTimeMs > end {
			end = word.EndTimeMs
		}
	}
	if end <= start {
		end = start + 5000
	}
	return end
}

func applyRomanizationToWords(words []Word, romanParts []string) {
	for i := range words {
		if i >= len(romanParts) {
			return
		}
		words[i].RomanText = strings.TrimSpace(romanParts[i])
	}
}

func joinAuxiliaryParts(parts []string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

func wordsText(words []Word) string {
	var b strings.Builder
	for _, word := range words {
		b.WriteString(word.Text)
	}
	return strings.TrimSpace(b.String())
}

func wrappedAsBackgroundLine(text string) bool {
	text = strings.TrimSpace(text)
	return len([]rune(text)) >= 2 &&
		((strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")")) ||
			(strings.HasPrefix(text, "（") && strings.HasSuffix(text, "）")))
}

func stripBackgroundWrapping(words []Word) {
	first := -1
	last := -1
	for i, word := range words {
		if strings.TrimSpace(word.Text) == "" {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 || last < 0 {
		return
	}
	words[first].Text = stripFirstBackgroundBracket(words[first].Text)
	words[last].Text = stripLastBackgroundBracket(words[last].Text)
}

func stripFirstBackgroundBracket(value string) string {
	runes := []rune(value)
	for i, r := range runes {
		if r == '(' || r == '（' {
			return string(runes[:i]) + string(runes[i+1:])
		}
		if !isInlineSpace(r) {
			break
		}
	}
	return value
}

func stripLastBackgroundBracket(value string) string {
	runes := []rune(value)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ')' || runes[i] == '）' {
			return string(runes[:i]) + string(runes[i+1:])
		}
		if !isInlineSpace(runes[i]) {
			break
		}
	}
	return value
}

func isInlineSpace(r rune) bool {
	return r == ' ' || r == '\t'
}
