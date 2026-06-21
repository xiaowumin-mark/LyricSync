package lyric

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	musicapi "github.com/xiaowumin-mark/AMLX-MUSIC-API"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/kugou"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/netease"
	_ "github.com/xiaowumin-mark/AMLX-MUSIC-API/qqmusic"

	"lyricsync/pkg/model"
)

type OnlineProvider struct {
	sources []string
	client  *http.Client
}

func NewOnlineProvider(sources []string) *OnlineProvider {
	return &OnlineProvider{
		sources: normalizeSources(sources),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *OnlineProvider) Search(ctx context.Context, track model.Track) (model.LyricDocument, error) {
	keyword := queryForTrack(track)
	if keyword == "" {
		return model.LyricDocument{}, fmt.Errorf("lyric: empty search keyword")
	}

	var errs []string
	for _, source := range p.sources {
		providerName := providerAlias(source)
		if providerName == "" {
			continue
		}
		client, err := musicapi.Get(providerName, musicapi.WithHTTPClient(p.client))
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		result, err := client.Search(ctx, keyword, musicapi.SearchTypeSong, 1, 8)
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		song := chooseSong(track, result.Songs)
		if song == nil {
			errs = append(errs, source+": no song match")
			continue
		}
		apiLyric, err := client.GetLyric(ctx, song.ID)
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		doc, err := FromMusicAPILyric(track, source, song, apiLyric)
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		return doc, nil
	}

	if len(errs) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no enabled online provider")
	}
	return model.LyricDocument{}, fmt.Errorf("lyric: online search failed: %s", strings.Join(errs, "; "))
}

func (p *OnlineProvider) SearchCandidates(ctx context.Context, track model.Track, perSource int) ([]model.LyricSearchCandidate, error) {
	keyword := queryForTrack(track)
	if keyword == "" {
		return nil, fmt.Errorf("lyric: empty search keyword")
	}
	if perSource <= 0 {
		perSource = 8
	}

	var candidates []model.LyricSearchCandidate
	var errs []string
	rank := 1
	for _, source := range p.sources {
		providerName := providerAlias(source)
		if providerName == "" {
			continue
		}
		client, err := musicapi.Get(providerName, musicapi.WithHTTPClient(p.client))
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		result, err := client.Search(ctx, keyword, musicapi.SearchTypeSong, 1, perSource)
		if err != nil {
			errs = append(errs, source+": "+err.Error())
			continue
		}
		for _, song := range result.Songs {
			if song == nil {
				continue
			}
			id := fmt.Sprintf("%s:%s", source, song.ID)
			candidates = append(candidates, model.LyricSearchCandidate{
				ID:              id,
				Source:          source,
				ProviderTrackID: song.ID,
				Title:           song.Name,
				Artist:          artistNames(song),
				Album:           song.Album.Name,
				DurationMs:      int64(song.Duration),
				Score:           scoreSong(track, song),
				Rank:            rank,
			})
			rank++
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Rank < candidates[j].Rank
		}
		return candidates[i].Score > candidates[j].Score
	})
	for index := range candidates {
		candidates[index].Rank = index + 1
	}

	if len(candidates) > 0 {
		return candidates, nil
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("lyric: no enabled online provider")
	}
	return nil, fmt.Errorf("lyric: online search failed: %s", strings.Join(errs, "; "))
}

func (p *OnlineProvider) FetchCandidate(ctx context.Context, track model.Track, candidate model.LyricSearchCandidate) (model.LyricDocument, error) {
	source := strings.TrimSpace(candidate.Source)
	providerName := providerAlias(source)
	if providerName == "" {
		return model.LyricDocument{}, fmt.Errorf("lyric: unsupported source %q", candidate.Source)
	}
	if strings.TrimSpace(candidate.ProviderTrackID) == "" {
		return model.LyricDocument{}, fmt.Errorf("lyric: provider track id is required")
	}
	client, err := musicapi.Get(providerName, musicapi.WithHTTPClient(p.client))
	if err != nil {
		return model.LyricDocument{}, err
	}
	apiLyric, err := client.GetLyric(ctx, candidate.ProviderTrackID)
	if err != nil {
		return model.LyricDocument{}, err
	}
	song := &musicapi.Song{
		ID:       candidate.ProviderTrackID,
		Name:     candidate.Title,
		Duration: int(candidate.DurationMs),
	}
	if candidate.Artist != "" {
		song.Artists = []musicapi.ArtistBrief{{Name: candidate.Artist}}
	}
	if candidate.Album != "" {
		song.Album.Name = candidate.Album
	}
	return FromMusicAPILyric(track, source, song, apiLyric)
}

func normalizeSources(sources []string) []string {
	if len(sources) == 0 {
		return []string{"netease", "qqmusic", "kugou"}
	}
	seen := map[string]bool{}
	var result []string
	for _, source := range sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" || source == "local" || source == "amll-ttml-db" {
			continue
		}
		if !seen[source] {
			seen[source] = true
			result = append(result, source)
		}
	}
	return result
}

func artistNames(song *musicapi.Song) string {
	if song == nil {
		return ""
	}
	names := make([]string, 0, len(song.Artists))
	for _, artist := range song.Artists {
		if strings.TrimSpace(artist.Name) != "" {
			names = append(names, strings.TrimSpace(artist.Name))
		}
	}
	return strings.Join(names, " / ")
}

func providerAlias(source string) string {
	switch strings.ToLower(source) {
	case "qq", "qqmusic", "qq-music":
		return "qq"
	case "netease", "neteasecloud", "netease-cloud", "netease-cloud-music":
		return "netease"
	case "kugou", "ku-gou":
		return "kugou"
	default:
		return ""
	}
}

func queryForTrack(track model.Track) string {
	title := strings.TrimSpace(track.Title)
	artist := strings.TrimSpace(track.Artist)
	if title == "" || strings.EqualFold(title, "waiting") || strings.EqualFold(title, "waiting for playback") || title == "等待播放" {
		return ""
	}
	if artist == "" || strings.EqualFold(artist, "unknown artist") || strings.EqualFold(artist, "lyricsync") {
		return title
	}
	return title + " " + artist
}

func chooseSong(track model.Track, songs []*musicapi.Song) *musicapi.Song {
	if len(songs) == 0 {
		return nil
	}
	type candidate struct {
		song  *musicapi.Song
		score int
	}
	candidates := make([]candidate, 0, len(songs))
	for _, song := range songs {
		if song == nil {
			continue
		}
		candidates = append(candidates, candidate{song: song, score: scoreSong(track, song)})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0].song
}

func scoreSong(track model.Track, song *musicapi.Song) int {
	score := 0
	title := normalizeText(track.Title)
	songName := normalizeText(song.Name)
	if title != "" && songName == title {
		score += 100
	} else if title != "" && strings.Contains(songName, title) {
		score += 40
	}

	artist := normalizeText(track.Artist)
	for _, songArtist := range song.Artists {
		name := normalizeText(songArtist.Name)
		if artist != "" && name == artist {
			score += 80
		} else if artist != "" && (strings.Contains(name, artist) || strings.Contains(artist, name)) {
			score += 30
		}
	}

	if track.Duration > 0 && song.Duration > 0 {
		diff := track.Duration - int64(song.Duration)
		if diff < 0 {
			diff = -diff
		}
		if diff < 2500 {
			score += 30
		} else if diff < 8000 {
			score += 10
		}
	}

	return score
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}
