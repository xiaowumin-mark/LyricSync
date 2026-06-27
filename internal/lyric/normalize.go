package lyric

import "strings"

type NormalizedContent struct {
	Document           Document
	TTML               string
	Available          bool
	HasTranslation     bool
	HasTransliteration bool
	HasWordTimeline    bool
}

func NormalizeContent(rawLyric string, ttmlLyric string) (NormalizedContent, error) {
	rawLyric = strings.TrimSpace(rawLyric)
	ttmlLyric = strings.TrimSpace(ttmlLyric)
	if rawLyric == "" && ttmlLyric == "" {
		return NormalizedContent{}, nil
	}
	var (
		doc Document
		err error
	)
	if ttmlLyric != "" {
		doc, err = ParseTTML(ttmlLyric)
	} else {
		doc, err = Parse(rawLyric)
	}
	if err != nil {
		return NormalizedContent{}, err
	}
	doc = doc.Normalized()
	available := IsUsable(doc)
	normalizedTTML := ""
	if available {
		normalizedTTML = GenerateTTML(doc, false)
	}
	return NormalizedContent{
		Document:           doc,
		TTML:               normalizedTTML,
		Available:          available,
		HasTranslation:     doc.HasTranslation(),
		HasTransliteration: doc.HasTransliteration(),
		HasWordTimeline:    doc.HasWordTimeline(),
	}, nil
}
