//go:build !windows

package perf

import "time"

func processCPUTime() (time.Duration, error) {
	return 0, nil
}
