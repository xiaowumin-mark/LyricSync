package lyric

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lyricsync/pkg/model"
)

type LocalProvider struct {
	paths []string
}

func NewLocalProvider(paths []string) *LocalProvider {
	return &LocalProvider{paths: paths}
}

func (p *LocalProvider) Search(ctx context.Context, track model.Track) (model.LyricDocument, error) {
	if len(p.paths) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no local lyric directories configured")
	}

	candidates, err := p.candidates(ctx, track)
	if err != nil {
		return model.LyricDocument{}, err
	}
	if len(candidates) == 0 {
		return model.LyricDocument{}, fmt.Errorf("lyric: no local lyric match")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	for _, candidate := range candidates {
		doc, err := loadLocalLyric(track, candidate.path)
		if err == nil {
			return doc, nil
		}
	}
	return model.LyricDocument{}, fmt.Errorf("lyric: local lyric candidates could not be parsed")
}

type localCandidate struct {
	path  string
	score int
}

func (p *LocalProvider) candidates(ctx context.Context, track model.Track) ([]localCandidate, error) {
	var candidates []localCandidate
	for _, root := range p.paths {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ttml" && ext != ".lrc" {
				return nil
			}
			score := scoreLocalFile(track, path)
			if score > 0 {
				candidates = append(candidates, localCandidate{path: path, score: score})
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return candidates, nil
}

func scoreLocalFile(track model.Track, path string) int {
	name := normalizeText(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	title := normalizeText(track.Title)
	artist := normalizeText(track.Artist)
	score := 0
	if title != "" && strings.Contains(name, title) {
		score += 80
	}
	if artist != "" && strings.Contains(name, artist) {
		score += 40
	}
	return score
}

func loadLocalLyric(track model.Track, path string) (model.LyricDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.LyricDocument{}, err
	}
	source := "local:" + filepath.Base(path)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttml":
		return FromTTMLText(track, source, string(data))
	case ".lrc":
		return FromLRCText(track, source, string(data))
	default:
		return model.LyricDocument{}, fmt.Errorf("lyric: unsupported local file %s", path)
	}
}
