package perf

import (
	"runtime"
	"sync"
	"time"
)

type CPUSampler struct {
	mu          sync.Mutex
	lastWall    time.Time
	lastCPU     time.Duration
	lastPercent float64
}

func NewCPUSampler() *CPUSampler {
	return &CPUSampler{}
}

func (s *CPUSampler) Percent(now time.Time) float64 {
	currentCPU, err := processCPUTime()
	if err != nil {
		return s.lastPercent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastWall.IsZero() {
		s.lastWall = now
		s.lastCPU = currentCPU
		return s.lastPercent
	}

	wallDelta := now.Sub(s.lastWall)
	cpuDelta := currentCPU - s.lastCPU
	s.lastWall = now
	s.lastCPU = currentCPU

	if wallDelta <= 0 || cpuDelta < 0 {
		return s.lastPercent
	}

	cores := runtime.NumCPU()
	if cores < 1 {
		cores = 1
	}
	percent := (float64(cpuDelta) / float64(wallDelta) / float64(cores)) * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.lastPercent = percent
	return percent
}
