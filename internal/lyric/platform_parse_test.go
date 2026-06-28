package lyric

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseKRCWithEmbeddedTranslationAndRomanization(t *testing.T) {
	payload := `{"content":[{"lyricContent":[["你好世界"]],"type":1},{"lyricContent":[["ni","hao"]],"type":0}]}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	input := "[language:" + encoded + "]\n[1000,1200]<0,500,0>你<500,700,0>好"
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !IsUsable(doc) || len(doc.Lines) != 1 {
		t.Fatalf("expected usable KRC document, got %#v", doc)
	}
	line := doc.Lines[0]
	if got := line.Text(); got != "你好" {
		t.Fatalf("line text = %q", got)
	}
	if line.TranslatedLyric != "你好世界" {
		t.Fatalf("translation = %q", line.TranslatedLyric)
	}
	if line.RomanLyric != "ni hao" {
		t.Fatalf("roman line = %q", line.RomanLyric)
	}
	if line.Words[0].RomanText != "ni" || line.Words[1].RomanText != "hao" {
		t.Fatalf("word romanization not applied: %#v", line.Words)
	}
}

func TestParseQRCWithBackgroundLine(t *testing.T) {
	input := `
[97648,4632]The (97648,384)scars (98032,565)
[96826,3715](You're (96826,333)gonna)(97159,299)
`
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !IsUsable(doc) || len(doc.Lines) != 2 {
		t.Fatalf("expected two QRC lines, got %#v", doc)
	}
	var bg Line
	for _, line := range doc.Lines {
		if line.IsBackground {
			bg = line
		}
	}
	if !bg.IsBackground {
		t.Fatalf("expected a background line: %#v", doc.Lines)
	}
	if strings.Contains(bg.Text(), "(") || strings.Contains(bg.Text(), ")") {
		t.Fatalf("background wrapping should be stripped, got %q", bg.Text())
	}
}

func TestParseYRCAndLRCFallback(t *testing.T) {
	yrc, err := Parse("[1000,1200](1000,500,0)你(1500,700,0)好")
	if err != nil {
		t.Fatal(err)
	}
	if !yrc.HasWordTimeline() || yrc.Lines[0].Text() != "你好" {
		t.Fatalf("expected YRC word timeline, got %#v", yrc)
	}
	lrc, err := Parse("[00:01.00]你好")
	if err != nil {
		t.Fatal(err)
	}
	if !IsUsable(lrc) || lrc.HasWordTimeline() {
		t.Fatalf("expected line-timed LRC fallback, got %#v", lrc)
	}
}
