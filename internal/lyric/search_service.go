package lyric

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

type SearchService struct {
	cacheDir string
	client   *http.Client
}

func NewSearchService(cacheDir string) *SearchService {
	return &SearchService{
		cacheDir: strings.TrimSpace(cacheDir),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *SearchService) SearchAll(ctx context.Context, query TrackQuery, cfg model.Config) SearchAllResult {
	if s == nil {
		return SearchAllResult{}
	}
	query = query.Clean()
	if query.Title == "" {
		return SearchAllResult{}
	}

	ttmlDB := newTTMLDBProvider(s.client, s.cacheDir, cfg.TTMLDB)
	providers := []Provider{
		ttmlDB,
		newQQProvider(s.client),
		newKugouProvider(s.client),
		newNeteaseProvider(s.client),
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	type providerRun struct {
		source string
		result ProviderResult
		err    error
	}
	done := make(chan providerRun, len(providers))
	for _, provider := range providers {
		provider := provider
		go func() {
			result, err := provider.Search(runCtx, query)
			done <- providerRun{source: provider.Source(), result: result, err: err}
		}()
	}

	out := SearchAllResult{}
	for range providers {
		run := <-done
		if run.err != nil {
			if runCtx.Err() == nil {
				out.Errors = append(out.Errors, ProviderError{Source: run.source, Err: run.err})
			}
			continue
		}
		if run.result.Source == "" {
			run.result.Source = run.source
		}
		if strings.TrimSpace(run.result.TTMLLyric) != "" && IsUsable(run.result.Document) {
			out.Results = append(out.Results, run.result)
		}
	}
	out.Results = filterConfirmedResults(out.Results)
	if updatedAt := ttmlDB.LastUpdatedAt(); !updatedAt.IsZero() {
		out.TTMLDBUpdatedAt = updatedAt
	}
	out.TTMLDBEntryCount = ttmlDB.EntryCount()
	return out
}

func filterConfirmedResults(results []ProviderResult) []ProviderResult {
	hasPlatformResult := false
	for _, result := range results {
		switch result.Source {
		case model.LyricSourceQQ, model.LyricSourceKugou, model.LyricSourceNetease:
			hasPlatformResult = true
		}
	}
	out := make([]ProviderResult, 0, len(results))
	for _, result := range results {
		if result.Source == model.LyricSourceTTMLDB && !hasPlatformResult {
			continue
		}
		out = append(out, result)
	}
	return out
}

func (s *SearchService) UpdateTTMLDBIndex(ctx context.Context, cfg model.TTMLDBConfig) (time.Time, int, error) {
	if s == nil {
		return time.Time{}, 0, nil
	}
	provider := newTTMLDBProvider(s.client, s.cacheDir, cfg)
	updatedAt, count, err := provider.UpdateIndex(ctx)
	if err != nil {
		return time.Time{}, 0, err
	}
	return updatedAt, count, nil
}
