package media

import (
	"math"
	"testing"
)

func BenchmarkSpectrumProcessorProcess(b *testing.B) {
	processor := newSpectrumProcessor()
	samples := make([]float32, 2048)
	for i := range samples {
		samples[i] = float32(math.Sin(float64(i) * 0.035))
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		values := processor.Process(samples, 32, 2, 48000)
		if len(values) != 32 {
			b.Fatalf("bins = %d, want 32", len(values))
		}
	}
}
