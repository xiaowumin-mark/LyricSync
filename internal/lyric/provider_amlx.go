package lyric

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	musicapi "github.com/xiaowumin-mark/AMLX-MUSIC-API"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/kugou"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/netease"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/qqmusic"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

type amlxProvider struct {
	source string
	name   string
	client *http.Client
}

func newQQProvider(client *http.Client) Provider {
	return &amlxProvider{source: model.LyricSourceQQ, name: "qq", client: client}
}

func newKugouProvider(client *http.Client) Provider {
	return &amlxProvider{source: model.LyricSourceKugou, name: "kugou", client: client}
}

func newNeteaseProvider(client *http.Client) Provider {
	return &amlxProvider{source: model.LyricSourceNetease, name: "netease", client: client}
}

func (p *amlxProvider) Source() string {
	return p.source
}

func (p *amlxProvider) Search(ctx context.Context, query TrackQuery) (ProviderResult, error) {
	query = query.Clean()
	if query.Title == "" {
		return ProviderResult{}, fmt.Errorf("%s query title is empty", p.source)
	}
	client, err := musicapi.Get(p.name, musicapi.WithHTTPClient(p.client), musicapi.WithLyricClean(true))
	if err != nil {
		return ProviderResult{}, err
	}
	result, err := client.Search(ctx, query.Keyword(), musicapi.SearchTypeSong, 1, 8)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%s search: %w", p.source, err)
	}
	best, score, ok := bestAMLXSong(query, result.Songs)
	if !ok {
		return ProviderResult{}, fmt.Errorf("%s lyric not found", p.source)
	}
	fetched, err := client.GetLyric(ctx, best.ID)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%s lyric: %w", p.source, err)
	}
	doc := documentFromAMLXLyric(fetched, metadataForResult(best.Name, songArtists(best), songAlbum(best), p.source, best.ID))
	providerResult, err := NewProviderResult(p.source, best.ID, rawAMLXLyric(fetched), doc)
	if err != nil {
		return ProviderResult{}, err
	}
	providerResult.Title = best.Name
	providerResult.Artist = songArtists(best)
	providerResult.Album = songAlbum(best)
	providerResult.DurationMs = int64(best.Duration) * 1000
	providerResult.Score = score
	return providerResult, nil
}

func bestAMLXSong(query TrackQuery, songs []*musicapi.Song) (*musicapi.Song, int, bool) {
	var best *musicapi.Song
	bestScore := 0
	for _, item := range songs {
		if item == nil || strings.TrimSpace(item.ID) == "" {
			continue
		}
		score := MatchScore(query, item.Name, songArtists(item), songAlbum(item), int64(item.Duration)*1000)
		if score > bestScore {
			best = item
			bestScore = score
		}
	}
	if best == nil || bestScore < 30 {
		return nil, 0, false
	}
	return best, bestScore, true
}

func documentFromAMLXLyric(input *musicapi.Lyric, metadata []Metadata) Document {
	if input == nil {
		return Document{}
	}
	translations := convertAMLXLines(input.Translation)
	romanizations := convertAMLXLines(input.Romanization)
	if doc, ok, err := parsePlatformRaw(input.Raw); err == nil && ok && IsUsable(doc) {
		doc.Metadata = append(cloneMetadata(metadata), doc.Metadata...)
		return mergeDocumentAuxiliaryLines(doc, translations, romanizations)
	}
	return DocumentFromTimedLines(convertAMLXLines(input.Lines), translations, romanizations, metadata)
}

func convertAMLXLines(lines []musicapi.LyricLine) []TimedTextLine {
	out := make([]TimedTextLine, 0, len(lines))
	for _, line := range lines {
		item := TimedTextLine{
			StartTimeMs: line.Time,
			DurationMs:  line.Duration,
			Text:        line.Text,
			Syllables:   make([]TimedSyllable, 0, len(line.Syllables)),
		}
		for _, syllable := range line.Syllables {
			item.Syllables = append(item.Syllables, TimedSyllable{
				StartTimeMs: syllable.Time,
				DurationMs:  syllable.Duration,
				Text:        syllable.Text,
			})
		}
		out = append(out, item)
	}
	return out
}

func mergeDocumentAuxiliaryLines(doc Document, translations []TimedTextLine, romanizations []TimedTextLine) Document {
	doc = doc.Normalized()
	for i := range doc.Lines {
		if strings.TrimSpace(doc.Lines[i].TranslatedLyric) == "" {
			doc.Lines[i].TranslatedLyric = matchingAuxText(translations, doc.Lines[i].StartTimeMs)
		}
		if strings.TrimSpace(doc.Lines[i].RomanLyric) == "" {
			doc.Lines[i].RomanLyric = matchingAuxText(romanizations, doc.Lines[i].StartTimeMs)
		}
	}
	return doc.Normalized()
}

func rawAMLXLyric(input *musicapi.Lyric) string {
	if input == nil {
		return ""
	}
	if strings.TrimSpace(input.Raw) != "" {
		return input.Raw
	}
	return ProviderRawText(linesToLRC(input.Lines), linesToLRC(input.Translation), linesToLRC(input.Romanization))
}

func linesToLRC(lines []musicapi.LyricLine) string {
	var b strings.Builder
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}
		b.WriteByte('[')
		b.WriteString(FormatTimestamp(line.Time))
		b.WriteByte(']')
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func songArtists(song *musicapi.Song) string {
	if song == nil {
		return ""
	}
	parts := make([]string, 0, len(song.Artists))
	for _, artist := range song.Artists {
		if strings.TrimSpace(artist.Name) != "" {
			parts = append(parts, strings.TrimSpace(artist.Name))
		}
	}
	return strings.Join(parts, ", ")
}

func songAlbum(song *musicapi.Song) string {
	if song == nil || song.Album == nil {
		return ""
	}
	return strings.TrimSpace(song.Album.Name)
}
