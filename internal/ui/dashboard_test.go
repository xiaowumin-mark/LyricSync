package ui

import (
	"testing"

	"github.com/xiaowumin-mark/LyricSync/internal/model"
)

func TestDashboardLyricRangeKeepsOneLineAboveAndFollowingWindow(t *testing.T) {
	lines := make([]model.CurrentLyricLine, 20)
	for i := range lines {
		start := int64(i * 1000)
		lines[i] = model.CurrentLyricLine{StartTimeMs: start, EndTimeMs: start + 1000}
	}

	start, end := dashboardLyricRange(lines, 5500)
	if start != 4 || end != 14 {
		t.Fatalf("range = %d..%d, want 4..14", start, end)
	}

	start, end = dashboardLyricRange(lines, 200)
	if start != 0 || end != 9 {
		t.Fatalf("start range = %d..%d, want 0..9", start, end)
	}
}

func TestDashboardLyricRangeKeepsAllSimultaneousActiveLines(t *testing.T) {
	lines := make([]model.CurrentLyricLine, 20)
	for i := range lines {
		start := int64(i * 1000)
		lines[i] = model.CurrentLyricLine{StartTimeMs: start, EndTimeMs: start + 1000}
	}
	lines[5].EndTimeMs = 7000
	lines[6].StartTimeMs = 5000
	lines[6].EndTimeMs = 7000

	start, end := dashboardLyricRange(lines, 5500)
	if start != 4 || end != 15 {
		t.Fatalf("multi-active range = %d..%d, want 4..15", start, end)
	}
}

func BenchmarkDashboardLyricRange(b *testing.B) {
	lines := make([]model.CurrentLyricLine, 5000)
	for i := range lines {
		start := int64(i * 1800)
		lines[i] = model.CurrentLyricLine{StartTimeMs: start, EndTimeMs: start + 1600}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		position := int64(i%len(lines)) * 1800
		start, end := dashboardLyricRange(lines, position)
		if start < 0 || end > len(lines) || start >= end {
			b.Fatalf("invalid range %d..%d", start, end)
		}
	}
}

func BenchmarkResampleWaveformInto(b *testing.B) {
	values := make([]float64, 512)
	for i := range values {
		values[i] = float64(i%64) / 64
	}
	out := make([]float64, waveformBarCount)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resampleWaveformInto(values, out)
	}
}

func BenchmarkCoverCachePathCached(b *testing.B) {
	track := model.Track{
		CoverMimeType: "image/png",
		CoverHash:     "bench-cover",
		CoverData:     []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a},
	}
	_ = coverCachePath(track)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if coverCachePath(track) == "" {
			b.Fatal("missing cover path")
		}
	}
}
