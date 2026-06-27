package lyric

import (
	"strings"
	"testing"
)

func TestParseLRCToUnifiedDocument(t *testing.T) {
	doc, err := ParseLRC("[ti:Demo]\n[00:01.00]第一行\n[00:03.50]第二行")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %#v", doc.Lines)
	}
	if got := doc.Lines[0].Text(); got != "第一行" {
		t.Fatalf("line text = %q", got)
	}
	if doc.Lines[0].StartTimeMs != 1000 || doc.Lines[0].EndTimeMs != 3500 {
		t.Fatalf("unexpected line timing: %#v", doc.Lines[0])
	}
	if !IsUsable(doc) {
		t.Fatal("expected timed LRC to be usable")
	}
}

func TestParseTTMLWithTranslationAndRoman(t *testing.T) {
	input := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:amll="http://www.example.com/ns/amll" xmlns:itunes="http://music.apple.com/lyric-ttml-internal"><head><metadata><amll:meta key="musicName" value="Demo"/></metadata></head><body><div><p begin="00:01.000" end="00:03.000"><span begin="00:01.000" end="00:02.000">Hel</span><span begin="00:02.000" end="00:03.000">lo</span><span ttm:role="x-translation" xml:lang="zh-CN">你好</span><span ttm:role="x-roman">ni hao</span></p></div></body></tt>`
	doc, err := ParseTTML(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Lines) != 1 {
		t.Fatalf("expected one line, got %#v", doc.Lines)
	}
	line := doc.Lines[0]
	if line.Text() != "Hello" || line.TranslatedLyric != "你好" || line.RomanLyric != "ni hao" {
		t.Fatalf("unexpected parsed line: %#v", line)
	}
	if !doc.HasWordTimeline() || !doc.HasTranslation() || !doc.HasTransliteration() {
		t.Fatalf("expected word timeline, translation and transliteration: %#v", doc)
	}
}

func TestGenerateTTMLRoundTripAndCompress(t *testing.T) {
	doc := Document{
		Metadata: []Metadata{{Key: "musicName", Values: []string{"Demo"}}},
		Lines: []Line{{
			StartTimeMs:     1000,
			EndTimeMs:       3000,
			TranslatedLyric: "你好",
			Words: []Word{
				{StartTimeMs: 1000, EndTimeMs: 2000, Text: "Hel"},
				{StartTimeMs: 2000, EndTimeMs: 3000, Text: "lo"},
			},
		}},
	}
	pretty := GenerateTTML(doc, true)
	compact, err := CompressTTML(pretty)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(compact, "\n") {
		t.Fatalf("expected compact TTML, got %q", compact)
	}
	roundTrip, err := ParseTTML(compact)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Lines[0].Text() != "Hello" || roundTrip.Lines[0].TranslatedLyric != "你好" {
		t.Fatalf("round trip mismatch: %#v", roundTrip.Lines[0])
	}
}

func TestPreviewSkipsPlaceholderTranslation(t *testing.T) {
	doc := Document{Lines: []Line{{
		StartTimeMs:     1000,
		EndTimeMs:       2000,
		TranslatedLyric: "//",
		Words:           []Word{{StartTimeMs: 1000, EndTimeMs: 2000, Text: "第一行"}},
	}}}
	if got := PreviewText(doc, 8); got != "第一行" {
		t.Fatalf("preview = %q", got)
	}
}

func TestNormalizeContentGeneratesTTMLFromRawLRC(t *testing.T) {
	got, err := NormalizeContent("[00:01.00]hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.TTML == "" {
		t.Fatalf("expected available normalized TTML, got %#v", got)
	}
	if _, err := ParseTTML(got.TTML); err != nil {
		t.Fatalf("generated TTML should parse: %v", err)
	}
}
