package lyric

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type xmlTreeNode struct {
	Name     xml.Name
	Attr     []xml.Attr
	Children []xmlTreeChild
}

type xmlTreeChild struct {
	Node *xmlTreeNode
	Text string
}

func ParseTTML(input string) (Document, error) {
	root, err := parseXMLTree(input)
	if err != nil {
		return Document{}, err
	}
	doc := Document{}
	for _, node := range allElements(root) {
		if node.Name.Local != "meta" {
			continue
		}
		key := attrLocal(node, "key")
		value := attrLocal(node, "value")
		if key == "" || value == "" {
			continue
		}
		addMetadata(&doc, key, value)
	}
	for _, node := range allElements(root) {
		if node.Name.Local != "p" {
			continue
		}
		lines, err := parseTTMLLine(node, false, false)
		if err != nil {
			return Document{}, err
		}
		doc.Lines = append(doc.Lines, lines...)
	}
	return doc.Normalized(), nil
}

func GenerateTTML(doc Document, pretty bool) string {
	doc = doc.Normalized()
	var b strings.Builder
	w := newXMLWriter(&b, pretty)
	timing := "None"
	if doc.HasTimedLine() {
		timing = "Line"
	}
	if doc.HasWordTimeline() {
		timing = "Word"
	}
	w.start("tt", map[string]string{
		"xmlns":         "http://www.w3.org/ns/ttml",
		"xmlns:ttm":     "http://www.w3.org/ns/ttml#metadata",
		"xmlns:amll":    "http://www.example.com/ns/amll",
		"xmlns:itunes":  "http://music.apple.com/lyric-ttml-internal",
		"itunes:timing": timing,
	})
	w.start("head", nil)
	w.start("metadata", nil)
	w.empty("ttm:agent", map[string]string{"type": "person", "xml:id": "v1"})
	if hasDuet(doc) {
		w.empty("ttm:agent", map[string]string{"type": "other", "xml:id": "v2"})
	}
	for _, meta := range doc.Metadata {
		for _, value := range meta.Values {
			w.empty("amll:meta", map[string]string{"key": meta.Key, "value": value})
		}
	}
	w.end("metadata")
	w.end("head")

	start, end := documentRange(doc)
	w.start("body", map[string]string{"dur": FormatTimestamp(end)})
	w.start("div", map[string]string{
		"begin": FormatTimestamp(start),
		"end":   FormatTimestamp(end),
	})
	for i := 0; i < len(doc.Lines); i++ {
		line := doc.Lines[i]
		if line.IsBackground {
			writeTTMLLine(w, line, nil)
			continue
		}
		var bg *Line
		if i+1 < len(doc.Lines) && doc.Lines[i+1].IsBackground {
			bg = &doc.Lines[i+1]
			i++
		}
		writeTTMLLine(w, line, bg)
	}
	w.end("div")
	w.end("body")
	w.end("tt")
	return b.String()
}

func CompressTTML(input string) (string, error) {
	doc, err := ParseTTML(input)
	if err != nil {
		return "", err
	}
	return GenerateTTML(doc, false), nil
}

func parseXMLTree(input string) (*xmlTreeNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	decoder.Strict = false
	root := &xmlTreeNode{Name: xml.Name{Local: "document"}}
	stack := []*xmlTreeNode{root}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			node := &xmlTreeNode{Name: token.Name, Attr: append([]xml.Attr(nil), token.Attr...)}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, xmlTreeChild{Node: node})
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := string([]byte(token))
			if text != "" {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, xmlTreeChild{Text: text})
			}
		}
	}
	return root, nil
}

func parseTTMLLine(node *xmlTreeNode, background bool, duet bool) ([]Line, error) {
	start, startOK, err := attrTimestamp(node, "begin")
	if err != nil {
		return nil, err
	}
	end, endOK, err := attrTimestamp(node, "end")
	if err != nil {
		return nil, err
	}
	role := attrLocal(node, "role")
	if role == "x-bg" {
		background = true
	}
	line := Line{
		StartTimeMs:  start,
		EndTimeMs:    end,
		IsBackground: background,
		IsDuet:       duet || attrLocal(node, "agent") == "v2",
	}
	var nested []Line
	for _, child := range node.Children {
		if child.Node == nil {
			if child.Text != "" && (!strings.Contains(child.Text, "\n") || strings.TrimSpace(child.Text) != "") {
				line.Words = append(line.Words, Word{
					StartTimeMs: start,
					EndTimeMs:   end,
					Text:        child.Text,
				})
			}
			continue
		}
		childRole := attrLocal(child.Node, "role")
		switch childRole {
		case "x-translation":
			if line.TranslatedLyric == "" {
				line.TranslatedLyric = child.Node.textContent()
			}
			continue
		case "x-roman":
			if line.RomanLyric == "" {
				line.RomanLyric = child.Node.textContent()
			}
			continue
		case "x-bg":
			bgLines, err := parseTTMLLine(child.Node, true, line.IsDuet)
			if err != nil {
				return nil, err
			}
			nested = append(nested, bgLines...)
			continue
		}
		if child.Node.Name.Local == "span" && attrLocal(child.Node, "begin") != "" && attrLocal(child.Node, "end") != "" {
			wordStart, _, err := attrTimestamp(child.Node, "begin")
			if err != nil {
				return nil, err
			}
			wordEnd, _, err := attrTimestamp(child.Node, "end")
			if err != nil {
				return nil, err
			}
			line.Words = append(line.Words, Word{
				StartTimeMs: wordStart,
				EndTimeMs:   wordEnd,
				Text:        child.Node.textContent(),
				Obscene:     strings.EqualFold(attrLocal(child.Node, "obscene"), "true"),
				EmptyBeat:   parseOptionalFloat(attrLocal(child.Node, "empty-beat")),
			})
			continue
		}
		text := child.Node.textContent()
		if text != "" {
			line.Words = append(line.Words, Word{StartTimeMs: start, EndTimeMs: end, Text: text})
		}
	}
	if !startOK || !endOK {
		line.StartTimeMs, line.EndTimeMs = rangeFromWords(line.Words)
	}
	lines := []Line{line}
	lines = append(lines, nested...)
	return lines, nil
}

func writeTTMLLine(w *xmlWriter, line Line, background *Line) {
	attrs := map[string]string{
		"begin":      FormatTimestamp(line.StartTimeMs),
		"end":        FormatTimestamp(line.EndTimeMs),
		"ttm:agent":  "v1",
		"itunes:key": "L" + strconv.Itoa(w.nextLineKey()),
	}
	if line.IsDuet {
		attrs["ttm:agent"] = "v2"
	}
	if line.IsBackground {
		attrs["ttm:role"] = "x-bg"
	}
	w.start("p", attrs)
	writeTTMLWords(w, line)
	if background != nil {
		w.start("span", map[string]string{
			"ttm:role": "x-bg",
			"begin":    FormatTimestamp(background.StartTimeMs),
			"end":      FormatTimestamp(background.EndTimeMs),
		})
		writeTTMLWords(w, *background)
		writeAuxiliarySpans(w, *background)
		w.end("span")
	}
	writeAuxiliarySpans(w, line)
	w.end("p")
}

func writeTTMLWords(w *xmlWriter, line Line) {
	if len(line.Words) == 0 {
		return
	}
	dynamic := line.HasWordTimeline()
	for _, word := range line.Words {
		if dynamic && strings.TrimSpace(word.Text) != "" {
			attrs := map[string]string{
				"begin": FormatTimestamp(word.StartTimeMs),
				"end":   FormatTimestamp(word.EndTimeMs),
			}
			if word.Obscene {
				attrs["amll:obscene"] = "true"
			}
			if word.EmptyBeat != 0 && !math.IsNaN(word.EmptyBeat) {
				attrs["amll:empty-beat"] = strconv.FormatFloat(word.EmptyBeat, 'f', -1, 64)
			}
			w.start("span", attrs)
			w.text(word.Text)
			w.end("span")
			continue
		}
		w.text(word.Text)
	}
}

func writeAuxiliarySpans(w *xmlWriter, line Line) {
	if cleanAuxiliaryText(line.TranslatedLyric) != "" {
		w.start("span", map[string]string{"ttm:role": "x-translation", "xml:lang": "zh-CN"})
		w.text(cleanAuxiliaryText(line.TranslatedLyric))
		w.end("span")
	}
	if strings.TrimSpace(line.RomanLyric) != "" {
		w.start("span", map[string]string{"ttm:role": "x-roman"})
		w.text(strings.TrimSpace(line.RomanLyric))
		w.end("span")
	}
}

func allElements(root *xmlTreeNode) []*xmlTreeNode {
	var out []*xmlTreeNode
	var walk func(*xmlTreeNode)
	walk = func(node *xmlTreeNode) {
		if node == nil {
			return
		}
		if node.Name.Local != "document" {
			out = append(out, node)
		}
		for _, child := range node.Children {
			if child.Node != nil {
				walk(child.Node)
			}
		}
	}
	walk(root)
	return out
}

func (n *xmlTreeNode) textContent() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	for _, child := range n.Children {
		if child.Node != nil {
			b.WriteString(child.Node.textContent())
		} else {
			b.WriteString(child.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func attrLocal(node *xmlTreeNode, local string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func attrTimestamp(node *xmlTreeNode, local string) (int64, bool, error) {
	value := strings.TrimSpace(attrLocal(node, local))
	if value == "" {
		return 0, false, nil
	}
	ms, err := ParseTimestamp(value)
	if err != nil {
		return 0, true, err
	}
	return ms, true, nil
}

func parseOptionalFloat(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func rangeFromWords(words []Word) (int64, int64) {
	if len(words) == 0 {
		return 0, 0
	}
	start := int64(0)
	end := int64(0)
	found := false
	for _, word := range words {
		if strings.TrimSpace(word.Text) == "" {
			continue
		}
		if !found || word.StartTimeMs < start {
			start = word.StartTimeMs
		}
		if word.EndTimeMs > end {
			end = word.EndTimeMs
		}
		found = true
	}
	if !found {
		return 0, 0
	}
	return start, end
}

func addMetadata(doc *Document, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	for i := range doc.Metadata {
		if doc.Metadata[i].Key == key {
			doc.Metadata[i].Values = append(doc.Metadata[i].Values, value)
			return
		}
	}
	doc.Metadata = append(doc.Metadata, Metadata{Key: key, Values: []string{value}})
}

func documentRange(doc Document) (int64, int64) {
	if len(doc.Lines) == 0 {
		return 0, 0
	}
	start := doc.Lines[0].StartTimeMs
	end := doc.Lines[0].EndTimeMs
	for _, line := range doc.Lines {
		if line.StartTimeMs < start {
			start = line.StartTimeMs
		}
		if line.EndTimeMs > end {
			end = line.EndTimeMs
		}
	}
	return start, end
}

func hasDuet(doc Document) bool {
	for _, line := range doc.Lines {
		if line.IsDuet {
			return true
		}
	}
	return false
}

type xmlWriter struct {
	b       *strings.Builder
	pretty  bool
	depth   int
	lineKey int
}

func newXMLWriter(b *strings.Builder, pretty bool) *xmlWriter {
	return &xmlWriter{b: b, pretty: pretty}
}

func (w *xmlWriter) nextLineKey() int {
	w.lineKey++
	return w.lineKey
}

func (w *xmlWriter) start(name string, attrs map[string]string) {
	w.indent()
	w.b.WriteByte('<')
	w.b.WriteString(name)
	w.attrs(attrs)
	w.b.WriteByte('>')
	w.depth++
	if w.pretty {
		w.b.WriteByte('\n')
	}
}

func (w *xmlWriter) end(name string) {
	w.depth--
	w.indent()
	w.b.WriteString("</")
	w.b.WriteString(name)
	w.b.WriteByte('>')
	if w.pretty {
		w.b.WriteByte('\n')
	}
}

func (w *xmlWriter) empty(name string, attrs map[string]string) {
	w.indent()
	w.b.WriteByte('<')
	w.b.WriteString(name)
	w.attrs(attrs)
	w.b.WriteString("/>")
	if w.pretty {
		w.b.WriteByte('\n')
	}
}

func (w *xmlWriter) text(text string) {
	if text == "" {
		return
	}
	if w.pretty && strings.TrimSpace(text) == text {
		w.indent()
	}
	w.b.WriteString(escapeXML(text))
	if w.pretty && strings.TrimSpace(text) == text {
		w.b.WriteByte('\n')
	}
}

func (w *xmlWriter) attrs(attrs map[string]string) {
	for _, key := range sortedKeys(attrs) {
		value := attrs[key]
		if value == "" {
			continue
		}
		w.b.WriteByte(' ')
		w.b.WriteString(key)
		w.b.WriteString(`="`)
		w.b.WriteString(escapeXML(value))
		w.b.WriteByte('"')
	}
}

func (w *xmlWriter) indent() {
	if !w.pretty {
		return
	}
	w.b.WriteString(strings.Repeat("  ", w.depth))
}

func sortedKeys(attrs map[string]string) []string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		value := values[i]
		j := i - 1
		for j >= 0 && values[j] > value {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = value
	}
}

func escapeXML(value string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return fmt.Sprint(value)
	}
	return b.String()
}
