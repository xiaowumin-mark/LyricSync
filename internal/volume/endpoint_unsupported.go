//go:build !windows

package volume

import "fmt"

func setSystemVolume(level float64) error {
	return fmt.Errorf("volume: system volume control is only supported on Windows")
}

func getSystemVolume() (float64, error) {
	return 0, fmt.Errorf("volume: system volume control is only supported on Windows")
}
