package lyric

import (
	"encoding/base64"
	"testing"

	ttml "github.com/xiaowumin-mark/amll-ttml"

	"lyricsync/pkg/model"
)

func TestOffsetDocumentShiftsTTMLAndModelLines(t *testing.T) {
	doc := DemoDocument()
	originalStart := doc.Lines[1].StartMs

	shifted, err := OffsetDocument(doc, 1250)
	if err != nil {
		t.Fatal(err)
	}
	if shifted.Lines[1].StartMs != originalStart+1250 {
		t.Fatalf("expected shifted start %d, got %d", originalStart+1250, shifted.Lines[1].StartMs)
	}
	if shifted.TTML == doc.TTML {
		t.Fatal("expected TTML to be regenerated after offset")
	}
	if shifted.AMLXBase64 == "" {
		t.Fatal("expected AMLX output after offset")
	}
}

func TestRebuildDocumentPreservesTranslationAndRomanization(t *testing.T) {
	doc := model.LyricDocument{
		ID:      "lyric-1",
		TrackID: "track-1",
		Source:  "test",
		Lines: []model.LyricLine{
			{StartMs: 0, EndMs: 1000, Text: "ありがとう", Translation: "thank you", Romanization: "arigatou"},
		},
	}

	rebuilt, err := RebuildDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.TTML == "" || rebuilt.AMLXBase64 == "" {
		t.Fatal("expected regenerated TTML and AMLX")
	}

	data, err := base64.StdEncoding.DecodeString(rebuilt.AMLXBase64)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ttml.DecodeBinary(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.LyricLines[0].TranslatedLyric != "thank you" {
		t.Fatalf("translation not preserved: %#v", decoded.LyricLines[0])
	}
	if decoded.LyricLines[0].RomanLyric != "arigatou" {
		t.Fatalf("romanization not preserved: %#v", decoded.LyricLines[0])
	}
}

func TestAlignDocumentUsesLineAsAnchor(t *testing.T) {
	doc := DemoDocument()
	targetPosition := int64(45000)
	aligned, offset, err := AlignDocument(doc, 1, targetPosition)
	if err != nil {
		t.Fatal(err)
	}
	if aligned.Lines[1].StartMs != targetPosition {
		t.Fatalf("expected anchor line at %d, got %d", targetPosition, aligned.Lines[1].StartMs)
	}
	if offset != targetPosition-doc.Lines[1].StartMs {
		t.Fatalf("unexpected offset %d", offset)
	}
	if aligned.TTML == doc.TTML {
		t.Fatal("expected TTML to be regenerated after alignment")
	}
}
